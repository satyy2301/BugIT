import * as vscode from 'vscode';
import { spawn, ChildProcess } from 'child_process';
import * as fs from 'fs';
import * as net from 'net';
import * as path from 'path';

let panel: vscode.WebviewPanel | undefined;
let currentPayload: IDELoadResponse | undefined;
let highlightedIndex = 0;
let replayProcess: ChildProcess | undefined;
let drePath: string | undefined;

interface EventSummary {
  index: number;
  service: string;
  direction: string;
  summary: string;
  payload_preview: string;
  is_error: boolean;
  timestamp_ns: number;
}

interface Incident {
  title: string;
  summary: string;
  root_cause: string;
  services: string[];
  failed_step: string;
}

interface ClockEntry {
  index: number;
  timestamp_ns: number;
}

interface IDELoadResponse {
  manifest: {
    id: string;
    cluster: string;
    trigger: { type: string; detail?: string };
    incident?: Incident;
  };
  vector_graph?: {
    nodes?: Array<{ id: string; node_id: string; sequence: number }>;
    edges?: Array<{ from: string; to: string }>;
  };
  clock_timeline?: { entries?: ClockEntry[] };
  event_count?: number;
  events?: EventSummary[];
  flow?: string;
  replay?: {
    proxy_addr: string;
    debug_addr: string;
    delve_addr?: string;
    status: string;
    config_path?: string;
  };
}

export function activate(context: vscode.ExtensionContext) {
  context.subscriptions.push(
    vscode.commands.registerCommand('bugit.loadSnapshot', loadSnapshot),
    vscode.commands.registerCommand('bugit.stepForward', () => debuggerCall('StepForward')),
    vscode.commands.registerCommand('bugit.stepBackward', () => debuggerCall('StepBackward')),
    vscode.commands.registerCommand('bugit.attachDelve', attachDelve),
    vscode.commands.registerCommand('bugit.stopReplay', stopReplay)
  );
}

function resolveRepoRoot(): string | undefined {
  const folders = vscode.workspace.workspaceFolders;
  if (!folders?.length) {
    return undefined;
  }
  let dir = folders[0].uri.fsPath;
  for (let i = 0; i < 5; i++) {
    const hasBin =
      fs.existsSync(path.join(dir, 'bin', 'dre-replay.exe')) ||
      fs.existsSync(path.join(dir, 'bin', 'dre-replay'));
    const hasDeploy = fs.existsSync(path.join(dir, 'deploy', 'replay.yaml'));
    if (hasBin || hasDeploy) {
      return dir;
    }
    const parent = path.dirname(dir);
    if (parent === dir) {
      break;
    }
    dir = parent;
  }
  return folders[0].uri.fsPath;
}

function resolveReplayBin(): string {
  const cfg = vscode.workspace.getConfiguration('bugit');
  const configured = cfg.get<string>('replayBin');
  if (configured && fs.existsSync(configured)) {
    return configured;
  }
  if (process.env.DRE_REPLAY_BIN && fs.existsSync(process.env.DRE_REPLAY_BIN)) {
    return process.env.DRE_REPLAY_BIN;
  }
  const root = resolveRepoRoot();
  if (root) {
    for (const c of [path.join(root, 'bin', 'dre-replay.exe'), path.join(root, 'bin', 'dre-replay')]) {
      if (fs.existsSync(c)) {
        return c;
      }
    }
  }
  return process.platform === 'win32' ? 'dre-replay.exe' : 'dre-replay';
}

function resolveConfigPath(): string {
  const cfg = vscode.workspace.getConfiguration('bugit').get<string>('replayConfig');
  if (cfg && fs.existsSync(cfg)) {
    return cfg;
  }
  const root = resolveRepoRoot();
  if (root) {
    const p = path.join(root, 'deploy', 'replay.yaml');
    if (fs.existsSync(p)) {
      return p;
    }
  }
  return '';
}

