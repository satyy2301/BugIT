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
code --install-extension bugit-dre-1.1.4.vsix
```

Optional CLI for terminal users:

```powershell
bugit doctor
```

## 2. Capture a bug (example: PlacementIQ)

1. Open your project in VS Code (whole repo or `backend/` — both work)
2. **Start your app normally** — run backend and frontend as you always do
3. Open the **BugIT** sidebar → click **Record**
4. BugIT auto-detects your backend port and attaches — inbound API traffic (4xx/5xx) is captured automatically
5. Use your app at its normal URLs (e.g. frontend `http://localhost:3000`, API `http://localhost:4000`)
6. Reproduce the bug
7. Click **Stop & Save** — timeline opens automatically

If nothing is listening on the detected backend port, BugIT falls back to starting the dev server for you.

**MERN monorepos (Next.js + Express):** Next.js often owns Node inspector `:9229`. BugIT automatically patches your backend `dev` script to use inspector `:9230` and a preload hook. On first Record you may see a one-time message — restart the backend (`npm run dev` or `rs` in nodemon), then Record again. API traffic is captured without stopping either server.

**Record vs capture:** Use **Record** (or `bugit record`) when your backend is already running — BugIT attaches to it. Use `bugit capture --auto` only when you want BugIT to start the dev server behind a record proxy.

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
| Stop & save | Sidebar **Stop & Save** | `bugit snapshot` or Ctrl+C in record terminal |
| Force stop | Sidebar **Force Stop** | Kill hung capture/proxy |
| Replay | Sidebar **Replay Latest** | `bugit replay` |
| Status | — | `bugit status` |
| Capture (spawn) | — | `bugit capture --auto` |
| Custom command | — | `bugit capture -- <command...>` |
| List snapshots | — | `bugit snapshots list` |
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
| No snapshot after Stop | Use your app so API requests hit the backend port; retry Stop within 30s of the error |
| Empty timeline / 0 events | Ensure backend is Node >=18; run `bugit doctor`; confirm API calls reach backend (not just frontend) |
| Recording won't stop | Click **Force Stop** |
| Attach failed / inspector :9229 blocked | Run `bugit doctor`; restart backend after BugIT patches dev script (uses :9230 in monorepos) |
| Spawn fallback when backend is up | Run `bugit doctor`; confirm backend port matches frontend `API_URL`; use `bugit record` not `capture --auto` |
| Wrong backend port detected | Set `app_port` in `.bugit/bugit.yaml` or ensure frontend `.env` has `API_URL` with the backend port |

See also [Cloud capture](cloud-capture.md) for production deployments.
