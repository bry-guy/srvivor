#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./castaway-selfhost-lib.sh
source "$SCRIPT_DIR/castaway-selfhost-lib.sh"

require_commands kubectl

namespace="$(castaway_namespace)"
export KUBECONFIG="$(castaway_kubeconfig_path)"

if [ ! -f "$KUBECONFIG" ]; then
  echo "kubeconfig not found: $KUBECONFIG" >&2
  exit 1
fi

printf 'Verifying Castaway app resources in namespace %s...\n' "$namespace"
kubectl get namespace "$namespace"
kubectl -n "$namespace" get secret castaway-web-secrets castaway-discord-bot-secrets castaway-postgres-secrets
kubectl -n "$namespace" get deploy,svc,pod || true
kubectl -n argocd get applications.argoproj.io castaway-home-k3s || true