async function loadSnapshot() {
  const folders = vscode.workspace.workspaceFolders;
  const defaultDir = folders?.[0]?.uri;
  const liveCapture = defaultDir ? vscode.Uri.file(path.join(defaultDir.fsPath, 'latest.dre')) : undefined;
  const demoFixture = defaultDir
    ? vscode.Uri.joinPath(defaultDir, 'test', 'fixtures', 'demo-checkout-500.dre')
    : undefined;
  const defaultUri =
    liveCapture && fs.existsSync(liveCapture.fsPath)
      ? liveCapture
      : demoFixture && fs.existsSync(demoFixture.fsPath)
        ? demoFixture
        : defaultDir;

  const uris = await vscode.window.showOpenDialog({
    canSelectMany: false,
    filters: { 'DRE Snapshots': ['dre'] },
    defaultUri,
    title: 'Load DRE snapshot',
  });
  if (!uris?.length) {
    return;
  }

  drePath = uris[0].fsPath;
  const replayBin = resolveReplayBin();
  const key = vscode.workspace.getConfiguration('bugit').get<string>('snapshotKey', 'dev-insecure-key-change-me');
  const configPath = resolveConfigPath();

  const loadArgs = ['load', '--format', 'ide', '--dre', drePath, '--key', key];
  if (configPath) {
    loadArgs.push('--config', configPath);
  }

  const child = spawn(replayBin, loadArgs, { shell: false, windowsHide: true });
  let stdout = '';
  let stderr = '';
  child.stdout.on('data', (d) => (stdout += d.toString()));
  child.stderr.on('data', (d) => (stderr += d.toString()));
  child.on('close', async (code) => {
    if (code !== 0) {
      vscode.window.showErrorMessage(`dre-replay failed: ${stderr || code}`);
      return;
    }
    try {
      currentPayload = JSON.parse(stdout) as IDELoadResponse;
      highlightedIndex = 0;
      await startReplay(replayBin, drePath!, key, configPath, currentPayload.replay?.debug_addr);
      renderPanel();
      const title = currentPayload.manifest.incident?.title ?? currentPayload.manifest.id;
      vscode.window.showInformationMessage(`Loaded: ${title}`);
    } catch (err) {
      vscode.window.showErrorMessage(`Failed to parse snapshot: ${err}`);
    }
  });
}

async function startReplay(
  replayBin: string,
  dre: string,
  key: string,
  configPath: string,
  debugAddr?: string,
) {
  stopReplay();
  if (debugAddr && await isDebuggerReachable(debugAddr)) {
    return;
  }
  const args = ['run', '--dre', dre, '--key', key];
  if (configPath) {
    args.push('--config', configPath);
  }
  replayProcess = spawn(replayBin, args, {
    shell: false,
    detached: false,
    windowsHide: true,
    stdio: 'ignore',
  });
  replayProcess.on('exit', () => {
    replayProcess = undefined;
  });
}

async function isDebuggerReachable(debugAddr: string): Promise<boolean> {
  const [host, portStr] = debugAddr.includes(':') ? debugAddr.split(':') : ['127.0.0.1', '19090'];
  const port = parseInt(portStr, 10);
  try {
    await debuggerRequest(host, port, { method: 'GetState' });
    return true;
  } catch {
    return false;
  }
}

function stopReplay() {
  if (replayProcess) {
    replayProcess.kill();
    replayProcess = undefined;
  }
}

async function attachDelve() {
  const goExtension = vscode.extensions.getExtension('golang.go');
  if (!goExtension) {
    const choice = await vscode.window.showWarningMessage(
      'Go extension (golang.go) is required for Delve attach. Install it or use Run and Debug → "DRE: Attach Delve".',
      'Open Extensions',
    );
    if (choice === 'Open Extensions') {
      await vscode.commands.executeCommand('workbench.extensions.search', 'golang.go');
    }
    return;
  }

  const addr = currentPayload?.replay?.delve_addr
    ?? vscode.workspace.getConfiguration('bugit').get<string>('delveAddr', '127.0.0.1:2345');
  const [host, portStr] = addr.includes(':') ? addr.split(':') : ['127.0.0.1', '2345'];
  const port = parseInt(portStr, 10);

  const folder = vscode.workspace.workspaceFolders?.[0];
  const started = await vscode.debug.startDebugging(folder, {
    type: 'go',
    request: 'attach',
    name: 'DRE Delve',
    mode: 'remote',
    host,
    port,
  });
  if (!started) {
    void vscode.window.showErrorMessage(
      `Failed to start Delve attach. Ensure dlv is listening on ${addr} (dre-replay run --binary ./your-app).`,
    );
  }
}

