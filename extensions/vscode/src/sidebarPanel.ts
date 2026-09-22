import * as vscode from 'vscode';
import { CaptureState, isRecording, onCaptureStatus, startCapture, stopCapture } from './captureManager';

export class BugitSidebarProvider implements vscode.WebviewViewProvider {
  public static readonly viewType = 'bugit.panel';

  constructor(private readonly extensionContext: vscode.ExtensionContext) {}

  resolveWebviewView(
    webviewView: vscode.WebviewView,
    _context: vscode.WebviewViewResolveContext,
    _token: vscode.CancellationToken,
  ): void {
    webviewView.webview.options = { enableScripts: true };
    webviewView.webview.html = this.renderHtml('idle', 'Ready');

    webviewView.webview.onDidReceiveMessage(async (msg) => {
      if (msg.type === 'record') {
        await vscode.commands.executeCommand('bugit.startRecord');
      } else if (msg.type === 'stop') {
        await vscode.commands.executeCommand('bugit.stopRecord');
      } else if (msg.type === 'replay') {
        await vscode.commands.executeCommand('bugit.openLatest');
      } else if (msg.type === 'demo') {
        await vscode.commands.executeCommand('bugit.loadSnapshot');
      }
    });

    onCaptureStatus((state, msg) => {
      webviewView.webview.html = this.renderHtml(state, msg);
    });
  }

  private renderHtml(state: CaptureState, message: string): string {
    const recording = state === 'recording';
    return `<!DOCTYPE html>
<html><head><meta charset="UTF-8">
<style>
  body { font-family: var(--vscode-font-family); padding: 12px; color: var(--vscode-foreground); }
  h2 { font-size: 14px; margin: 0 0 8px; }
  p { font-size: 12px; opacity: 0.9; line-height: 1.4; }
  button { width: 100%; margin: 6px 0; padding: 10px; border: none; border-radius: 4px; cursor: pointer;
    background: var(--vscode-button-background); color: var(--vscode-button-foreground); font-size: 13px; }
  button.secondary { background: var(--vscode-button-secondaryBackground); color: var(--vscode-button-secondaryForeground); }
  button:disabled { opacity: 0.5; cursor: default; }
  .status { background: var(--vscode-editor-inactiveSelectionBackground); padding: 10px; border-radius: 4px; margin-bottom: 12px; font-size: 12px; }
</style></head><body>
  <h2>BugIT</h2>
  <div class="status">${escapeHtml(message)}</div>
  <button id="record" ${recording ? 'disabled' : ''}>Record</button>
  <button id="stop" class="secondary" ${recording ? '' : 'disabled'}">Stop &amp; Save</button>
  <button id="replay" class="secondary">Replay Latest</button>
  <button id="demo" class="secondary">Load Demo Snapshot</button>
  <p>Record → use your app normally → Stop → Replay</p>
<script>
  const vscode = acquireVsCodeApi();
  document.getElementById('record')?.addEventListener('click', () => vscode.postMessage({ type: 'record' }));
  document.getElementById('stop')?.addEventListener('click', () => vscode.postMessage({ type: 'stop' }));
  document.getElementById('replay')?.addEventListener('click', () => vscode.postMessage({ type: 'replay' }));
  document.getElementById('demo')?.addEventListener('click', () => vscode.postMessage({ type: 'demo' }));
</script>
</body></html>`;
  }
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

export function registerSidebar(context: vscode.ExtensionContext) {
  const provider = new BugitSidebarProvider(context);
  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider(BugitSidebarProvider.viewType, provider),
  );
}

export { isRecording };
