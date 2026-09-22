# BugIT — DRE Engine

Deterministic Replay Environment for local development and Kubernetes microservices. **One-click capture** in VS Code on Windows/macOS/Linux, plus eBPF cluster capture for production.

## Quick Start (VS Code — 3 steps)

1. Install **BugIT DRE Engine** from the VS Code Marketplace
2. Open your project folder
3. BugIT sidebar → **Record** → use your app normally → **Stop & Save** → **Replay**

Full walkthrough: [Local quickstart](docs/local-quickstart.md)

Example (PlacementIQ monorepo — open whole repo or `backend/`):

```
BugIT sidebar → Record → http://localhost:4000 → Stop & Save
```

## Optional CLI

```powershell
npm install -g @bugit/cli
cd your-project
bugit capture --auto
bugit replay
```

## Build from source

```powershell
.\scripts\build.ps1          # Windows: bugit.exe + dre-replay.exe + extension bin bundle
```

```bash
make build                   # Linux/macOS
bash scripts/bundle-extension-binaries.sh
```

Windows: eBPF capture needs WSL2; **local capture works natively**.

## Components

| Component | Description |
|-----------|-------------|
| `extensions/vscode` | **Primary UX** — sidebar Record/Stop/Replay, timeline, jump-to-source |
| `bugit` | Unified CLI — capture, replay, doctor (optional) |
| `dre-replay` | Local replay proxy + debugger sync |
| `dre-collector` | Event aggregator + `.dre` export |
| `dre-agent` | eBPF capture (Linux/K8s team mode) |

## Team / production (K8s + eBPF)

```bash
bash scripts/setup-dev.sh
make build-linux docker kind-load deploy-kind
```

See [EKS deploy](docs/runbooks/eks-deploy.md). Use `bugit capture --mode cluster` for runbook pointer.

## Demo snapshot

```powershell
go run ./scripts/generate-demo-snapshot -out test/fixtures/demo-checkout-500.dre
# VS Code: BugIT sidebar → Load Demo Snapshot
```

## Documentation

- [Local quickstart](docs/local-quickstart.md) — **start here**
- [Demo walkthrough](docs/demo-walkthrough.md)
- [PRD v2](docs/PRD-v2.md)
- [Dev environments](docs/dev-environments.md)

## Publish extension (maintainers)

```powershell
cd extensions/vscode
npm run package
npx @vscode/vsce publish -p <PAT>
```
