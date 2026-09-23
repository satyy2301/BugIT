import * as vscode from 'vscode';
import * as cp from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import { preflightBugitBinary } from './binaryCheck';
import { parseYamlValue, resolveBackendPort } from './portResolver';

export type CaptureState = 'idle' | 'recording' | 'saved';

type CaptureMode = 'record' | 'capture-auto';

let captureProcess: cp.ChildProcess | undefined;
let captureState: CaptureState = 'idle';
let statusListeners: Array<(s: CaptureState, msg: string) => void> = [];
let recordingHint = '';
let captureOutput = '';
let captureStartedAt = 0;
let captureOutputChannel: vscode.OutputChannel | undefined;
let usingCaptureFallback = false;

export function onCaptureStatus(listener: (s: CaptureState, msg: string) => void): vscode.Disposable {
  statusListeners.push(listener);
  listener(captureState, statusMessage());
  return new vscode.Disposable(() => {
    statusListeners = statusListeners.filter((l) => l !== listener);
  });
}

function emitStatus() {
  const msg = statusMessage();
  for (const l of statusListeners) {
    l(captureState, msg);
  }
}

function statusMessage(): string {
  const folder = vscode.workspace.workspaceFolders?.[0];
  if (!folder) {
    return 'Open a project folder';
  }
  if (captureState === 'recording') {
    if (recordingHint) {
      return recordingHint;
    }
    return `Recording — use your app normally (${publicUrl(folder.uri.fsPath)})`;
  }
  if (captureState === 'saved') {
    return 'Snapshot saved — click Replay';
  }
  return 'Ready — click Record';
}

function publicUrl(workspace: string): string {
  const cfgPort = vscode.workspace.getConfiguration('bugit').get<number>('publicPort');
  if (cfgPort) {
    return `http://localhost:${cfgPort}`;
  }
  const captureRoot = findCaptureRoot(workspace);
  const port = resolveBackendPort(workspace, captureRoot);
  return `http://localhost:${port}`;
}

function findCaptureRoot(workspace: string): string {
  const cfg = vscode.workspace.getConfiguration('bugit').get<string>('captureRoot');
  if (cfg) {
    const p = path.isAbsolute(cfg) ? cfg : path.join(workspace, cfg);
    if (fs.existsSync(path.join(p, 'package.json'))) {
      return p;
    }
  }
  for (const sub of ['backend', 'server', 'api']) {
    const p = path.join(workspace, sub);
    if (fs.existsSync(path.join(p, 'package.json'))) {
      return p;
    }
  }
  if (fs.existsSync(path.join(workspace, 'package.json'))) {
    return workspace;
  }
  return workspace;
}

export function resolveBugitBin(context: vscode.ExtensionContext): string {
  const configured = vscode.workspace.getConfiguration('bugit').get<string>('bugitBin');
  if (configured && fs.existsSync(configured)) {
    return configured;
  }
  const plat =
    process.platform === 'win32' ? 'win32-x64' : process.platform === 'darwin' ? 'darwin-arm64' : 'linux-x64';
  const exe = process.platform === 'win32' ? 'bugit.exe' : 'bugit';
  const bundled = path.join(context.extensionPath, 'bin', plat, exe);
  if (fs.existsSync(bundled)) {
    return bundled;
  }
  return exe;
}

export function isRecording(): boolean {
  return captureState === 'recording';
}

function runBugitCommand(bugitBin: string, args: string[], workspace: string): Promise<void> {
  return new Promise((resolve) => {
    const proc = cp.spawn(bugitBin, args, { cwd: workspace, env: process.env, shell: false });
    proc.on('exit', () => resolve());
    setTimeout(resolve, 10000);
  });
}

async function triggerSnapshot(bugitBin: string, workspace: string): Promise<void> {
  await runBugitCommand(bugitBin, ['snapshot', '--root', workspace, '--detail', 'stop and save'], workspace);
}

async function waitForSnapshot(workspace: string, timeoutMs: number): Promise<string> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const latest = latestSnapshotPath(workspace);
    if (latest && fs.existsSync(latest)) {
      return latest;
    }
    await new Promise((r) => setTimeout(r, 250));
  }
  return '';
}

