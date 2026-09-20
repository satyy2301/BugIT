import * as vscode from 'vscode';
import { spawn } from 'child_process';
import * as fs from 'fs';
import * as net from 'net';
import * as path from 'path';

let panel: vscode.WebviewPanel | undefined;
let currentPayload: IDELoadResponse | undefined;
let highlightedIndex = 0;

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

interface IDELoadResponse {
  manifest: {
    id: string;
    cluster: string;
    trigger: { type: string; detail?: string };
    incident?: Incident;
  };
  vector_graph?: { nodes?: Array<{ id: string; node_id: string; sequence: number }> };
  event_count?: number;
  events?: EventSummary[];
  flow?: string;
  replay?: { proxy_addr: string; debug_addr: string; status: string };
}

export function activate(context: vscode.ExtensionContext) {
  context.subscriptions.push(
    vscode.commands.registerCommand('bugit.loadSnapshot', loadSnapshot),
    vscode.commands.registerCommand('bugit.stepForward', () => stepDebug('StepForward')),
    vscode.commands.registerCommand('bugit.stepBackward', () => stepDebug('StepBackward'))
  );
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
  const folders = vscode.workspace.workspaceFolders;
  if (folders && folders.length > 0) {
    const root = folders[0].uri.fsPath;
    for (const c of [path.join(root, 'bin', 'dre-replay.exe'), path.join(root, 'bin', 'dre-replay')]) {
      if (fs.existsSync(c)) {
        return c;
      }
    }
  }
  return process.platform === 'win32' ? 'dre-replay.exe' : 'dre-replay';
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
    title: 'Load DRE snapshot (use make fetch-snapshot for live captures from kind)',
  });
  if (!uris?.length) {
    return;
  }

  const drePath = uris[0].fsPath;
  const replayBin = resolveReplayBin();
  const key = vscode.workspace.getConfiguration('bugit').get<string>('snapshotKey', 'dev-insecure-key-change-me');

  const child = spawn(replayBin, ['load', '--format', 'ide', '--dre', drePath, '--key', key], {
    shell: false,
    windowsHide: true,
  });
  let stdout = '';
  let stderr = '';
  child.stdout.on('data', (d) => (stdout += d.toString()));
  child.stderr.on('data', (d) => (stderr += d.toString()));
  child.on('close', (code) => {
    if (code !== 0) {
      vscode.window.showErrorMessage(`dre-replay failed: ${stderr || code}`);
      return;
    }
    try {
      currentPayload = JSON.parse(stdout) as IDELoadResponse;
      highlightedIndex = 0;
      renderPanel();
      spawn(replayBin, ['run', '--dre', drePath, '--key', key], {
        shell: false,
        detached: true,
        windowsHide: true,
        stdio: 'ignore',
      });
      const title = currentPayload.manifest.incident?.title ?? currentPayload.manifest.id;
      vscode.window.showInformationMessage(`Loaded: ${title}`);
    } catch (err) {
      vscode.window.showErrorMessage(`Failed to parse snapshot: ${err}`);
    }
  });
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

  if (panel) {
    panel.reveal();
  } else {
    panel = vscode.window.createWebviewPanel('bugitTimeline', 'DRE Timeline', vscode.ViewColumn.Beside, {
      enableScripts: true,
    });
    panel.onDidDispose(() => {
      panel = undefined;
    });
  }

  const eventRows = events
    .map((e, i) => {
      const cls = e.is_error ? 'error-row' : i === highlightedIndex ? 'active-row' : '';
      return `<tr class="${cls}" data-index="${e.index}">
        <td>${i + 1}</td>
        <td>${esc(e.service)}</td>
        <td>${esc(e.direction)}</td>
        <td>${esc(e.summary)}</td>
        <td class="preview">${esc(e.payload_preview)}</td>
      </tr>`;
    })
    .join('');

  const services = incident?.services?.join(', ') ?? '—';
  const triggerBadge = trigger.type === 'http_5xx' ? 'auto-detected' : trigger.type;

  panel.webview.html = `<!DOCTYPE html>
<html>
<head>
  <style>
    body { font-family: -apple-system, sans-serif; padding: 16px; color: #ccc; background: #1e1e1e; line-height: 1.5; }
    .banner { background: #3d1f1f; border: 1px solid #c44; border-radius: 8px; padding: 16px; margin-bottom: 16px; }
    .banner h2 { margin: 0 0 8px; color: #f88; }
    .banner p { margin: 4px 0; color: #ddd; }
    .meta { display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 16px; }
    .badge { background: #333; padding: 4px 10px; border-radius: 4px; font-size: 12px; }
    .badge.auto { background: #5a2d2d; color: #f88; }
    .flow { background: #252526; padding: 12px; border-radius: 6px; font-size: 13px; margin-bottom: 16px; word-break: break-word; }
    .replay { background: #1a2d1a; border: 1px solid #3a5a3a; padding: 12px; border-radius: 6px; margin-bottom: 16px; }
    table { border-collapse: collapse; width: 100%; font-size: 13px; }
    th, td { border: 1px solid #444; padding: 8px; text-align: left; vertical-align: top; }
    th { background: #2d2d2d; }
    .preview { font-family: monospace; font-size: 11px; color: #aaa; max-width: 280px; }
    .error-row { background: #3d2020; }
    .error-row td { color: #faa; }
    .active-row { outline: 2px solid #569cd6; }
    h3 { margin-top: 20px; color: #9cdcfe; }
  </style>
</head>
<body>
  <div class="banner">
    <h2>${esc(incident?.title ?? 'Snapshot ' + p.manifest.id)}</h2>
    <p>${esc(incident?.summary ?? 'No incident summary in this snapshot.')}</p>
    <p><strong>Root cause:</strong> ${esc(incident?.root_cause ?? '—')}</p>
    <p><strong>Failed step:</strong> ${esc(incident?.failed_step ?? '—')}</p>
  </div>
  <div class="meta">
    <span class="badge">Cluster: ${esc(p.manifest.cluster)}</span>
    <span class="badge ${trigger.type === 'http_5xx' ? 'auto' : ''}">Trigger: ${esc(triggerBadge)}</span>
    <span class="badge">Events: ${p.event_count ?? events.length}</span>
    <span class="badge">Services: ${esc(services)}</span>
  </div>
  <div class="replay">
    <strong>Replay proxy:</strong> ${esc(replay?.proxy_addr ?? '127.0.0.1:18080')}
    <span class="badge" style="margin-left:8px">${esc(replay?.status ?? 'unknown')}</span>
    <br><small>Local app can connect here to receive recorded responses (zero network to production).</small>
    <br><small>Debugger API: ${esc(replay?.debug_addr ?? '127.0.0.1:19090')} — use Step Forward/Backward commands</small>
  </div>
  <h3>Cross-service flow</h3>
  <div class="flow">${esc(p.flow ?? '—')}</div>
  <h3>Event timeline</h3>
  <table>
    <tr><th>#</th><th>Service</th><th>Dir</th><th>Summary</th><th>Payload</th></tr>
    ${eventRows || '<tr><td colspan="5">No events</td></tr>'}
  </table>
</body>
</html>`;
}

