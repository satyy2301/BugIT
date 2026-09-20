import * as vscode from 'vscode';
import { spawn } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

let panel: vscode.WebviewPanel | undefined;
let timeline: Array<{ id: string; node_id: string; sequence: number }> = [];

interface IDELoadResponse {
  manifest: Record<string, unknown>;
  vector_graph?: { nodes?: Array<{ id: string; node_id: string; sequence: number }> };
  event_count?: number;
  vector_nodes?: number;
}

export function activate(context: vscode.ExtensionContext) {
  context.subscriptions.push(
    vscode.commands.registerCommand('bugit.loadSnapshot', loadSnapshot),
    vscode.commands.registerCommand('bugit.stepForward', () => postDebugMethod('StepForward')),
    vscode.commands.registerCommand('bugit.stepBackward', () => postDebugMethod('StepBackward'))
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
    const candidates = [
      path.join(root, 'bin', 'dre-replay.exe'),
      path.join(root, 'bin', 'dre-replay'),
    ];
    for (const c of candidates) {
      if (fs.existsSync(c)) {
        return c;
      }
    }
  }
  return process.platform === 'win32' ? 'dre-replay.exe' : 'dre-replay';
}

async function loadSnapshot() {
  const uris = await vscode.window.showOpenDialog({
    canSelectMany: false,
    filters: { 'DRE Snapshots': ['dre'] },
    defaultUri: vscode.workspace.workspaceFolders?.[0]?.uri,
  });
  if (!uris || uris.length === 0) {
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
      const payload = JSON.parse(stdout) as IDELoadResponse;
      timeline = payload.vector_graph?.nodes ?? [];
      showPanel(payload.manifest, payload.event_count ?? 0);
      spawn(replayBin, ['run', '--dre', drePath, '--key', key], {
        shell: false,
        detached: true,
        windowsHide: true,
        stdio: 'ignore',
      });
      vscode.window.showInformationMessage(
        `Loaded snapshot ${payload.manifest.id} (${payload.event_count ?? 0} events)`
      );
    } catch (err) {
      vscode.window.showErrorMessage(`Failed to parse snapshot: ${err}`);
    }
  });
}

function showPanel(manifest: Record<string, unknown>, eventCount: number) {
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
  const rows = timeline
    .map((n) => `<tr><td>${n.id}</td><td>${n.node_id}</td><td>${n.sequence}</td></tr>`)
    .join('');
  panel.webview.html = `<!DOCTYPE html>
<html>
<head>
  <style>
    body { font-family: sans-serif; padding: 12px; color: #ccc; background: #1e1e1e; }
    table { border-collapse: collapse; width: 100%; margin-top: 12px; }
    th, td { border: 1px solid #444; padding: 6px 8px; text-align: left; }
    th { background: #2d2d2d; }
  </style>
</head>
<body>
  <h2>Snapshot ${manifest.id}</h2>
  <p><strong>Cluster:</strong> ${manifest.cluster}</p>
  <p><strong>Events:</strong> ${eventCount}</p>
  <p><strong>Trigger:</strong> ${(manifest.trigger as { type?: string })?.type ?? 'unknown'}</p>
  <h3>Vector Timeline</h3>
  <table>
    <tr><th>ID</th><th>Node</th><th>Seq</th></tr>
    ${rows || '<tr><td colspan="3">No vector graph in snapshot</td></tr>'}
  </table>
</body>
</html>`;
}

function postDebugMethod(method: string) {
  vscode.window.showInformationMessage(`Debugger stub: ${method}`);
}

export function deactivate() {
  panel?.dispose();
}
