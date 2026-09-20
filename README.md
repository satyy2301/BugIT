# BugIT — DRE-Engine

Deterministic Replay Environment for Kubernetes microservices. Captures kernel-level I/O and scheduling events via eBPF, aggregates them cluster-wide, and replays incidents locally.

## Components

| Component | Description |
|-----------|-------------|
| `dre-agent` | eBPF DaemonSet + userspace loader |
| `dre-collector` | Event aggregator, vector clock engine, `.dre` exporter |
| `dre-replay` | Local offline replay proxy and debugger sync API |
| `dre-cli` | Trigger and download snapshots |
| `extensions/vscode` | VS Code snapshot loader and timeline panel |

## Quick Start (Linux + kind)

```bash
bash scripts/setup-dev.sh
make build
bash scripts/kind-up.sh
make docker
make deploy-kind
dre-cli trigger --collector http://localhost:8080
```

## Demo snapshot (try first)

```powershell
go run ./scripts/generate-demo-snapshot -out test/fixtures/demo-checkout-500.dre
# VS Code: extensions/vscode → F5 → DRE: Load Snapshot → pick demo-checkout-500.dre
```

See [Demo walkthrough](docs/demo-walkthrough.md) for what the bug is and how replay works.

## Build

```bash
make build          # all Go binaries
make bpf            # eBPF objects (Linux only)
make demo-snapshot  # generate test/fixtures/demo-checkout-500.dre
make test
```

## Documentation

- [PRD](docs/PRD.md)
- [Demo walkthrough](docs/demo-walkthrough.md)
- [Dev environments](docs/dev-environments.md)
