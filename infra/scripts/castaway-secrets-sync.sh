#!/usr/bin/env bash
set -euo pipefail
# App Secrets only. Database credentials (castaway-web-db, castaway-discord-bot-db) are owned by bry-guy/infra:
# mise run selfhost:platform:castaway:secrets:apply

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./castaway-selfhost-lib.sh
source "$SCRIPT_DIR/castaway-selfhost-lib.sh"

require_commands kubectl
assert_required_env \
  CASTAWAY_DISCORD_BOT_TOKEN \
  CASTAWAY_DISCORD_APPLICATION_ID \
  CASTAWAY_DISCORD_PUBLIC_KEY \
  DISCORD_TARGET_SEVER_ID \
  CASTAWAY_BOT_API_TOKEN

namespace="$(castaway_namespace)"
export KUBECONFIG="$(castaway_kubeconfig_path)"

if [ ! -f "$KUBECONFIG" ]; then
  echo "kubeconfig not found: $KUBECONFIG" >&2
  echo "Provision/fetch the platform kubeconfig first, e.g. in ~/dev/infra: mise run selfhost:platform:kubeconfig:fetch" >&2
  exit 1
fi

kubectl create namespace "$namespace" --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic castaway-web-secrets \
  -n "$namespace" \
  --from-literal=SERVICE_AUTH_BEARER_TOKENS="$CASTAWAY_BOT_API_TOKEN" \
  ${CASTAWAY_DISCORD_OAUTH_CLIENT_SECRET:+--from-literal=DISCORD_OAUTH_CLIENT_SECRET="$CASTAWAY_DISCORD_OAUTH_CLIENT_SECRET"} \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic castaway-discord-bot-secrets \
  -n "$namespace" \
  --from-literal=CASTAWAY_DISCORD_BOT_TOKEN="$CASTAWAY_DISCORD_BOT_TOKEN" \
  --from-literal=CASTAWAY_DISCORD_APPLICATION_ID="$CASTAWAY_DISCORD_APPLICATION_ID" \
  --from-literal=CASTAWAY_DISCORD_PUBLIC_KEY="$CASTAWAY_DISCORD_PUBLIC_KEY" \
  --from-literal=DISCORD_TARGET_SEVER_ID="$DISCORD_TARGET_SEVER_ID" \
  --from-literal=CASTAWAY_API_AUTH_TOKEN="$CASTAWAY_BOT_API_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -

if [ -n "${CASTAWAY_CLOUDFLARED_TOKEN:-}" ]; then
  kubectl create secret generic castaway-cloudflared \
    -n "$namespace" \
    --from-literal=TUNNEL_TOKEN="$CASTAWAY_CLOUDFLARED_TOKEN" \
    --dry-run=client -o yaml | kubectl apply -f -
fi

printf 'Synchronized Castaway Kubernetes secrets into namespace %s using kubeconfig %s\n' "$namespace" "$KUBECONFIG"
