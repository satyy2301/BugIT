import * as vscode from 'vscode';
import { spawn } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

let panel: vscode.WebviewPanel | undefined;
let timeline: Array<{ id: string; node_id: string; sequence: number }> = [];

export function activate(context: vscode.ExtensionContext) {
  context.subscriptions.push(
    vscode.commands.registerCommand('bugit.loadSnapshot', loadSnapshot),
    vscode.commands.registerCommand('bugit.stepForward', () => postDebugMethod('StepForward')),
    vscode.commands.registerCommand('bugit.stepBackward', () => postDebugMethod('StepBackward'))
  );
}

async function loadSnapshot() {
  const uris = await vscode.window.showOpenDialog({
    canSelectMany: false,
    filters: { 'DRE Snapshots': ['dre'] }
  });
  if (!uris || uris.length === 0) {
    return;
  }
  const drePath = uris[0].fsPath;
  const replayBin = process.env.DRE_REPLAY_BIN || 'dre-replay';
  const child = spawn(replayBin, ['load', '--dre', drePath], { shell: true });
  let stdout = '';
  child.stdout.on('data', (d) => (stdout += d.toString()));
  child.stderr.on('data', (d) => vscode.window.showInformationMessage(d.toString().trim()));
  child.on('close', (code) => {
    if (code !== 0) {
      vscode.window.showErrorMessage(`dre-replay failed with code ${code}`);
      return;
    }
    try {
      const manifest = JSON.parse(stdout);
      loadVectorGraph(drePath);
      showPanel(manifest);
      spawn(replayBin, ['run', '--dre', drePath], { shell: true, detached: true });
      vscode.window.showInformationMessage(`Loaded snapshot ${manifest.id}`);
    } catch (err) {
      vscode.window.showErrorMessage(`Failed to parse manifest: ${err}`);
    }
  });
}

function loadVectorGraph(drePath: string) {
  timeline = [];
  const dir = path.dirname(drePath);
  const graphPath = path.join(dir, 'vector_graph.json');
  if (!fs.existsSync(graphPath)) {
    return;
  }
  try {
    const graph = JSON.parse(fs.readFileSync(graphPath, 'utf8'));
    timeline = graph.nodes || [];
  } catch {
    timeline = [];
  }
}

function showPanel(manifest: Record<string, unknown>) {
  if (panel) {
    panel.reveal();
  } else {
    panel = vscode.window.createWebviewPanel('bugitTimeline', 'DRE Timeline', vscode.ViewColumn.Beside, {
      enableScripts: true
    });
    panel.onDidDispose(() => {
      panel = undefined;
    });
  }
  const rows = timeline
    .map((n) => `<tr><td>${n.id}</td><td>${n.node_id}</td><td>${n.sequence}</td></tr>`)
    .join('');
  panel.webview.html = `<!DOCTYPE html>
<html><body>
  <h2>Snapshot ${manifest.id}</h2>
  <p>Cluster: ${manifest.cluster}</p>
  <p>Events: ${manifest.event_count}</p>
  <table border="1" cellpadding="4">
    <tr><th>ID</th><th>Node</th><th>Seq</th></tr>
    ${rows || '<tr><td colspan="3">No vector graph bundled</td></tr>'}
  </table>
</body></html>`;
}

function postDebugMethod(method: string) {
  vscode.window.showInformationMessage(`Debugger stub: ${method}`);
}

export function deactivate() {
  panel?.dispose();
}
