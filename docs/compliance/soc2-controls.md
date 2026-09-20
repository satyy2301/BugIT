# SOC2 control mapping (internal beta)

This document maps DRE-Engine controls to SOC2 Trust Services Criteria for **internal beta readiness**. It is not a substitute for a Type II audit.

## CC6 — Logical and physical access

| Control | Implementation |
|---------|----------------|
| CC6.1 | IRSA (EKS) / Workload Identity (GKE) for collector S3/GCS access; no long-lived cloud keys in pods |
| CC6.2 | Kubernetes RBAC for leader election leases; namespace isolation |
| CC6.3 | Snapshot encryption key in Kubernetes Secret; not in git |
| CC6.6 | mTLS optional for agent→collector gRPC (`grpc.tls.enabled`) |

## CC7 — System operations

| Control | Implementation |
|---------|----------------|
| CC7.1 | Prometheus metrics + Grafana dashboards (`deploy/monitoring/`) |
| CC7.2 | Alert rules for bypass, drop rate, snapshot failures |
| CC7.3 | Runbooks for bypass, snapshot failure, deploy, rollback |
| CC7.4 | CI gates: unit tests, replay e2e, storage integration |

## CC8 — Change management

| Control | Implementation |
|---------|----------------|
| CC8.1 | GitLab CI pipeline with manual kind/perf jobs |
| CC8.1 | Helm revision rollback documented in `docs/runbooks/upgrade-rollback.md` |

## Evidence collection (beta)

- Export Grafana snapshots monthly
- Retain CI job logs for integration tests
- Document incident response using runbooks