function renderPanel() {
  if (!currentPayload) {
    return;
  }
  const p = currentPayload;
  const incident = p.manifest.incident;
  const trigger = p.manifest.trigger;
  const events = p.events ?? [];
  const replay = p.replay;
  const clockEntries = p.clock_timeline?.entries ?? [];
  const graph = p.vector_graph;

  if (panel) {
    panel.reveal();
  } else {
    panel = vscode.window.createWebviewPanel('bugitTimeline', 'DRE Replay', vscode.ViewColumn.Beside, {
      enableScripts: true,
      retainContextWhenHidden: true,
    });
    panel.onDidDispose(() => {
      stopReplay();
      panel = undefined;
    });
    panel.webview.onDidReceiveMessage(async (msg) => {
      if (msg.type === 'step') {
        await debuggerCall(msg.method, msg.index);
      } else if (msg.type === 'stop') {
        stopReplay();
        renderPanel();
      }
    });
  }

  const eventRows = events
    .map((e) => {
      const cls = e.is_error ? 'error-row' : e.index === highlightedIndex ? 'active-row' : '';
      return `<tr class="${cls}" data-index="${e.index}">
        <td>${e.index + 1}</td>
        <td>${esc(e.service)}</td>
        <td>${esc(e.direction)}</td>
        <td>${esc(e.summary)}</td>
        <td class="preview">${esc(e.payload_preview)}</td>
      </tr>`;
    })
    .join('');

  const graphRows = (graph?.edges ?? [])
    .map((e) => `<tr><td>${esc(e.from)}</td><td>→</td><td>${esc(e.to)}</td></tr>`)
    .join('');

  const clockRows = clockEntries
    .map((c) => {
      const cls = c.index === highlightedIndex ? 'active-row' : '';
      return `<tr class="${cls}"><td>${c.index + 1}</td><td>${c.timestamp_ns}</td>
        <td><button type="button" class="seek-btn" data-action="seek" data-index="${c.index}">Seek</button></td></tr>`;
    })
    .join('');

  const services = incident?.services?.join(', ') ?? '—';
  const triggerBadge = trigger.type === 'http_5xx' ? 'auto-detected' : trigger.type;
  const replayStatus = replayProcess ? 'running (extension)' : (replay?.status ?? 'external or stopped');
  const nonce = getNonce();

  panel.webview.html = `<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-${nonce}';">
  <style>
    body { font-family: -apple-system, sans-serif; padding: 16px; color: #ccc; background: #1e1e1e; line-height: 1.5; }
    .banner { background: #3d1f1f; border: 1px solid #c44; border-radius: 8px; padding: 16px; margin-bottom: 16px; }
    .banner h2 { margin: 0 0 8px; color: #f88; }
    .controls { display: flex; gap: 8px; margin-bottom: 16px; flex-wrap: wrap; }
    button { background: #0e639c; color: #fff; border: none; padding: 8px 14px; border-radius: 4px; cursor: pointer; }
    button.secondary { background: #444; }
    .meta { display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 16px; }
    .badge { background: #333; padding: 4px 10px; border-radius: 4px; font-size: 12px; }
    .badge.auto { background: #5a2d2d; color: #f88; }
    .flow { background: #252526; padding: 12px; border-radius: 6px; font-size: 13px; margin-bottom: 16px; }
    .replay { background: #1a2d1a; border: 1px solid #3a5a3a; padding: 12px; border-radius: 6px; margin-bottom: 16px; }
    table { border-collapse: collapse; width: 100%; font-size: 13px; margin-bottom: 16px; }
    th, td { border: 1px solid #444; padding: 8px; text-align: left; vertical-align: top; }
    th { background: #2d2d2d; }
    .preview { font-family: monospace; font-size: 11px; color: #aaa; max-width: 280px; }
    .error-row { background: #3d2020; }
    .active-row { outline: 2px solid #569cd6; }
    h3 { margin-top: 20px; color: #9cdcfe; }
  </style>
</head>
<body>
  <div class="banner">
    <h2>${esc(incident?.title ?? 'Snapshot ' + p.manifest.id)}</h2>
    <p>${esc(incident?.summary ?? 'No incident summary.')}</p>
    <p><strong>Root cause:</strong> ${esc(incident?.root_cause ?? '—')}</p>
  </div>
  <div class="controls">
    <button type="button" data-action="step" data-method="StepBackward">◀ Step Back</button>
    <button type="button" data-action="step" data-method="StepForward">Step Forward ▶</button>
    <button type="button" class="secondary" data-action="stop">Stop Replay</button>
  </div>
  <div class="meta">
    <span class="badge">Cluster: ${esc(p.manifest.cluster)}</span>
    <span class="badge ${trigger.type === 'http_5xx' ? 'auto' : ''}">Trigger: ${esc(triggerBadge)}</span>
    <span class="badge">Events: ${p.event_count ?? events.length}</span>
    <span class="badge">Services: ${esc(services)}</span>
  </div>
  <div class="replay">
    <strong>Proxy:</strong> ${esc(replay?.proxy_addr ?? '127.0.0.1:18080')}
    <span class="badge">${esc(replayStatus)}</span><br>
    <strong>Debugger:</strong> ${esc(replay?.debug_addr ?? '127.0.0.1:19090')}<br>
    <strong>Delve:</strong> ${esc(replay?.delve_addr ?? '127.0.0.1:2345')}
  </div>
  <h3>Cross-service flow</h3>
  <div class="flow">${esc(p.flow ?? '—')}</div>
  <h3>Vector graph</h3>
  <table><tr><th>From</th><th></th><th>To</th></tr>${graphRows || '<tr><td colspan="3">No edges</td></tr>'}</table>
  <h3>Clock timeline</h3>
  <table><tr><th>Event #</th><th>Timestamp (ns)</th><th></th></tr>${clockRows || '<tr><td colspan="3">No clock entries</td></tr>'}</table>
  <h3>Event timeline</h3>
  <table>
    <tr><th>#</th><th>Service</th><th>Dir</th><th>Summary</th><th>Payload</th></tr>
    ${eventRows || '<tr><td colspan="5">No events</td></tr>'}
  </table>
  <script nonce="${nonce}">
    const vscode = acquireVsCodeApi();
    document.querySelectorAll('[data-action]').forEach((el) => {
      el.addEventListener('click', () => {
        const action = el.getAttribute('data-action');
        if (action === 'step') {
          vscode.postMessage({ type: 'step', method: el.getAttribute('data-method') });
        } else if (action === 'seek') {
          const index = parseInt(el.getAttribute('data-index') ?? '0', 10);
          vscode.postMessage({ type: 'step', method: 'Seek', index });
        } else if (action === 'stop') {
          vscode.postMessage({ type: 'stop' });
        }
      });
    });
  </script>
</body>
</html>`;
}

