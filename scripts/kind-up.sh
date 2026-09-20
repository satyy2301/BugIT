#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${KIND_CLUSTER_NAME:-dre-engine}"
NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.29.4}"

if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  cat <<EOF | kind create cluster --name "${CLUSTER_NAME}" --image "${NODE_IMAGE}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraMounts:
      - hostPath: /sys/kernel/debug
        containerPath: /sys/kernel/debug
EOF
fi

kubectl cluster-info --context "kind-${CLUSTER_NAME}"
echo "Kind cluster '${CLUSTER_NAME}' is ready."
