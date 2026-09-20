#!/usr/bin/env bash
set -euo pipefail

# Wrapper: run agent_load.sh when kind cluster is available, else skip.

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NAMESPACE="${NAMESPACE:-dre-engine}"

if ! kubectl get ns "$NAMESPACE" &>/dev/null; then
  echo "SKIP: namespace $NAMESPACE not found (deploy kind cluster first)"
  exit 0
fi

export PERF_JSON="${PERF_JSON:-1}"
export PERF_JSON_PATH="${PERF_JSON_PATH:-$ROOT/test/perf/results.json}"
bash "$ROOT/test/perf/agent_load.sh"
