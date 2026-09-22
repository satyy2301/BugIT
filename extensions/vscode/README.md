# BugIT DRE Engine — VS Code Extension

One-click local bug capture and replay. No CLI install required — `bugit` and `dre-replay` are bundled.

## Quick start

1. Install **BugIT DRE Engine**
2. Open your project folder
3. **Run your app normally** (backend + frontend)
4. BugIT sidebar → **Record** → use your app → **Stop & Save** → **Replay**

Monorepos: open the whole repo — BugIT auto-detects `backend/` and API ports from `.env`.

## Sidebar

| Button | Action |
|--------|--------|
| Record | Attach to running backend (`bugit record`) or spawn fallback |
| Stop & Save | Save snapshot and open timeline |
| Force Stop | Kill recording/proxy even if hung |
| Replay Latest | Opens the latest snapshot |
| Load Demo Snapshot | Bundled checkout demo |

## Commands

| Command | Description |
|---------|-------------|
| BugIT: Record | Start attach-mode recording |
| BugIT: Stop and Save | Stop and open timeline |
| BugIT: Force Stop Recording | Force-kill capture session |
| DRE: Open Latest Snapshot | Load `.bugit/latest.dre` |
| DRE: Open Source at Current Event | Jump to file:line from source map |

## Settings (advanced overrides only)

- `bugit.captureRoot` — app folder (auto-detected)
- `bugit.publicPort` — backend port hint (auto-detected from `.env`)
- `bugit.bugitBin` / `bugit.replayBin` — override bundled binaries

## Publish (maintainers)

```powershell
npm run package
$env:VSCE_PAT = "your-token"
npx @vscode/vsce publish --no-dependencies
```
