#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./selfhost-proxmox-remote-lib.sh
source "$SCRIPT_DIR/selfhost-proxmox-remote-lib.sh"

require_commands expect ssh scp kubectl tofu fnox
assert_required_env \
  PROXMOX_VE_USERNAME \
  PROXMOX_VE_PASSWORD \
  SELFHOST_PROXMOX_NODE_ADDRESS \
  SELFHOST_SERVICE_TEMPLATE_VM_ID \
  SELFHOST_STATEFUL_TEMPLATE_VM_ID \
  SELFHOST_APPLIANCE_TEMPLATE_VM_ID \
  SELFHOST_CASTAWAY_SRVIVOR_REPO_PATH \
  SELFHOST_CASTAWAY_POSTGRES_RESTIC_REPOSITORY

srvivor_repo_path="${SELFHOST_CASTAWAY_SRVIVOR_REPO_PATH}"

if [ ! -d "$srvivor_repo_path/.git" ]; then
  echo "srvivor repo not found at: $srvivor_repo_path" >&2
  exit 1
fi

for manifest_path in \
  "$srvivor_repo_path/deploy/argocd/project-castaway.yaml" \
  "$srvivor_repo_path/deploy/argocd/app-home-k3s.yaml"
do
  if [ ! -f "$manifest_path" ]; then
    echo "required Argo CD manifest not found: $manifest_path" >&2
    exit 1
  fi
done

if ! tofu -chdir=selfhost/regions/us-west validate >/dev/null; then
  echo "OpenTofu validation failed for selfhost/regions/us-west" >&2
  exit 1
fi

printf 'Checking required Proxmox templates on %s...\n' "$SELFHOST_PROXMOX_NODE_ADDRESS"
for template_vmid in \
  "$SELFHOST_SERVICE_TEMPLATE_VM_ID" \
  "$SELFHOST_STATEFUL_TEMPLATE_VM_ID" \
  "$SELFHOST_APPLIANCE_TEMPLATE_VM_ID"
do
  if ! proxmox_ssh "qm config $template_vmid >/dev/null 2>&1" >/dev/null; then
    echo "required template VM is missing or unreadable on Proxmox: $template_vmid" >&2
    exit 1
  fi
  printf '  - template VM %s found\n' "$template_vmid"
done

printf 'Checking required secret availability through fnox...\n'
fnox exec -- bash -lc '
  set -euo pipefail
  required_vars=(
    SELFHOST_CASTAWAY_VM_SSH_PRIVATE_KEY
    TAILSCALE_AUTHKEY
    CASTAWAY_POSTGRES_SUPERUSER_PASSWORD
    CASTAWAY_WEB_DB_PASSWORD
    CASTAWAY_DISCORD_BOT_DB_PASSWORD
    CASTAWAY_DISCORD_BOT_TOKEN
    CASTAWAY_DISCORD_APPLICATION_ID
    CASTAWAY_DISCORD_PUBLIC_KEY
    DISCORD_TARGET_SEVER_ID
    CASTAWAY_BOT_API_TOKEN
    SELFHOST_CASTAWAY_POSTGRES_RESTIC_PASSWORD
    OCI_OBJECTSTORAGE_ACCESS_KEY_ID
    OCI_OBJECTSTORAGE_SECRET_ACCESS_KEY
  )

  for var in "${required_vars[@]}"; do
    if [ -z "${!var:-}" ]; then
      echo "required fnox-provided variable is missing: $var" >&2
      exit 1
    fi
  done
'

printf 'Castaway preflight passed. Templates, manifests, tofu config, and secret inputs are present.\n'
