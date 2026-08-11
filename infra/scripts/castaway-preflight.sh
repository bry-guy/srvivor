#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./castaway-selfhost-lib.sh
source "$SCRIPT_DIR/castaway-selfhost-lib.sh"

require_commands kubectl git

repo="$(repo_root)"
infra_repo="$(castaway_infra_repo_path)"
kubeconfig="$(castaway_kubeconfig_path)"

printf 'Castaway selfhost app preflight\n'
printf '  app repo:      %s\n' "$repo"
printf '  platform repo: %s\n' "$infra_repo"
printf '  kubeconfig:    %s\n' "$kubeconfig"

missing=0
for path in \
  "$repo/deploy/argocd/project-castaway.yaml" \
  "$repo/deploy/argocd/app-home-k3s.yaml" \
  "$repo/deploy/environments/home-k3s/kustomization.yaml"; do
  if [ ! -f "$path" ]; then
    echo "missing required app manifest: $path" >&2
    missing=1
  fi
done

if [ ! -d "$infra_repo" ]; then
  echo "platform infra repo not found: $infra_repo" >&2
  missing=1
elif [ ! -f "$infra_repo/mise.toml" ]; then
  echo "platform infra repo does not look like ~/dev/infra: $infra_repo" >&2
  missing=1
fi

if [ ! -f "$kubeconfig" ]; then
  echo "kubeconfig not found: $kubeconfig" >&2
  echo "Run in platform infra first: mise run selfhost:platform:kubeconfig:fetch" >&2
  missing=1
fi

if [ "$missing" -ne 0 ]; then
  exit 1
fi

KUBECONFIG="$kubeconfig" kubectl cluster-info >/dev/null
KUBECONFIG="$kubeconfig" kubectl get namespace argocd >/dev/null

printf 'Preflight OK.\n'
