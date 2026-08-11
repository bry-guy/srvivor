#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./selfhost-ssh-lib.sh
source "$SCRIPT_DIR/selfhost-ssh-lib.sh"

require_commands kubectl ssh scp mktemp
assert_required_env \
  SELFHOST_CASTAWAY_KUBECONFIG_PATH \
  SELFHOST_CASTAWAY_APPLIANCE_NODE_NAME \
  SELFHOST_CASTAWAY_SERVICE_NODE_NAME \
  SELFHOST_CASTAWAY_STATEFUL_VM_HOST \
  SELFHOST_CASTAWAY_STATEFUL_VM_SSH_USER \
  SELFHOST_CASTAWAY_VM_SSH_PRIVATE_KEY

export KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH"
export SELFHOST_REMOTE_HOST="$SELFHOST_CASTAWAY_STATEFUL_VM_HOST"
export SELFHOST_REMOTE_USER="$SELFHOST_CASTAWAY_STATEFUL_VM_SSH_USER"
export SELFHOST_REMOTE_SSH_PRIVATE_KEY="$SELFHOST_CASTAWAY_VM_SSH_PRIVATE_KEY"

ssh_key_file="$(selfhost_write_ssh_key)"
trap 'rm -f "$ssh_key_file"' EXIT

printf 'Checking cluster nodes...\n'
kubectl get nodes -o wide
kubectl get nodes -L selfhost.bry-guy.net/role

printf 'Checking core namespaces and pods...\n'
kubectl get ns argocd castaway
kubectl -n argocd get pods
kubectl -n castaway get secrets

printf 'Checking Argo CD application objects...\n'
kubectl -n argocd get applications.argoproj.io || true

printf 'Checking stateful VM PostgreSQL service...\n'
selfhost_ssh 'sudo systemctl --no-pager --full status castaway-postgres.service' "$ssh_key_file"

printf 'Checking stateful VM PostgreSQL backup timer...\n'
selfhost_ssh 'sudo systemctl --no-pager --full status castaway-postgres-backup.timer && sudo systemctl --no-pager --full status castaway-postgres-backup.service || true' "$ssh_key_file"

printf 'Castaway verification checks completed.\n'
