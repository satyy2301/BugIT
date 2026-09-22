# BugIT DRE Engine — VS Code Extension

One-click local bug capture and replay. No CLI install required — `bugit` and `dre-replay` are bundled.

## Quick start

1. Install **BugIT DRE Engine**
2. Open your project folder
3. BugIT sidebar → **Record** → use your app at its normal port → **Stop & Save** → **Replay**

Monorepos: open the whole repo or `backend/` — BugIT auto-detects the Node app folder.

## Sidebar

| Button | Action |
|--------|--------|
| Record | Starts capture (`bugit capture --auto`) |
| Stop & Save | Saves `.bugit/latest.dre` and opens timeline |
| Replay Latest | Opens the latest snapshot |
| Load Demo Snapshot | Bundled checkout demo |

## Commands

| Command | Description |
|---------|-------------|
| BugIT: Record | Start capture |
| BugIT: Stop and Save | Stop capture and open timeline |
| DRE: Open Latest Snapshot | Load `.bugit/latest.dre` |
| DRE: Open Source at Current Event | Jump to file:line from source map |

## Settings

- `bugit.captureRoot` — app folder (e.g. `backend`), auto-written on first Record
- `bugit.publicPort` — public port (default 4000)
- `bugit.devCommand` — dev command (default `npm run dev`)
- `bugit.bugitBin` / `bugit.replayBin` — override bundled binaries
- `bugit.snapshotKey` — AES key for encrypted snapshots

## Publish (maintainers)

```bash
npm run package
npx @vscode/vsce publish -p <PAT>
```
