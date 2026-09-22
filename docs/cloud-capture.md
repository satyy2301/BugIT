# Cloud capture — production deployments

Local **attach mode** (`bugit record`) discovers ports from your workspace and taps HTTP via Node CDP. In Kubernetes or cloud VMs, use **dre-agent** instead — no per-service port configuration in VS Code.

## Mental model

| Environment | How capture works |
|-------------|-------------------|
| Local dev | `bugit record` → attach to running Node backend |
| Kubernetes | `dre-agent` sidecar → eBPF/socket probes → collector |
| VM / bare metal | `dre-agent` on host or systemd service |

## Kubernetes (EKS/GKE)

1. Deploy collector from `deploy/helm/dre-collector`
2. Add `dre-agent` to your backend pod (see `deploy/helm/dre-agent`)
3. Agent forwards IO events to collector gRPC — auto-triggers on 5xx, SIGSEGV, process exit
4. Fetch snapshots: **DRE: Fetch Latest** or `dre-cli` against collector HTTP API

No `bugit.yaml` or `bugit.publicPort` in the cluster. Service ports come from pod networking and Helm values.

## VS Code in cloud-connected workflows

1. Port-forward collector: `kubectl port-forward svc/dre-collector 28080:8080`
2. Set `bugit.collectorUrl` to `http://127.0.0.1:28080` (optional)
3. **DRE: Fetch Latest Snapshot** loads production captures into the timeline

## Roadmap alignment

- Phase 5 PRD: mTLS agent→collector, Grafana dashboards, Helm overlays — see `docs/PRD-v2.md`
- Local attach and cloud agent share the same `.dre` snapshot format and replay path
