#!/usr/bin/env bash
set -euo pipefail

OUT="${1:-dre-agent/bpf/vmlinux.h}"

if [[ ! -f /sys/kernel/btf/vmlinux ]]; then
  echo "ERROR: /sys/kernel/btf/vmlinux not found"
  exit 1
fi

bpftool btf dump file /sys/kernel/btf/vmlinux format c > "$OUT"
echo "Generated $OUT"
