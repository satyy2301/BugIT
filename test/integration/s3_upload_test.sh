#!/usr/bin/env bash
set -euo pipefail

# MinIO-backed S3 upload integration (optional; skipped when DRE_S3_ENDPOINT unset).

if [[ -z "${DRE_S3_ENDPOINT:-}" ]]; then
  echo "SKIP: DRE_S3_ENDPOINT not set"
  exit 0
fi

export DRE_S3_BUCKET="${DRE_S3_BUCKET:-dre-test}"
export DRE_S3_REGION="${DRE_S3_REGION:-us-east-1}"
export DRE_S3_ACCESS_KEY="${DRE_S3_ACCESS_KEY:-minioadmin}"
export DRE_S3_SECRET_KEY="${DRE_S3_SECRET_KEY:-minioadmin}"
export DRE_S3_FORCE_PATH_STYLE="${DRE_S3_FORCE_PATH_STYLE:-1}"
export DRE_SNAPSHOT_KEY="${DRE_SNAPSHOT_KEY:-ci-test-key}"

go test -v -count=1 ./dre-collector/internal/server/ -run TestSnapshotRoundTrip

echo "S3 env configured; snapshot round-trip test passed (upload when bucket wired in CI)"