function esc(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

async function stepDebug(method: string) {
  if (!currentPayload) {
    vscode.window.showWarningMessage('Load a snapshot first (DRE: Load Snapshot)');
    return;
  }
  const addr = currentPayload.replay?.debug_addr ?? '127.0.0.1:19090';
  const [host, portStr] = addr.includes(':') ? addr.split(':') : ['127.0.0.1', '19090'];
  const port = parseInt(portStr, 10);

  try {
    const resp = await debuggerRequest(host, port, method);
    if (typeof resp.index === 'number') {
      highlightedIndex = Math.min(resp.index, (currentPayload.events?.length ?? 1) - 1);
      renderPanel();
    }
    vscode.window.showInformationMessage(`${method}: step ${resp.index ?? 0} / ${resp.total ?? '?'}`);
  } catch (err) {
    vscode.window.showWarningMessage(`Debugger not running. Load snapshot first to start replay. (${err})`);
  }
}

function debuggerRequest(host: string, port: number, method: string): Promise<{ index?: number; total?: number }> {
  return new Promise((resolve, reject) => {
    const sock = net.createConnection({ host, port }, () => {
      sock.write(JSON.stringify({ method }) + '\n');
    });
    let data = '';
    sock.on('data', (chunk) => (data += chunk.toString()));
    sock.on('end', () => {
      try {
        resolve(JSON.parse(data.trim()));
      } catch {
        reject(new Error('invalid debugger response'));
      }
    });
    sock.on('error', reject);
    setTimeout(() => {
      sock.destroy();
      reject(new Error('timeout'));
    }, 2000);
  });
}

export function deactivate() {
  panel?.dispose();
}