function getNonce(): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let nonce = '';
  for (let i = 0; i < 32; i++) {
    nonce += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return nonce;
}

function esc(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

async function debuggerCall(method: string, seekIndex?: number) {
  if (!currentPayload) {
    vscode.window.showWarningMessage('Load a snapshot first');
    return;
  }
  const addr = currentPayload.replay?.debug_addr ?? '127.0.0.1:19090';
  const [host, portStr] = addr.includes(':') ? addr.split(':') : ['127.0.0.1', '19090'];
  const port = parseInt(portStr, 10);

  try {
    const body: Record<string, unknown> = { method };
    if (method === 'Seek' && seekIndex !== undefined) {
      body.index = seekIndex;
    }
    const resp = await debuggerRequest(host, port, body);
    const idx = Number(resp.index);
    if (!Number.isNaN(idx)) {
      highlightedIndex = idx;
      renderPanel();
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg.includes('ECONNREFUSED')) {
      vscode.window.showWarningMessage(
        `Debugger not reachable on ${addr}. Start replay with dre-replay run or DRE: Load Snapshot.`,
      );
    } else if (msg === 'invalid debugger response') {
      vscode.window.showWarningMessage(`Debugger returned invalid JSON on ${addr}.`);
    } else {
      vscode.window.showWarningMessage(`Debugger error (${msg}). Is replay running on ${addr}?`);
    }
  }
}

function debuggerRequest(host: string, port: number, body: Record<string, unknown>): Promise<{ index?: number; total?: number }> {
  return new Promise((resolve, reject) => {
    const sock = net.createConnection({ host, port }, () => {
      sock.write(JSON.stringify(body) + '\n');
    });
    let data = '';
    let settled = false;
    const finish = (err?: Error, result?: { index?: number; total?: number }) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      sock.destroy();
      if (err) {
        reject(err);
      } else {
        resolve(result!);
      }
    };
    sock.on('data', (chunk) => {
      data += chunk.toString();
      try {
        finish(undefined, JSON.parse(data.trim()) as { index?: number; total?: number });
      } catch {
        // incomplete JSON; wait for more data
      }
    });
    sock.on('error', (err) => finish(err));
    const timer = setTimeout(() => finish(new Error('timeout')), 2000);
  });
}

export function deactivate() {
  stopReplay();
  panel?.dispose();
}
