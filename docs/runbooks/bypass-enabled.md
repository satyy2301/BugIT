# Runbook: Agent bypass enabled

## Symptoms

- `dre_bypass_mode == 1` in metrics or Grafana alert **DreAgentBypassEnabled**
- Missing events in collector; empty or stale snapshots

## Diagnosis

1. Check agent logs: `kubectl logs -n dre-engine -l app=dre-agent --tail=100`
2. Check RSS vs limit: `kubectl top pod -n dre-engine -l app=dre-agent`
3. Check drop rate: `dre_ringbuf_drops_total` / `dre_events_emitted_total`

## Remediation

1. **Memory pressure** — increase `agent.resources.limits.memory` or reduce node load
2. **High drop rate** — lower traffic to pod, increase ringbuf (`DRE_RINGBUF_SIZE_MB`), or scale workloads
3. **Restart agent** — `kubectl rollout restart ds/dre-agent -n dre-engine` after fixing root cause
4. Bypass clears when pressure subsides; verify `dre_bypass_mode` returns to 0

## Prevention

- Run `bash test/perf/agent_load.sh` before production traffic increases
- Set alerts from `deploy/monitoring/alerts/dre-agent.yaml`
