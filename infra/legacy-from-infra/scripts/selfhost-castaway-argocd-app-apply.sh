#!/usr/bin/env bash
set -euo pipefail

require_commands() {
  local cmd
  for cmd in "$@"; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      echo "required command not found: $cmd" >&2
      exit 1
    fi
  done
}

assert_required_env() {
  local var
  for var in "$@"; do
    if [ -z "${!var:-}" ]; then
      echo "required environment variable is missing: $var" >&2
      exit 1
    fi
  done
}

require_commands kubectl
assert_required_env \
  SELFHOST_CASTAWAY_KUBECONFIG_PATH \
  SELFHOST_CASTAWAY_SRVIVOR_REPO_PATH

export KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH"
srvivor_repo_path="$SELFHOST_CASTAWAY_SRVIVOR_REPO_PATH"
project_manifest="$srvivor_repo_path/deploy/argocd/project-castaway.yaml"
app_manifest="$srvivor_repo_path/deploy/argocd/app-home-k3s.yaml"

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
