# Changelog

## 1.1.4

- Fix MERN monorepo inspector conflict: when Next.js owns :9229, BugIT auto-patches backend dev script to use inspector :9230+
- Direct HTTP ingest preload (`.bugit/preload.cjs`) — captures inbound API traffic after one backend restart without CDP
- Target diff polling prefers newly appeared backend targets on non-9229 ports
- Removed harmful `capture --auto` fallback when attach fails and backend is already running (no more EADDRINUSE crash)
- `bugit doctor` shows inspect bootstrap/preload status and recommended restart command
- VS Code shows actionable restart guidance and optional copy restart command

## 1.1.3

- Fix attach attaching to Next.js inspector on :9229 instead of Express backend — always enables backend inspector and resolves target by file path
- Scan inspector ports 9229–9239; score targets by backend `server.js` / capture root path; disqualify Next.js/:3000 targets
- Post-inject health check (hook ready + binding ping) before recording starts
- Log chosen inspector target; improved doctor output listing all targets with scores
- NodeBridge reuses the same backend inspector WebSocket URL as attach capture

## 1.1.2

- Attach mode captures **inbound backend API traffic** (4xx/5xx) while backend + frontend keep running
- ServerTap via Node `diagnostics_channel` — full HTTP request/response payloads in snapshots
- Smarter Node inspector target selection (backend PID/port vs Next.js dev server)
- Ingest errors logged instead of silently dropped; clearer Record status and zero-event warnings
- `bugit doctor` reports Node version, backend port, and inspector readiness
- Optional `trigger_4xx: true` in `.bugit/bugit.yaml` for auto-snapshot on 4xx responses

## 1.1.1

- Fix stale bundled CLI: packaging now rebuilds and verifies binaries before VSIX publish
- Pre-flight binary check before Record — clear error if CLI lacks `record` or version mismatch
- `BugIT: Doctor` command reports binary path, version, and record support
- Capture fallback to `bugit capture --auto` if `record` exits immediately
- Improved spawn/exit error handling in capture manager

## 1.1.0

- Attach-mode recording: run app normally, click Record (`bugit record`)
- Auto port discovery from `.env` / monorepo layout
- Force Stop sidebar button
- Fixed reverse proxy response body drain (Failed to fetch)
- Reliable Stop & Save with explicit snapshot trigger
- Auto-sync `bugit.yaml` on Record

## 1.0.0

- Plug-and-play local capture via `bugit capture`
- Bundled dre-replay binary support
- Jump to source from `source_map.json`
- Open Latest Snapshot from `.bugit/latest.dre`
- Start Capture terminal task
- First-run walkthrough

## 0.3.0

- Timeline v2, Mermaid graph, DAP replay debugging
