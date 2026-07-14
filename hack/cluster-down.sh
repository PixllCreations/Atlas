#!/usr/bin/env bash
# Delete the local k3d cluster created for Atlas.
set -euo pipefail

CLUSTER_NAME="${ATLAS_CLUSTER_NAME:-atlas}"

if ! command -v k3d >/dev/null 2>&1; then
  echo "k3d not found; nothing to delete." >&2
  exit 0
fi

if k3d cluster list 2>/dev/null | awk 'NR > 1 { print $1 }' | grep -qx "$CLUSTER_NAME"; then
  echo "Deleting k3d cluster '${CLUSTER_NAME}'..."
  k3d cluster delete "${CLUSTER_NAME}"
  echo "Cluster deleted (attached registry removed with it)."
else
  echo "Cluster '${CLUSTER_NAME}' not found."
fi
