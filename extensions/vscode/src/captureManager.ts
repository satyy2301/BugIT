import * as vscode from 'vscode';
import * as cp from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

export type CaptureState = 'idle' | 'recording' | 'saved';

let captureProcess: cp.ChildProcess | undefined;
let captureState: CaptureState = 'idle';
let statusListeners: Array<(s: CaptureState, msg: string) => void> = [];
let recordingHint = '';

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
  const envPath = path.join(captureRoot, '.env');
  let port = 4000;
  if (fs.existsSync(envPath)) {
    const m = fs.readFileSync(envPath, 'utf8').match(/^PORT=(\d+)/m);
    if (m) {
      port = parseInt(m[1], 10);
    }
  }
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

function handleCaptureOutput(text: string): void {
  if (text.includes('attached to backend')) {
    const m = text.match(/backend on :(\d+)/);
    recordingHint = m ? `Recording — attached to :${m[1]}` : 'Recording — attached to running backend';
    emitStatus();
  } else if (text.includes('spawn fallback') || text.includes('BugIT recording at')) {
    recordingHint = 'Recording — spawn fallback (dev server started by BugIT)';
    emitStatus();
  }
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
  recordingHint = '';
  captureState = 'recording';
  emitStatus();

  captureProcess = cp.spawn(bugitBin, ['record', '--root', workspace], {
    cwd: workspace,
    env: process.env,
    shell: false,
  });

  const out = vscode.window.createOutputChannel('BugIT Capture');
  out.show(true);
  out.appendLine(`Started: ${bugitBin} record`);
  out.appendLine('Run your app normally — BugIT attaches to the running backend when possible');

  captureProcess.stdout?.on('data', (d) => {
    const text = d.toString();
    out.append(text);
    handleCaptureOutput(text);
  });
  captureProcess.stderr?.on('data', (d) => {
    const text = d.toString();
    out.append(text);
    handleCaptureOutput(text);
  });

  captureProcess.on('exit', () => {
    captureProcess = undefined;
    const latest = latestSnapshotPath(workspace);
    captureState = latest ? 'saved' : 'idle';
    recordingHint = '';
    emitStatus();
  });
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

  if (latest) {
    captureState = 'saved';
    emitStatus();
    vscode.window.showInformationMessage('Bug snapshot saved');
    return latest;
  }
  captureState = 'idle';
  emitStatus();
  vscode.window.showWarningMessage('No snapshot saved yet — send a request to your app, then Stop again');
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
  captureProcess = undefined;
  recordingHint = '';
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
  const portMatch = fs.existsSync(path.join(captureRoot, '.env'))
    ? fs.readFileSync(path.join(captureRoot, '.env'), 'utf8').match(/^PORT=(\d+)/m)
    : null;
  const port = portMatch ? parseInt(portMatch[1], 10) : 4000;

  const bugitDir = path.join(captureRoot, '.bugit');
  if (!fs.existsSync(bugitDir)) {
    fs.mkdirSync(bugitDir, { recursive: true });
  }
  const yamlPath = path.join(bugitDir, 'bugit.yaml');
  const yaml = [
    'collector_http: http://127.0.0.1:28080',
    'collector_grpc: 127.0.0.1:29090',
    `record_proxy: 127.0.0.1:${port}`,
    `app_port: ${port}`,
    'snapshot_key: dev-insecure-key-change-me',
    'inspect_port: 9229',
    `capture_root: ${captureRoot.replace(/\\/g, '/')}`,
    'dev_command: npm run dev',
    '',
  ].join('\n');
  fs.writeFileSync(yamlPath, yaml);

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
