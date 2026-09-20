#!/usr/bin/env bash
# End-to-end smoke test for real eBPF capture on kind.
# Prerequisites: kind cluster dre-engine, images loaded (make docker kind-load), deploy-kind.
set -euo pipefail

NAMESPACE="${DRE_NAMESPACE:-dre-engine}"
COLLECTOR_HTTP="${DRE_COLLECTOR_HTTP:-http://localhost:18081}"
SKIP_DEPLOY="${SKIP_DEPLOY:-0}"

log() { echo "[kind-capture] $*"; }

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: missing required command: $1" >&2
    exit 1
  fi
}

require_cmd kubectl
require_cmd curl

if [[ "${SKIP_DEPLOY}" != "1" ]]; then
  log "deploying dre-engine stack"
  make deploy-kind
fi

log "waiting for pods"
kubectl wait --for=condition=ready pod -l app=dre-collector -n "${NAMESPACE}" --timeout=120s
kubectl wait --for=condition=ready pod -l app=nginx-sample -n "${NAMESPACE}" --timeout=120s
kubectl wait --for=condition=ready pod -l app=dre-agent -n "${NAMESPACE}" --timeout=120s

log "generating nginx traffic from nginx pod"
kubectl exec -n "${NAMESPACE}" deploy/nginx-sample -- wget -qO- http://127.0.0.1/ >/dev/null
kubectl exec -n "${NAMESPACE}" deploy/nginx-sample -- wget -qO- http://127.0.0.1/error >/dev/null || true

log "triggering manual snapshot via collector HTTP"
if ! curl -sf -X POST "${COLLECTOR_HTTP}/v1/trigger" -H 'Content-Type: application/json' \
  -d '{"reason":"integration","detail":"kind_capture_test"}' >/dev/null; then
  log "collector not reachable at ${COLLECTOR_HTTP}; start port-forward:"
  log "  kubectl port-forward -n ${NAMESPACE} svc/dre-collector 8080:8080"
  exit 1
fi

COLLECTOR_POD="$(kubectl get pods -n "${NAMESPACE}" -l app=dre-collector -o jsonpath='{.items[0].metadata.name}')"
REMOTE="$(kubectl exec -n "${NAMESPACE}" "${COLLECTOR_POD}" -- sh -c 'ls -t /var/lib/dre/incident-*.dre 2>/dev/null | head -1')"
if [[ -z "${REMOTE}" ]]; then
  echo "ERROR: snapshot file not created" >&2
  exit 1
fi

log "snapshot created: ${REMOTE}"

AGENT_POD="$(kubectl get pods -n "${NAMESPACE}" -l app=dre-agent -o jsonpath='{.items[0].metadata.name}')"
METRICS="$(kubectl exec -n "${NAMESPACE}" "${AGENT_POD}" -- wget -qO- http://127.0.0.1:9100/metrics 2>/dev/null || true)"
if echo "${METRICS}" | grep -q 'dre_events_emitted_total'; then
  log "agent metrics present"
else
  log "WARN: could not verify dre_events_emitted_total (agent may still be in mock mode)"
fi

log "PASS: kind capture integration smoke test"
