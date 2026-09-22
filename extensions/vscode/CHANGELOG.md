# Changelog

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