function killProcessTree(proc: cp.ChildProcess, force: boolean): void {
  if (!proc.pid) {
    return;
  }
  if (process.platform === 'win32') {
    const args = force ? ['/PID', String(proc.pid), '/T', '/F'] : ['/PID', String(proc.pid), '/T'];
    cp.spawn('taskkill', args, { shell: false });
    return;
  }
  try {
    proc.kill(force ? 'SIGKILL' : 'SIGINT');
  } catch {
    // ignore
  }
}

function shouldSkipCaptureFallback(output: string): boolean {
  const lower = output.toLowerCase();
  return (
    lower.includes('backend listening') ||
    lower.includes('resolve backend inspector') ||
    lower.includes('eaddrinuse') ||
    lower.includes('restart backend') ||
    lower.includes('inspector on :923') ||
    lower.includes('configured backend inspector') ||
    lower.includes('already listening on')
  );
}

function parseRestartCommand(output: string): string | undefined {
  const m = output.match(/restart backend \(([^)]+)\)/i);
  if (m) {
    return m[1].trim();
  }
  if (output.includes('configured backend inspector on :923')) {
    return 'npm run dev';
  }
  return undefined;
}

function handleCaptureOutput(text: string): void {
  captureOutput += text;
  if (text.includes('capturing inbound + outbound HTTP') || text.includes('capturing inbound HTTP')) {
    const m = text.match(/HTTP on :(\d+)/);
    recordingHint = m
      ? `Recording — capturing API traffic on :${m[1]}`
      : 'Recording — capturing inbound + outbound API traffic';
    emitStatus();
  } else if (text.includes('BugIT inspector target:')) {
    const m = text.match(/inspector target: .+ on :(\d+)/);
    recordingHint = m
      ? `Recording — backend inspector on :${m[1]}`
      : 'Recording — backend inspector attached';
    emitStatus();
  } else if (text.includes('attached to backend')) {
    const m = text.match(/backend on :(\d+)/);
    recordingHint = m ? `Recording — attached to :${m[1]}` : 'Recording — attached to running backend';
    emitStatus();
  } else if (text.includes('configured backend inspector on :')) {
    const m = text.match(/configured backend inspector on :(\d+)/);
    recordingHint = m
      ? `Recording — restart backend for inspector :${m[1]}, then Record again`
      : 'Recording — restart backend once for BugIT inspector';
    emitStatus();
  } else if (text.includes('preload capture active') || text.includes('preload mode')) {
    recordingHint = 'Recording — preload HTTP ingest (restart backend if no events)';
    emitStatus();
  } else if (text.includes('no HTTP events captured')) {
    vscode.window.showWarningMessage(
      'BugIT captured no API traffic — restart backend if prompted, use your app, then Stop again'
    );
  } else if (text.includes('spawn fallback') || text.includes('BugIT recording at') || text.includes('BugIT capture in')) {
    recordingHint = usingCaptureFallback
      ? 'Recording — capture fallback (dev server started by BugIT)'
      : 'Recording — spawn fallback (dev server started by BugIT)';
    emitStatus();
  }
}

function captureArgs(mode: CaptureMode, workspace: string): string[] {
  if (mode === 'capture-auto') {
    return ['capture', '--auto', '--root', workspace];
  }
  return ['record', '--root', workspace];
}

function looksLikeUnknownRecordCommand(text: string): boolean {
  return text.includes('plug-and-play local bug capture') && !text.includes('bugit record');
}

function resetCaptureSession(): void {
  captureProcess = undefined;
  captureOutput = '';
  captureStartedAt = 0;
  usingCaptureFallback = false;
  recordingHint = '';
}

function handleCaptureFailure(message: string, workspace: string): void {
  captureOutputChannel?.appendLine(`ERROR: ${message}`);
  if (captureOutput.trim()) {
    captureOutputChannel?.appendLine(captureOutput.trim());
  }
  resetCaptureSession();
  const latest = latestSnapshotPath(workspace);
  captureState = latest ? 'saved' : 'idle';
  emitStatus();
  vscode.window.showErrorMessage(message);
}

