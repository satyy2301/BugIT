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

Run on **WSL2 Ubuntu** or Linux (eBPF capture does not run on Windows-native).

```bash
bash scripts/setup-dev.sh
make build-linux          # bpf + Go binaries
bash scripts/kind-up.sh
make docker
make kind-load            # load images into kind
make deploy-kind
kubectl port-forward -n dre-engine svc/dre-collector 18081:8080 &
kubectl exec -n dre-engine deploy/nginx-sample -- wget -qO- http://127.0.0.1/error || true
DRE_COLLECTOR_HTTP=http://localhost:18081 bin/dre-cli trigger --collector http://localhost:18081
DRE_COLLECTOR_HTTP=http://localhost:18081 make fetch-snapshot   # copies latest .dre to ./latest.dre
```

On Windows, port `8080` is often reserved — use `18081` (or any free local port) for port-forward.

Integration smoke test (kind cluster required):

```bash
bash test/integration/kind_capture_test.sh
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
- [PRD v2 (Phase 1+)](docs/PRD-v2.md)
- [Demo walkthrough](docs/demo-walkthrough.md)
- [Dev environments](docs/dev-environments.md)
