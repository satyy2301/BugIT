# BugIT — DRE Engine

Deterministic Replay Environment for local development and Kubernetes microservices. **Zero-code capture** on Windows/macOS/Linux, plus eBPF cluster capture for production.

## Quick Start (local — any OS)

```powershell
npm install -g @bugit/cli
cd your-project
bugit capture -- npm run dev
# reproduce bug, Ctrl+C
bugit replay
```

VS Code: install **BugIT DRE Engine** extension → **DRE: Open Latest Snapshot**

Full walkthrough: [Local quickstart](docs/local-quickstart.md)

Example project:

```powershell
cd C:\Users\satyy\OneDrive\Desktop\satyam\MERN\PlacementIQ\backend
bugit capture -- npm run dev
```

## Build from source

```bash
make build          # bugit, dre-replay, dre-collector, dre-cli, dre-agent
make demo-snapshot  # test/fixtures/demo-checkout-500.dre
```

Windows: eBPF capture needs WSL2; **local capture works natively**.

## Components

| Component | Description |
|-----------|-------------|
| `bugit` | Unified CLI — capture, replay, doctor |
| `dre-replay` | Local replay proxy + debugger sync |
| `dre-collector` | Event aggregator + `.dre` export |
| `dre-agent` | eBPF capture (Linux/K8s team mode) |
| `extensions/vscode` | Timeline UI, jump-to-source, walkthrough |

## Team / production (K8s + eBPF)

```bash
bash scripts/setup-dev.sh
make build-linux docker kind-load deploy-kind
```

See [EKS deploy](docs/runbooks/eks-deploy.md). Use `bugit capture --mode cluster` for runbook pointer.

## Demo snapshot

```powershell
go run ./scripts/generate-demo-snapshot -out test/fixtures/demo-checkout-500.dre
# VS Code: DRE: Load Snapshot
```

## Documentation

- [Local quickstart](docs/local-quickstart.md) — **start here**
- [Demo walkthrough](docs/demo-walkthrough.md)
- [PRD v2](docs/PRD-v2.md)
- [Dev environments](docs/dev-environments.md)
