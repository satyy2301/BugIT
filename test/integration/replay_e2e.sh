#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FIXTURE="${ROOT}/test/fixtures/demo-checkout-500.dre"
REPLAY="${ROOT}/bin/dre-replay"
KEY="${DRE_SNAPSHOT_KEY:-dev-insecure-key-change-me}"

if [[ ! -f "$FIXTURE" ]]; then
  echo "SKIP: fixture missing; run make demo-snapshot"
  exit 0
fi

if [[ ! -x "$REPLAY" ]]; then
  go build -o "$REPLAY" ./dre-replay-cli/cmd/dre-replay
fi

PROXY_PORT="${PROXY_PORT:-18080}"
DEBUG_PORT="${DEBUG_PORT:-19090}"

"$REPLAY" run --dre "$FIXTURE" --key "$KEY" --proxy "127.0.0.1:${PROXY_PORT}" &
PID=$!
trap 'kill $PID 2>/dev/null || true' EXIT

sleep 1
RESP=$(printf 'POST /charge HTTP/1.1\r\nHost: payment-service\r\n\r\n' | nc -w 2 127.0.0.1 "$PROXY_PORT" || true)
if [[ "$RESP" != *"500"* ]]; then
  echo "expected HTTP 500 in proxy response, got: $RESP"
  exit 1
fi

STATE=$(printf '{"method":"GetState"}\n' | nc -w 2 127.0.0.1 "$DEBUG_PORT" || true)
if [[ -z "$STATE" ]]; then
  echo "debugger API not reachable on :${DEBUG_PORT}"
  exit 1
fi

echo "replay e2e passed"
