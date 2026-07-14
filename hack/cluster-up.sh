#!/usr/bin/env bash
# Create a local k3d (k3s-in-Docker) cluster + registry for Atlas.
set -euo pipefail

CLUSTER_NAME="${ATLAS_CLUSTER_NAME:-atlas}"
API_PORT="${ATLAS_K3D_API_PORT:-6445}"
REGISTRY_NAME="${ATLAS_REGISTRY_NAME:-atlas-registry}"
REGISTRY_HOST_PORT="${ATLAS_REGISTRY_PORT:-5001}"
NETWORK_NAME="k3d-${CLUSTER_NAME}"
KUBECONFIG_HOST="${HOME}/.kube/config"
KUBECONFIG_DOCKER="${HOME}/.kube/atlas-docker.yaml"

port_in_use() {
	local port="$1"
	if command -v lsof >/dev/null 2>&1; then
		lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1
	else
		return 1
	fi
}

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required. Install Docker Desktop (or Engine) first." >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "Docker daemon is not running." >&2
  exit 1
fi

if ! command -v k3d >/dev/null 2>&1; then
  echo "k3d is required to create the local cluster." >&2
  echo "  macOS:  brew install k3d" >&2
  echo "  other:  https://k3d.io/#installation" >&2
  exit 1
fi

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required." >&2
  echo "  macOS:  brew install kubectl" >&2
  exit 1
fi

mkdir -p "${HOME}/.kube"
if [ -d "${KUBECONFIG_HOST}" ]; then
  echo "${KUBECONFIG_HOST} is a directory (Docker bind-mount footgun)." >&2
  echo "Remove it, then re-run:  rmdir ${KUBECONFIG_HOST}" >&2
  exit 1
fi

cluster_exists() {
	k3d cluster list 2>/dev/null | awk 'NR > 1 { print $1 }' | grep -qx "$CLUSTER_NAME"
}

if cluster_exists; then
  echo "Cluster '${CLUSTER_NAME}' already exists — refreshing kubeconfig..."
else
  if port_in_use "${REGISTRY_HOST_PORT}"; then
    echo "Host port ${REGISTRY_HOST_PORT} is already in use (on macOS, 5000 is often AirPlay Receiver)." >&2
    echo "Pick a free port, e.g.:  ATLAS_REGISTRY_PORT=5001 make cluster" >&2
    exit 1
  fi
  echo "Creating k3d cluster '${CLUSTER_NAME}' (API :${API_PORT}, registry :${REGISTRY_HOST_PORT})..."
  k3d cluster create "${CLUSTER_NAME}" \
    --api-port "0.0.0.0:${API_PORT}" \
    --registry-create "${REGISTRY_NAME}:0.0.0.0:${REGISTRY_HOST_PORT}" \
    --port "80:80@loadbalancer" \
    --port "443:443@loadbalancer" \
    --k3s-arg "--tls-san=host.docker.internal@server:0" \
    --k3s-arg "--tls-san=127.0.0.1@server:0" \
    --k3s-arg "--tls-san=localhost@server:0" \
    --kubeconfig-update-default=false \
    --wait
fi

RAW="$(mktemp)"
HOST_CFG="$(mktemp)"
DOCKER_CFG="$(mktemp)"
trap 'rm -f "$RAW" "$HOST_CFG" "$DOCKER_CFG"' EXIT

k3d kubeconfig get "${CLUSTER_NAME}" >"$RAW"

# Host kubectl / make run: loopback works on the Mac.
perl -pe "s#https://(?:127\\.0\\.0\\.1|0\\.0\\.0\\.0|localhost):${API_PORT}#https://127.0.0.1:${API_PORT}#g" \
  "$RAW" >"$HOST_CFG"

# Atlas Compose containers: host.docker.internal (added via extra_hosts).
# Do not use this from the Mac host — many systems do not resolve that name.
perl -pe "s#https://(?:127\\.0\\.0\\.1|0\\.0\\.0\\.0|localhost):${API_PORT}#https://host.docker.internal:${API_PORT}#g" \
  "$RAW" >"$DOCKER_CFG"

cp "$HOST_CFG" "${KUBECONFIG_HOST}"
cp "$DOCKER_CFG" "${KUBECONFIG_DOCKER}"
chmod 600 "${KUBECONFIG_HOST}" "${KUBECONFIG_DOCKER}"

export KUBECONFIG="${KUBECONFIG_HOST}"
echo "Waiting for node Ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=180s

echo
echo "Cluster ready."
echo "  kubeconfig (host):   ${KUBECONFIG_HOST}  → https://127.0.0.1:${API_PORT}"
echo "  kubeconfig (docker): ${KUBECONFIG_DOCKER} → https://host.docker.internal:${API_PORT}"
echo "  registry host:       localhost:${REGISTRY_HOST_PORT}"
echo "  registry (k8s):      ${REGISTRY_NAME}:5000"
echo "  docker network:      ${NETWORK_NAME}"
echo
echo "Use ATLAS_REGISTRY_URL=${REGISTRY_NAME}:5000 (Compose default) so Kaniko pushes/pulls in-cluster."
echo "Next:  make docker-up"
echo "Apps via Ingress: add '*.homelab.local' → 127.0.0.1 in /etc/hosts (Traefik on :80)."