function attachCaptureProcessHandlers(
  proc: cp.ChildProcess,
  bugitBin: string,
  workspace: string,
  mode: CaptureMode,
): void {
  proc.stdout?.on('data', (d) => {
    const text = d.toString();
    captureOutputChannel?.append(text);
    handleCaptureOutput(text);
  });
  proc.stderr?.on('data', (d) => {
    const text = d.toString();
    captureOutputChannel?.append(text);
    handleCaptureOutput(text);
  });

  proc.on('error', (err) => {
    handleCaptureFailure(`Failed to start BugIT (${bugitBin}): ${err.message}`, workspace);
  });

  proc.on('exit', (code) => {
    const elapsed = Date.now() - captureStartedAt;
    const earlyExit = elapsed < 2000 && code !== 0 && code !== null;
    const unknownRecord = mode === 'record' && looksLikeUnknownRecordCommand(captureOutput);

    if (earlyExit || unknownRecord) {
      if (mode === 'record' && !usingCaptureFallback && !shouldSkipCaptureFallback(captureOutput)) {
        captureOutputChannel?.appendLine('WARN: bugit record failed — retrying with capture --auto');
        usingCaptureFallback = true;
        recordingHint = '';
        spawnCaptureProcess(bugitBin, workspace, 'capture-auto');
        return;
      }
      const restartCmd = parseRestartCommand(captureOutput);
      let message = 'BugIT recording exited immediately. Run BugIT: Doctor to check your CLI binary.';
      if (shouldSkipCaptureFallback(captureOutput)) {
        message =
          'Backend inspector blocked by Next.js on :9229. BugIT configured :9230 — restart backend, then Record again.';
      }
      if (restartCmd) {
        vscode.window
          .showErrorMessage(message, 'Copy restart command')
          .then((choice) => {
            if (choice === 'Copy restart command') {
              void vscode.env.clipboard.writeText(restartCmd);
            }
          });
      } else {
        handleCaptureFailure(message, workspace);
      }
      if (restartCmd) {
        resetCaptureSession();
        captureState = 'idle';
        emitStatus();
      }
      return;
    }

    captureProcess = undefined;
    captureOutput = '';
    captureStartedAt = 0;
    usingCaptureFallback = false;
    const latest = latestSnapshotPath(workspace);
    captureState = latest ? 'saved' : 'idle';
    recordingHint = '';
    emitStatus();
  });
}

function spawnCaptureProcess(bugitBin: string, workspace: string, mode: CaptureMode): void {
  resetCaptureSession();
  usingCaptureFallback = mode === 'capture-auto';
  captureStartedAt = Date.now();
  captureOutput = '';

  const args = captureArgs(mode, workspace);
  captureProcess = cp.spawn(bugitBin, args, {
    cwd: workspace,
    env: process.env,
    shell: false,
  });

  captureOutputChannel?.appendLine(`Started: ${bugitBin} ${args.join(' ')}`);
  if (mode === 'record') {
    captureOutputChannel?.appendLine('Run your app normally — BugIT attaches to the running backend when possible');
  } else {
    captureOutputChannel?.appendLine('Capture fallback — BugIT will start your dev server if needed');
  }

  attachCaptureProcessHandlers(captureProcess, bugitBin, workspace, mode);
}

export async function startCapture(context: vscode.ExtensionContext): Promise<void> {
  const folder = vscode.workspace.workspaceFolders?.[0];
  if (!folder) {
    vscode.window.showWarningMessage('Open a project folder first');
    return;
  }
  if (captureProcess) {
    vscode.window.showInformationMessage('BugIT is already recording');
    return;
  }

  await writeWorkspaceDefaults(folder.uri.fsPath);

  const bugitBin = resolveBugitBin(context);
  const workspace = folder.uri.fsPath;

  const preflight = await preflightBugitBinary(bugitBin);
  if (!preflight) {
    return;
  }

  recordingHint = '';
  captureState = 'recording';
  emitStatus();

  captureOutputChannel = vscode.window.createOutputChannel('BugIT Capture');
  captureOutputChannel.show(true);

  spawnCaptureProcess(bugitBin, workspace, 'record');
}

