import * as vscode from 'vscode';
import { spawn, ChildProcess } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import { downloadSnapshot, listSnapshots, pickLatest } from './collector';
import { debuggerRequest, DebuggerResponse, isDebuggerReachable, parseDebugAddr } from './replayClient';

let panel: vscode.WebviewPanel | undefined;
let currentPayload: IDELoadResponse | undefined;
let highlightedIndex = 0;
let selectedEventIndex = 0;
let replayProcess: ChildProcess | undefined;
let drePath: string | undefined;
let extensionContext: vscode.ExtensionContext;
let eventStatusBar: vscode.StatusBarItem | undefined;

interface EventSummary {
  index: number;
  service: string;
  direction: string;
  summary: string;
  payload_preview: string;
  payload?: string;
  is_error: boolean;
  timestamp_ns: number;
  pid?: number;
  tid?: number;
  comm?: string;
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
    delve_ready?: boolean;
    status: string;
    config_path?: string;
  };
}

export function activate(context: vscode.ExtensionContext) {
  extensionContext = context;
  eventStatusBar = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
  eventStatusBar.name = 'DRE Event';
  context.subscriptions.push(eventStatusBar);
  context.subscriptions.push(
    vscode.commands.registerCommand('bugit.loadSnapshot', loadSnapshot),
    vscode.commands.registerCommand('bugit.fetchLatest', fetchLatest),
    vscode.commands.registerCommand('bugit.loadFromCollector', loadFromCollector),
    vscode.commands.registerCommand('bugit.stepForward', () => debuggerCall('StepForward')),
    vscode.commands.registerCommand('bugit.stepBackward', () => debuggerCall('StepBackward')),
    vscode.commands.registerCommand('bugit.attachDelve', attachDelve),
    vscode.commands.registerCommand('bugit.startReplayDebug', startReplayDebug),
    vscode.commands.registerCommand('bugit.stopReplay', stopReplay),
    vscode.commands.registerCommand('bugit.openSourceAtEvent', openSourceAtEvent),
    vscode.debug.registerDebugConfigurationProvider('bugit-dre', {
      resolveDebugConfiguration: () => ({
        type: 'bugit-dre',
        request: 'launch',
        name: 'DRE Replay',
        debugAddr: currentPayload?.replay?.debug_addr ?? '127.0.0.1:19090',
      }),
    }),
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

function collectorUrl(): string {
  return vscode.workspace.getConfiguration('bugit').get<string>('collectorUrl', 'http://localhost:8080');
}

function latestDrePath(): string {
  const root = resolveRepoRoot() ?? vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? '';
  return path.join(root, 'latest.dre');
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
  await loadSnapshotFromPath(uris[0].fsPath);
}

async function fetchLatest() {
  const url = collectorUrl();
  try {
    const snaps = await listSnapshots(url);
    const latest = pickLatest(snaps);
    if (!latest) {
      vscode.window.showWarningMessage(`No snapshots on collector ${url}`);
      return;
    }
    const dest = latestDrePath();
    await downloadSnapshot(url, latest, dest);
    await loadSnapshotFromPath(dest);
  } catch (err) {
    vscode.window.showErrorMessage(`Fetch latest failed: ${err}`);
  }
}

async function loadFromCollector() {
  const url = collectorUrl();
  try {
    const snaps = await listSnapshots(url);
    if (!snaps.length) {
      vscode.window.showWarningMessage(`No snapshots on collector ${url}`);
      return;
    }
    const pick = await vscode.window.showQuickPick(
      snaps.map((s) => ({
        label: s.id,
        description: `${s.event_count ?? '?'} events · ${s.captured_at ?? 'unknown time'}`,
        snap: s,
      })),
      { placeHolder: 'Select snapshot from collector' },
    );
    if (!pick) {
      return;
    }
    const dest = path.join(path.dirname(latestDrePath()), `incident-${pick.snap.id}.dre`);
    await downloadSnapshot(url, pick.snap, dest);
    await loadSnapshotFromPath(dest);
  } catch (err) {
    vscode.window.showErrorMessage(`Load from collector failed: ${err}`);
  }
}

async function loadSnapshotFromPath(filePath: string) {
  drePath = filePath;
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
      selectedEventIndex = 0;
      await startReplay(replayBin, drePath!, key, configPath, currentPayload.replay?.debug_addr);
      updateEventStatusBar(0);
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

function stopReplay() {
  if (replayProcess) {
    replayProcess.kill();
    replayProcess = undefined;
  }
}

async function startReplayDebug() {
  if (!currentPayload) {
    vscode.window.showWarningMessage('Load a snapshot first');
    return;
  }
  const folder = vscode.workspace.workspaceFolders?.[0];
  const started = await vscode.debug.startDebugging(folder, {
    type: 'bugit-dre',
    request: 'launch',
    name: 'DRE Replay',
    debugAddr: currentPayload.replay?.debug_addr ?? '127.0.0.1:19090',
  });
  if (!started) {
    vscode.window.showErrorMessage('Failed to start DRE replay debug session');
  }
}

async function attachDelve() {
  const goExtension = vscode.extensions.getExtension('golang.go');
  if (!goExtension) {
    const choice = await vscode.window.showWarningMessage(
      'Go extension (golang.go) is required for Delve attach.',
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
      `Failed to start Delve attach. Ensure dlv is listening on ${addr}.`,
    );
  }
}

function buildMermaidSource(edges: Array<{ from: string; to: string }>): string {
  if (!edges.length) {
    return 'flowchart LR\n  empty[No edges]';
  }
  const lines = ['flowchart LR'];
  const nodes = new Set<string>();
  for (const e of edges) {
    nodes.add(e.from);
    nodes.add(e.to);
    const from = e.from.replace(/[^a-zA-Z0-9_]/g, '_');
    const to = e.to.replace(/[^a-zA-Z0-9_]/g, '_');
    lines.push(`  ${from}["${e.from}"] --> ${to}["${e.to}"]`);
  }
  return lines.join('\n');
}

function groupEventsByService(events: EventSummary[]): Map<string, EventSummary[]> {
  const groups = new Map<string, EventSummary[]>();
  for (const e of events) {
    const list = groups.get(e.service) ?? [];
    list.push(e);
    groups.set(e.service, list);
  }
  return groups;
}

function buildEventFlowHtml(events: EventSummary[], cursor: number): string {
  if (!events.length) {
    return '<span class="flow-step muted">No events</span>';
  }
  return events
    .map((e) => {
      const past = e.index < cursor ? 'past' : '';
      const current = e.index === cursor ? 'current' : '';
      const err = e.is_error ? 'error' : '';
      return `<span class="flow-step ${past} ${current} ${err}" title="Event ${e.index + 1}">${esc(e.service)}: ${esc(e.summary)}</span>`;
    })
    .join('<span class="flow-arrow">→</span>');
}

function currentEventBar(events: EventSummary[], cursor: number, total: number): string {
  const ev = events.find((e) => e.index === cursor);
  if (!ev) {
    return `Event ${cursor + 1} of ${total}`;
  }
  return `Event ${cursor + 1} of ${total} · ${ev.service} · ${ev.summary}`;
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
  const edges = graph?.edges ?? [];
  const mermaidSrc = buildMermaidSource(edges);
  const cursorEvent = events.find((e) => e.index === highlightedIndex);
  const payloadDetail = cursorEvent?.payload ?? cursorEvent?.payload_preview ?? '(select an event)';
  const eventFlowHtml = buildEventFlowHtml(events, highlightedIndex);
  const currentBar = currentEventBar(events, highlightedIndex, p.event_count ?? events.length);

  if (!panel) {
    panel = vscode.window.createWebviewPanel('bugitTimeline', 'DRE Replay', vscode.ViewColumn.Beside, {
      enableScripts: true,
      retainContextWhenHidden: true,
      localResourceRoots: [vscode.Uri.joinPath(extensionContext.extensionUri, 'media')],
    });
    panel.onDidDispose(() => {
      stopReplay();
      panel = undefined;
    });
    panel.webview.onDidReceiveMessage(async (msg) => {
      if (msg.type === 'step') {
        await debuggerCall(msg.method, msg.index);
      } else if (msg.type === 'toggleBp') {
        await toggleBreakpoint(msg.index);
      } else if (msg.type === 'stop') {
        stopReplay();
        renderPanel();
      }
    });
  } else {
    panel.reveal();
  }

  const flatRows = events
    .map((e) => eventRowHtml(e, true))
    .join('');

  const serviceGroups = groupEventsByService(events);
  const groupedRows = [...serviceGroups.entries()]
    .map(([svc, evts]) => {
      const rows = evts.map((e) => eventRowHtml(e, false)).join('');
      return `<details><summary><strong>${esc(svc)}</strong> (${evts.length})</summary><table>
        <tr><th>#</th><th>Dir</th><th>Summary</th><th>BP</th></tr>${rows}</table></details>`;
    })
    .join('');

  const clockRows = clockEntries
    .map((c) => {
      const cls = c.index === highlightedIndex ? 'active-row' : '';
      return `<tr class="${cls}"><td>${c.index + 1}</td><td>${c.timestamp_ns}</td>
        <td><button type="button" data-action="seek" data-index="${c.index}">Seek</button></td></tr>`;
    })
    .join('');

  const services = incident?.services?.join(', ') ?? '—';
  const triggerBadge = trigger.type === 'http_5xx' ? 'auto-detected' : trigger.type;
  const replayStatus = replayProcess ? 'running (extension)' : (replay?.status ?? 'external or stopped');
  const nonce = getNonce();
  const mermaidUri = panel.webview.asWebviewUri(
    vscode.Uri.joinPath(extensionContext.extensionUri, 'media', 'mermaid.min.js'),
  );

  panel.webview.html = `<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-${nonce}' ${mermaidUri};">
  <style>
    body { font-family: -apple-system, sans-serif; padding: 16px; color: #ccc; background: #1e1e1e; line-height: 1.5; }
    .banner { background: #3d1f1f; border: 1px solid #c44; border-radius: 8px; padding: 16px; margin-bottom: 16px; }
    .banner h2 { margin: 0 0 8px; color: #f88; }
    .controls { display: flex; gap: 8px; margin-bottom: 16px; flex-wrap: wrap; }
    button { background: #0e639c; color: #fff; border: none; padding: 8px 14px; border-radius: 4px; cursor: pointer; }
    button.secondary { background: #444; }
    button.bp { background: #8b4513; padding: 4px 8px; font-size: 11px; }
    .meta { display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 16px; }
    .badge { background: #333; padding: 4px 10px; border-radius: 4px; font-size: 12px; }
    .badge.auto { background: #5a2d2d; color: #f88; }
    .flow { background: #252526; padding: 12px; border-radius: 6px; font-size: 13px; margin-bottom: 16px; }
    .replay { background: #1a2d1a; border: 1px solid #3a5a3a; padding: 12px; border-radius: 6px; margin-bottom: 16px; }
    .current-event { background: #2d3a4a; border: 1px solid #569cd6; padding: 10px 14px; border-radius: 6px; margin-bottom: 12px; font-size: 14px; font-weight: 500; }
    .timeline-scroll { max-height: 280px; overflow-y: auto; margin-bottom: 16px; border: 1px solid #444; border-radius: 6px; }
    .payload-detail { background: #252526; border: 1px solid #444; padding: 12px; border-radius: 6px; font-family: monospace; font-size: 11px; white-space: pre-wrap; max-height: 200px; overflow: auto; margin-bottom: 16px; }
    .event-flow { background: #252526; padding: 12px; border-radius: 6px; margin-bottom: 16px; overflow-x: auto; display: flex; flex-wrap: wrap; align-items: center; gap: 4px; }
    .flow-step { background: #333; border: 1px solid #555; padding: 6px 10px; border-radius: 4px; font-size: 11px; white-space: nowrap; opacity: 0.45; }
    .flow-step.past { opacity: 0.75; border-color: #666; }
    .flow-step.current { opacity: 1; border: 2px solid #569cd6; background: #2d3a4a; font-weight: 600; }
    .flow-step.error { border-color: #c44; }
    .flow-step.muted { opacity: 0.6; }
    .flow-arrow { color: #888; margin: 0 2px; }
    .mermaid-wrap { background: #252526; padding: 12px; border-radius: 6px; margin-bottom: 16px; overflow-x: auto; }
    table { border-collapse: collapse; width: 100%; font-size: 13px; margin-bottom: 0; }
    .timeline-scroll table { margin-bottom: 0; }
    th, td { border: 1px solid #444; padding: 8px; text-align: left; vertical-align: top; }
    th { background: #2d2d2d; }
    .preview { font-family: monospace; font-size: 11px; color: #aaa; max-width: 280px; }
    .error-row { background: #3d2020; }
    .active-row { outline: 2px solid #569cd6; }
    .selectable-row { cursor: pointer; }
    .selectable-row:hover { background: #2a2a2a; }
    h3 { margin-top: 20px; color: #9cdcfe; }
    details { margin-bottom: 12px; }
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
    ${replay?.delve_ready ? '<span class="badge">delve ready</span>' : ''}
  </div>
  <div class="current-event">${esc(currentBar)}</div>
  <h3>Event timeline</h3>
  <div class="timeline-scroll" id="timeline-scroll">
    <table>
      <tr><th>#</th><th>Service</th><th>Dir</th><th>Summary</th><th>Payload</th><th>BP</th></tr>
      ${flatRows || '<tr><td colspan="6">No events</td></tr>'}
    </table>
  </div>
  <h3>Payload detail (event ${highlightedIndex + 1})</h3>
  <div class="payload-detail">${esc(payloadDetail)}</div>
  <h3>Service flow (cursor)</h3>
  <div class="event-flow">${eventFlowHtml}</div>
  <details>
    <summary><h3 style="display:inline;margin:0">Vector clock graph</h3></summary>
    <div class="mermaid-wrap"><pre class="mermaid">${esc(mermaidSrc)}</pre></div>
  </details>
  <details>
    <summary><h3 style="display:inline;margin:0">Events by service</h3></summary>
    ${groupedRows || '<p>No events</p>'}
  </details>
  <details>
    <summary><h3 style="display:inline;margin:0">Clock timeline &amp; cross-service flow</h3></summary>
    <table><tr><th>Event #</th><th>Timestamp (ns)</th><th></th></tr>${clockRows || '<tr><td colspan="3">No clock entries</td></tr>'}</table>
    <div class="flow">${esc(p.flow ?? '—')}</div>
  </details>
  <script nonce="${nonce}" src="${mermaidUri}"></script>
  <script nonce="${nonce}">
    const vscode = acquireVsCodeApi();
    document.querySelectorAll('[data-action]').forEach((el) => {
      el.addEventListener('click', (ev) => {
        ev.stopPropagation();
        const action = el.getAttribute('data-action');
        if (action === 'step') {
          vscode.postMessage({ type: 'step', method: el.getAttribute('data-method') });
        } else if (action === 'seek') {
          const index = parseInt(el.getAttribute('data-index') ?? '0', 10);
          vscode.postMessage({ type: 'step', method: 'Seek', index });
        } else if (action === 'stop') {
          vscode.postMessage({ type: 'stop' });
        } else if (action === 'toggleBp') {
          const index = parseInt(el.getAttribute('data-index') ?? '0', 10);
          vscode.postMessage({ type: 'toggleBp', index });
        }
      });
    });
    document.querySelectorAll('[data-select-index]').forEach((row) => {
      row.addEventListener('click', (ev) => {
        if (ev.target.closest('button')) return;
        const index = parseInt(row.getAttribute('data-select-index') ?? '0', 10);
        vscode.postMessage({ type: 'step', method: 'Seek', index });
      });
    });
    if (typeof mermaid !== 'undefined') {
      mermaid.initialize({ startOnLoad: false, theme: 'dark' });
      mermaid.run({ nodes: document.querySelectorAll('.mermaid') });
    }
    const active = document.getElementById('event-row-${highlightedIndex}') || document.querySelector('.active-row');
    if (active) active.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  </script>
</body>
</html>`;
}

function eventRowHtml(e: EventSummary, includeService: boolean): string {
  const cls = [
    e.is_error ? 'error-row' : '',
    e.index === highlightedIndex ? 'active-row' : '',
    'selectable-row',
  ]
    .filter(Boolean)
    .join(' ');
  const svcCol = includeService ? `<td>${esc(e.service)}</td>` : '';
  return `<tr id="event-row-${e.index}" class="${cls}" data-select-index="${e.index}">
    <td>${e.index + 1}</td>
    ${svcCol}
    <td>${esc(e.direction)}</td>
    <td>${esc(e.summary)}</td>
    ${includeService ? `<td class="preview">${esc(e.payload_preview)}</td>` : ''}
    <td><button type="button" class="bp" data-action="toggleBp" data-index="${e.index}">BP</button></td>
  </tr>`;
}

async function toggleBreakpoint(index: number) {
  if (!currentPayload) {
    return;
  }
  const addr = currentPayload.replay?.debug_addr ?? '127.0.0.1:19090';
  const { host, port } = parseDebugAddr(addr);
  try {
    await debuggerRequest(host, port, { method: 'SetBreakpoint', index, enabled: true });
    vscode.window.showInformationMessage(`Breakpoint set at event ${index + 1}`);
  } catch (err) {
    vscode.window.showWarningMessage(`Breakpoint failed: ${err}`);
  }
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

function updateEventStatusBar(index: number, resp?: DebuggerResponse) {
  if (!eventStatusBar) {
    return;
  }
  const ev = currentPayload?.events?.find((e) => e.index === index);
  const pid = Number(resp?.pid ?? ev?.pid ?? 0);
  const comm = String(resp?.comm ?? ev?.comm ?? ev?.service ?? '—');
  eventStatusBar.text = `DRE: Event ${index + 1} · pid=${pid} · ${comm}`;
  eventStatusBar.tooltip = 'Replay cursor — attach Delve to inspect Go source at this process';
  eventStatusBar.show();
}

async function openSourceAtEvent() {
  const ev = currentPayload?.events?.find((e) => e.index === highlightedIndex);
  if (!ev) {
    vscode.window.showWarningMessage('Load a snapshot and seek to an event first');
    return;
  }
  const delveSessions = vscode.debug.activeDebugSession;
  if (!delveSessions || delveSessions.type !== 'go') {
    vscode.window.showInformationMessage(
      `Event ${highlightedIndex + 1}: pid=${ev.pid ?? 0} comm=${ev.comm ?? ev.service}. ` +
        'Run DRE: Attach Delve, set breakpoints in Go source, then step replay cursor.',
    );
    return;
  }
  vscode.window.showInformationMessage(
    `Delve active — correlate event ${highlightedIndex + 1} (pid=${ev.pid ?? 0}) with your Go breakpoints.`,
  );
}

async function debuggerCall(method: string, seekIndex?: number) {
  if (!currentPayload) {
    vscode.window.showWarningMessage('Load a snapshot first');
    return;
  }
  const addr = currentPayload.replay?.debug_addr ?? '127.0.0.1:19090';
  const { host, port } = parseDebugAddr(addr);

  try {
    const body: Record<string, unknown> = { method };
    if (method === 'Seek' && seekIndex !== undefined) {
      body.index = seekIndex;
    }
    const resp = await debuggerRequest(host, port, body);
    const idx = Number(resp.index);
    if (!Number.isNaN(idx)) {
      highlightedIndex = idx;
      selectedEventIndex = idx;
      updateEventStatusBar(idx, resp);
      renderPanel();
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg.includes('ECONNREFUSED')) {
      vscode.window.showWarningMessage(
        `Debugger not reachable on ${addr}. Start replay with dre-replay run or DRE: Load Snapshot.`,
      );
    } else {
      vscode.window.showWarningMessage(`Debugger error (${msg}).`);
    }
  }
}

export function deactivate() {
  stopReplay();
  panel?.dispose();
}
