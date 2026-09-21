# BugIT DRE Engine — VS Code Extension

Capture, replay, and debug bugs locally with zero code changes.

## Quick start

1. Install the **BugIT DRE Engine** extension
2. Install CLI: `npm install -g @bugit/cli`
3. In your project terminal: `bugit capture -- npm run dev`
4. Reproduce the bug, then press Ctrl+C
5. Command Palette → **DRE: Open Latest Snapshot**

## Commands

| Command | Description |
|---------|-------------|
| DRE: Load Snapshot | Pick a `.dre` file |
| DRE: Open Latest Snapshot | Load `.bugit/latest.dre` |
| DRE: Start Capture | Run `bugit capture -- npm run dev` |
| DRE: Open Source at Current Event | Jump to file:line from source map |

## Settings

- `bugit.replayBin` — path to dre-replay (auto-detects bundled binary)
- `bugit.replayConfig` — replay.yaml (defaults to `.bugit/replay.yaml`)
- `bugit.appBinary` — Go binary for Delve attach during replay
- `bugit.snapshotKey` — AES key for encrypted snapshots

## Requirements

Bundled `dre-replay` binary (platform-specific) or build from [BugIT repo](https://gitlab.com/bugit/dre-engine).
