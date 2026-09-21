# Local Quickstart — BugIT Plug-and-Play

Capture and replay bugs on **Windows, macOS, or Linux** with zero code changes.

## 1. Install

```powershell
# CLI (global)
npm install -g @bugit/cli

# VS Code extension
code --install-extension bugit.bugit-dre
```

Or build from source:

```powershell
cd BugIT
make build
# adds bin/bugit.exe and bin/dre-replay.exe
```

Verify:

```powershell
bugit doctor
```

## 2. Capture a bug (example: PlacementIQ)

```powershell
cd C:\Users\satyy\OneDrive\Desktop\satyam\MERN\PlacementIQ\backend
bugit capture -- npm run dev
```

What happens:

1. Creates `.bugit/` in your project (snapshots, config, replay.yaml)
2. Starts local collector + HTTP record proxy
3. Runs your app with `PORT=<internal>` and debug inspector enabled
4. **Send API requests through the record proxy** shown in terminal (`http://127.0.0.1:28081`)
5. On Ctrl+C or HTTP 5xx, saves `.bugit/latest.dre`

## 3. Replay

```powershell
bugit replay
```

Or in VS Code: **DRE: Open Latest Snapshot**

## 4. Jump to source

1. Load snapshot in VS Code
2. Click an event in the timeline (or step with F10)
3. Run **DRE: Open Source at Current Event**

Requires Node `--inspect` during capture (auto-injected by `bugit capture`).

## 5. Commands reference

| Command | Purpose |
|---------|---------|
| `bugit capture -- npm run dev` | Record while dev server runs |
| `bugit snapshot` | Manual snapshot (collector must be running) |
| `bugit replay` | Replay latest `.dre` |
| `bugit open` | Open in VS Code |
| `bugit snapshots list` | List `.bugit/snapshots/` |
| `bugit doctor` | Check install |

## 6. Project layout

```
your-project/
  .bugit/
    bugit.yaml          # ports, keys
    replay.yaml         # auto-generated for replay
    latest.dre          # most recent snapshot
    snapshots/          # all incidents
    data/               # collector buffer (gitignored)
```

## 7. Team / production capture (K8s)

For cluster-wide eBPF capture, use team mode:

```bash
bugit capture --mode cluster   # prints pointer to K8s runbooks
```

See [EKS deploy](runbooks/eks-deploy.md) and [demo walkthrough](demo-walkthrough.md).

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `dre-replay not found` | Run `make build` or install `@bugit/cli` |
| Empty snapshot | Send traffic through record proxy URL |
| No source map | Ensure Node app started via `bugit capture` (inspect auto-enabled) |
| Port in use | Edit `.bugit/bugit.yaml` collector/proxy ports |
