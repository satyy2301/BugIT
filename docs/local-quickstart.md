# Local Quickstart — BugIT One-Click

Capture and replay bugs on **Windows, macOS, or Linux** with zero code changes.

## 1. Install

In VS Code or Cursor: **Extensions** → search **BugIT DRE Engine** → **Install**

Or from source:

```powershell
cd BugIT
.\scripts\build.ps1
cd extensions\vscode
npm install
npm run package
code --install-extension bugit-dre-1.1.0.vsix
```

Optional CLI for terminal users:

```powershell
bugit doctor
```

## 2. Capture a bug (example: PlacementIQ)

1. Open your project in VS Code (whole repo or `backend/` — both work)
2. **Start your app normally** — run backend and frontend as you always do
3. Open the **BugIT** sidebar → click **Record**
4. BugIT auto-detects your backend port and attaches (no manual `bugit.yaml` edits)
5. Use your app at its normal URLs (e.g. frontend `http://localhost:3000`, API `http://localhost:4000`)
6. Reproduce the bug
7. Click **Stop & Save** — timeline opens automatically

If nothing is listening on the detected backend port, BugIT falls back to starting the dev server for you.

## 3. Replay

Click **Replay Latest** in the sidebar, or run **DRE: Open Latest Snapshot**.

Terminal fallback:

```powershell
bugit replay
```

## 4. Jump to source

1. Click an event in the timeline (or step with F10)
2. Run **DRE: Open Source at Current Event**

Works best when Node inspector is available (BugIT can enable it on the running backend).

## 5. Commands reference

| Action | VS Code | CLI (optional) |
|--------|---------|----------------|
| Record | Sidebar **Record** | `bugit record` |
| Stop & save | Sidebar **Stop & Save** | Ctrl+C in record terminal |
| Force stop | Sidebar **Force Stop** | Kill hung capture/proxy |
| Replay | Sidebar **Replay Latest** | `bugit replay` |
| Doctor | — | `bugit doctor` |

## 6. Project layout

```
your-project/
  backend/                # auto-detected in monorepos
    .bugit/
      bugit.yaml          # auto-written on Record
      replay.yaml
      latest.dre
      snapshots/
```

## Troubleshooting

| Issue | Fix |
|-------|-----|
| Sidebar buttons do nothing | Reload VS Code; ensure a folder is open |
| No snapshot after Stop | Send at least one API request while recording; retry Stop |
| Recording won't stop | Click **Force Stop** |
| Attach failed | Ensure backend is running; try `NODE_OPTIONS=--inspect` on backend |
| Spawn fallback when backend is up | Check backend port matches `.env` / `NEXT_PUBLIC_API_URL` |

See also [Cloud capture](cloud-capture.md) for production deployments.
