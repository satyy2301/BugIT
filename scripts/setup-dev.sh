#!/usr/bin/env bash
set -euo pipefail

echo "==> DRE-Engine development environment setup"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "WARNING: eBPF development requires Linux. Use WSL2 or a Linux VM for bpf builds."
fi

if command -v apt-get &>/dev/null; then
  sudo apt-get update
  sudo apt-get install -y \
    clang llvm libbpf-dev bpftool \
    linux-headers-"$(uname -r)" \
    make git curl
elif command -v dnf &>/dev/null; then
  sudo dnf install -y clang llvm libbpf-devel bpftool kernel-devel make git curl
fi

if ! command -v go &>/dev/null; then
  echo "Install Go 1.22+ from https://go.dev/dl/"
  exit 1
fi

if ! command -v kind &>/dev/null; then
  echo "==> Installing kind"
  curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.24.0/kind-linux-amd64
  chmod +x ./kind
  sudo mv ./kind /usr/local/bin/kind
fi

if ! command -v kubectl &>/dev/null; then
  echo "Install kubectl: https://kubernetes.io/docs/tasks/tools/"
  exit 1
fi

if [[ -f /sys/kernel/btf/vmlinux ]]; then
  echo "OK: BTF available at /sys/kernel/btf/vmlinux"
else
  echo "WARNING: BTF not found. CO-RE eBPF may fail. Enable CONFIG_DEBUG_INFO_BTF."
fi

go install github.com/cilium/ebpf/cmd/bpf2go@v0.16.0

echo "==> Setup complete. Run: make bpf && make build"
