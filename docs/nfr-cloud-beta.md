# Cloud beta NFRs

Targets for Phase 5 internal beta on EKS/GKE.

| Metric | Target | Verification |
|--------|--------|--------------|
| Agent CPU (baseline) | < 2% of one core | `kubectl top pod -l app=dre-agent` under idle cluster |
| Agent RSS | < 128 MiB request, < 512 MiB limit | Helm resources + `kubectl top` |
| Ringbuf drop rate | < 1% at 1k req/s | `bash test/perf/agent_load.sh` |
| Collector snapshot export p99 | < 30s for 10k events | `dre_collector_snapshot_export_seconds` histogram |
| Replay debugger Seek latency | < 100ms | `test/integration/replay_e2e.sh` GetState round-trip |
| Snapshot durability | 100% upload to S3/GCS after trigger | `integration:storage` CI job |

## Perf gates

```bash
# Requires kind cluster with dre-engine deployed
bash test/perf/load_test.sh
```

JSON summary written to `test/perf/results.json` when `PERF_JSON=1`.

## Observability

- Grafana dashboards: `deploy/monitoring/dashboards/`
- Alert rules: `deploy/monitoring/alerts/`
- Fail nightly `perf:nightly` job if drop rate exceeds threshold
