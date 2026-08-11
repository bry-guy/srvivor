#!/usr/bin/env bash
# Shared helpers for Castaway app-owned selfhost deployment scripts.

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

repo_root() {
  git rev-parse --show-toplevel 2>/dev/null || pwd
}

castaway_kubeconfig_path() {
  printf '%s\n' "${CASTAWAY_SELFHOST_KUBECONFIG_PATH:-${SELFHOST_CASTAWAY_KUBECONFIG_PATH:-${SELFHOST_PLATFORM_MIGRATION_KUBECONFIG_PATH:-${SELFHOST_PLATFORM_KUBECONFIG_PATH:-$HOME/.kube/selfhost-platform-7049-1.yaml}}}}"
}

castaway_namespace() {
  printf '%s\n' "${CASTAWAY_SELFHOST_NAMESPACE:-${SELFHOST_CASTAWAY_NAMESPACE:-castaway}}"
}

castaway_infra_repo_path() {
  printf '%s\n' "${CASTAWAY_PLATFORM_INFRA_REPO_PATH:-${SELFHOST_INFRA_REPO_PATH:-$HOME/dev/infra}}"
}
