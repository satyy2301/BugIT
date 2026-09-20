#!/usr/bin/env bash
set -euo pipefail

# REQ-EBPF-003: validate ringbuf drop rate under sustained load.
# Requires: kind cluster with dre-agent + nginx-sample deployed.

NAMESPACE="${NAMESPACE:-dre-engine}"
DURATION="${DURATION:-60}"
TARGET_RPS="${TARGET_RPS:-1000}"
MAX_DROP_PCT="${MAX_DROP_PCT:-1}"

AGENT_POD="$(kubectl get pods -n "$NAMESPACE" -l app=dre-agent -o jsonpath='{.items[0].metadata.name}')"
if [[ -z "$AGENT_POD" ]]; then
  echo "no dre-agent pod found in $NAMESPACE"
  exit 1
fi

drops_before="$(kubectl exec -n "$NAMESPACE" "$AGENT_POD" -- wget -qO- http://127.0.0.1:9100/metrics | grep '^dre_ringbuf_drops_total' | awk '{print $2}' || echo 0)"
events_before="$(kubectl exec -n "$NAMESPACE" "$AGENT_POD" -- wget -qO- http://127.0.0.1:9100/metrics | grep '^dre_events_emitted_total' | awk '{print $2}' || echo 0)"

echo "load test: ${TARGET_RPS} req/s for ${DURATION}s against nginx-sample"
kubectl run -n "$NAMESPACE" loadgen --rm -i --restart=Never --image=williamyeh/wrk:latest -- \
  wrk -t4 -c64 -d"${DURATION}s" -R"${TARGET_RPS}" http://nginx-sample/ || true

drops_after="$(kubectl exec -n "$NAMESPACE" "$AGENT_POD" -- wget -qO- http://127.0.0.1:9100/metrics | grep '^dre_ringbuf_drops_total' | awk '{print $2}' || echo 0)"
events_after="$(kubectl exec -n "$NAMESPACE" "$AGENT_POD" -- wget -qO- http://127.0.0.1:9100/metrics | grep '^dre_events_emitted_total' | awk '{print $2}' || echo 0)"

drops=$((drops_after - drops_before))
events=$((events_after - events_before))
total=$((drops + events))
if [[ "$total" -eq 0 ]]; then
  echo "no events observed; check agent deployment"
  exit 1
fi

drop_pct=$((drops * 100 / total))
echo "drops=$drops events=$events drop_pct=${drop_pct}%"
passed=true
if [[ "$drop_pct" -gt "$MAX_DROP_PCT" ]]; then
  echo "drop rate ${drop_pct}% exceeds ${MAX_DROP_PCT}% threshold"
  passed=false
fi

if [[ "${PERF_JSON:-}" == "1" ]]; then
  json_path="${PERF_JSON_PATH:-test/perf/results.json}"
  mkdir -p "$(dirname "$json_path")"
  cat >"$json_path" <<EOF
{"drops":$drops,"events":$events,"drop_pct":$drop_pct,"max_drop_pct":$MAX_DROP_PCT,"target_rps":$TARGET_RPS,"duration_s":$DURATION,"passed":$passed}
EOF
  echo "wrote $json_path"
fi

if [[ "$passed" != true ]]; then
  exit 1
fi
echo "load test passed"
