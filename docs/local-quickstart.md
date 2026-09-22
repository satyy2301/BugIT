# Local Quickstart — BugIT One-Click

Capture and replay bugs on **Windows, macOS, or Linux** with zero code changes.

## 1. Install

In VS Code: **Extensions** → search **BugIT DRE Engine** → **Install**

Or from source:

```powershell
cd BugIT
.\scripts\build.ps1
cd extensions\vscode
npm install
npm run package
code --install-extension bugit-dre-1.0.0.vsix
```

Optional CLI for terminal users:

```powershell
npm install -g @bugit/cli
bugit doctor
```

## 2. Capture a bug (example: PlacementIQ)

1. Open your project in VS Code (whole repo or `backend/` — both work)
2. Open the **BugIT** sidebar
3. Click **Record**
4. Use your app at its normal URL (e.g. `http://localhost:4000/api/...`)
5. Reproduce the bug
6. Click **Stop & Save** — timeline opens automatically

BugIT auto-detects `backend/` in monorepos and writes `.vscode/settings.json` on first Record.

## 3. Replay

Click **Replay Latest** in the sidebar, or run **DRE: Open Latest Snapshot**.

Terminal fallback:

```powershell
bugit replay
```

## 4. Jump to source

1. Click an event in the timeline (or step with F10)
2. Run **DRE: Open Source at Current Event**

Requires Node `--inspect` during capture (auto-injected by BugIT).

## 5. Commands reference

| Action | VS Code | CLI (optional) |
|--------|---------|----------------|
| Record | Sidebar **Record** | `bugit capture --auto` |
| Stop & save | Sidebar **Stop & Save** | Ctrl+C in capture terminal |
| Replay | Sidebar **Replay Latest** | `bugit replay` |
| Doctor | — | `bugit doctor` |

## 6. Project layout

```
your-project/
  .vscode/settings.json   # bugit.captureRoot, publicPort (auto-written)
  backend/                # auto-detected in monorepos
    .bugit/
      bugit.yaml
      replay.yaml
      latest.dre
      snapshots/
```

## Troubleshooting

| Issue | Fix |
|-------|-----|
| Sidebar buttons do nothing | Reload VS Code; ensure a folder is open |
| No snapshot after Stop | Send at least one HTTP request to your app while recording |
| Wrong app folder | Set `bugit.captureRoot` in settings (e.g. `backend`) |
| Port in use | Stop other dev servers on the same port before Record |
