# DRE monitoring

## Grafana dashboards

Import JSON from `dashboards/`:

1. Grafana → Dashboards → New → Import
2. Upload `dre-agent.json` and `dre-collector.json`
3. Select Prometheus data source

## Prometheus alerts

Apply rules from `alerts/`:

```bash
kubectl apply -f deploy/monitoring/alerts/ -n monitoring
```

Or merge into your Prometheus `rule_files` / Prometheus Operator `PrometheusRule`.

## Metrics endpoints

| Workload | Port | Path |
|----------|------|------|
| dre-agent | 9100 | `/metrics` |
| dre-collector | 8081 | `/metrics` |

## Key metrics

- `dre_events_emitted_total` / `dre_ringbuf_drops_total` — capture health
- `dre_bypass_mode` — agent bypass active
- `dre_collector_events_ingested_total` — ingest rate
- `dre_collector_snapshots_total` — export count
- `dre_collector_snapshot_export_seconds` — export latency histogram
- `dre_collector_is_leader` — HA leader status
