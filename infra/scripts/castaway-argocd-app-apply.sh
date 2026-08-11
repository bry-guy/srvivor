#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./castaway-selfhost-lib.sh
source "$SCRIPT_DIR/castaway-selfhost-lib.sh"

require_commands kubectl git

repo="$(repo_root)"
export KUBECONFIG="$(castaway_kubeconfig_path)"
project_manifest="$repo/deploy/argocd/project-castaway.yaml"
app_manifest="$repo/deploy/argocd/app-home-k3s.yaml"

if [ ! -f "$KUBECONFIG" ]; then
  echo "kubeconfig not found: $KUBECONFIG" >&2
  echo "Provision/fetch the platform kubeconfig first, e.g. in ~/dev/infra: mise run selfhost:platform:kubeconfig:fetch" >&2
  exit 1
fi

if [ ! -f "$project_manifest" ]; then
  echo "required manifest not found: $project_manifest" >&2
  exit 1
fi

if [ ! -f "$app_manifest" ]; then
  echo "required manifest not found: $app_manifest" >&2
  exit 1
fi

printf 'Applying Castaway Argo CD project from %s...\n' "$project_manifest"
kubectl apply -f "$project_manifest"

printf 'Applying Castaway Argo CD application from %s...\n' "$app_manifest"
kubectl apply -f "$app_manifest"

printf 'Current Argo CD applications:\n'
kubectl -n argocd get applications.argoproj.io || true