export async function stopCapture(context: vscode.ExtensionContext): Promise<string | undefined> {
  if (!captureProcess) {
    vscode.window.showWarningMessage('BugIT is not recording');
    return undefined;
  }

  const folder = vscode.workspace.workspaceFolders?.[0];
  if (!folder) {
    return undefined;
  }
  const workspace = folder.uri.fsPath;
  const bugitBin = resolveBugitBin(context);

  await triggerSnapshot(bugitBin, workspace);
  killProcessTree(captureProcess, false);
  await new Promise((r) => setTimeout(r, 500));

  const latest = await waitForSnapshot(workspace, 5000);
  captureProcess = undefined;
  recordingHint = '';
  usingCaptureFallback = false;

  if (latest) {
    captureState = 'saved';
    emitStatus();
    vscode.window.showInformationMessage('Bug snapshot saved');
    return latest;
  }
  captureState = 'idle';
  emitStatus();
  vscode.window.showWarningMessage(
    'No snapshot saved yet — use your app (hit the backend API), then Stop again'
  );
  return undefined;
}

export async function forceStopCapture(context: vscode.ExtensionContext): Promise<void> {
  const folder = vscode.workspace.workspaceFolders?.[0];
  const workspace = folder?.uri.fsPath ?? '';
  const bugitBin = resolveBugitBin(context);

  if (workspace) {
    await triggerSnapshot(bugitBin, workspace).catch(() => undefined);
  }
  if (captureProcess) {
    killProcessTree(captureProcess, true);
  }
  resetCaptureSession();
  captureState = 'idle';
  emitStatus();
  vscode.window.showInformationMessage('BugIT recording force-stopped');
}

export function latestSnapshotPath(workspace: string): string {
  const root = findCaptureRoot(workspace);
  const latest = path.join(root, '.bugit', 'latest.dre');
  if (fs.existsSync(latest)) {
    return latest;
  }
  return '';
}

async function writeWorkspaceDefaults(workspace: string): Promise<void> {
  const captureRoot = findCaptureRoot(workspace);
  const relRoot = path.relative(workspace, captureRoot).replace(/\\/g, '/') || '.';
  const port = resolveBackendPort(workspace, captureRoot);

  const bugitDir = path.join(captureRoot, '.bugit');
  if (!fs.existsSync(bugitDir)) {
    fs.mkdirSync(bugitDir, { recursive: true });
  }
  const yamlPath = path.join(bugitDir, 'bugit.yaml');
  const existing = fs.existsSync(yamlPath) ? fs.readFileSync(yamlPath, 'utf8') : '';
  const inspectPort = parseYamlValue(existing, 'inspect_port');
  const snapshotKey = parseYamlValue(existing, 'snapshot_key') ?? 'dev-insecure-key-change-me';
  const devCommand = parseYamlValue(existing, 'dev_command') ?? 'npm run dev';
  const collectorHTTP = parseYamlValue(existing, 'collector_http') ?? 'http://127.0.0.1:28080';
  const collectorGRPC = parseYamlValue(existing, 'collector_grpc') ?? '127.0.0.1:29090';

  const lines = [
    `collector_http: ${collectorHTTP}`,
    `collector_grpc: ${collectorGRPC}`,
    `record_proxy: 127.0.0.1:${port}`,
    `app_port: ${port}`,
    `snapshot_key: ${snapshotKey}`,
    `capture_root: ${captureRoot.replace(/\\/g, '/')}`,
    `dev_command: ${devCommand}`,
  ];
  if (inspectPort) {
    lines.push(`inspect_port: ${inspectPort}`);
  }
  fs.writeFileSync(yamlPath, lines.join('\n') + '\n');

  const vsDir = path.join(workspace, '.vscode');
  const settingsPath = path.join(vsDir, 'settings.json');
  let settings: Record<string, unknown> = {};
  if (fs.existsSync(settingsPath)) {
    try {
      settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
    } catch {
      settings = {};
    }
  }
  settings['bugit.captureRoot'] = relRoot;
  settings['bugit.devCommand'] = 'npm run dev';
  settings['bugit.publicPort'] = port;
  if (!fs.existsSync(vsDir)) {
    fs.mkdirSync(vsDir, { recursive: true });
  }
  fs.writeFileSync(settingsPath, JSON.stringify(settings, null, 2) + '\n');
}

export function resetCaptureState() {
  captureState = 'idle';
  recordingHint = '';
  emitStatus();
}
