#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${DRE_NAMESPACE:-dre-engine}"
COLLECTOR_HTTP="${DRE_COLLECTOR_HTTP:-http://localhost:8080}"
OUT="${1:-./latest.dre}"

if command -v curl >/dev/null 2>&1; then
  LIST="$(curl -sf "${COLLECTOR_HTTP}/v1/snapshots")"
  ID="$(printf '%s' "${LIST}" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | tail -1)"
  if [[ -z "${ID}" ]]; then
    echo "ERROR: no snapshots from ${COLLECTOR_HTTP}/v1/snapshots" >&2
    exit 1
  fi
  curl -sf "${COLLECTOR_HTTP}/v1/snapshots/${ID}/download" -o "${OUT}"
  echo "Downloaded snapshot ${ID} -> ${OUT}"
  exit 0
fi

POD="$(kubectl get pods -n "${NAMESPACE}" -l app=dre-collector -o jsonpath='{.items[0].metadata.name}')"
if [[ -z "${POD}" ]]; then
  echo "ERROR: dre-collector pod not found in namespace ${NAMESPACE}" >&2
  exit 1
fi

REMOTE="$(kubectl exec -n "${NAMESPACE}" "${POD}" -- sh -c 'ls -t /var/lib/dre/incident-*.dre 2>/dev/null | head -1' 2>/dev/null || true)"
if [[ -z "${REMOTE}" ]]; then
  echo "ERROR: no .dre snapshots; set DRE_COLLECTOR_HTTP and use curl, or port-forward collector" >&2
  exit 1
fi

kubectl cp -n "${NAMESPACE}" "${POD}:${REMOTE}" "${OUT}"
echo "Fetched ${REMOTE} -> ${OUT}"
