import * as vscode from 'vscode';
import * as cp from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

export type CaptureState = 'idle' | 'recording' | 'saved';

let captureProcess: cp.ChildProcess | undefined;
let captureState: CaptureState = 'idle';
let statusListeners: Array<(s: CaptureState, msg: string) => void> = [];

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
    return `Recording — use your app at ${publicUrl(folder.uri.fsPath)}`;
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
  captureState = 'recording';
  emitStatus();

  captureProcess = cp.spawn(bugitBin, ['capture', '--auto', '--root', workspace], {
    cwd: workspace,
    env: process.env,
    shell: false,
  });

  const out = vscode.window.createOutputChannel('BugIT Capture');
  out.show(true);
  out.appendLine(`Started: ${bugitBin} capture --auto`);
  out.appendLine(`Use your app at ${publicUrl(workspace)}`);

  captureProcess.stdout?.on('data', (d) => out.append(d.toString()));
  captureProcess.stderr?.on('data', (d) => out.append(d.toString()));

  captureProcess.on('exit', () => {
    captureProcess = undefined;
    captureState = 'saved';
    emitStatus();
  });
}

export async function stopCapture(context: vscode.ExtensionContext): Promise<string | undefined> {
  if (!captureProcess) {
    vscode.window.showWarningMessage('BugIT is not recording');
    return undefined;
  }

  captureProcess.kill('SIGINT');
  await new Promise((r) => setTimeout(r, 1500));

  const folder = vscode.workspace.workspaceFolders?.[0];
  if (!folder) {
    return undefined;
  }
  const latest = latestSnapshotPath(folder.uri.fsPath);
  if (latest && fs.existsSync(latest)) {
    captureState = 'saved';
    emitStatus();
    vscode.window.showInformationMessage('Bug snapshot saved');
    return latest;
  }
  vscode.window.showWarningMessage('No snapshot saved yet — send a request to your app, then Stop again');
  return undefined;
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
  emitStatus();
}
