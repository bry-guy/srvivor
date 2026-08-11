#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./castaway-selfhost-lib.sh
source "$SCRIPT_DIR/castaway-selfhost-lib.sh"

require_commands kubectl
assert_required_env \
  CASTAWAY_POSTGRES_SUPERUSER_PASSWORD \
  CASTAWAY_WEB_DB_PASSWORD \
  CASTAWAY_DISCORD_BOT_DB_PASSWORD \
  CASTAWAY_DISCORD_BOT_TOKEN \
  CASTAWAY_DISCORD_APPLICATION_ID \
  CASTAWAY_DISCORD_PUBLIC_KEY \
  DISCORD_TARGET_SEVER_ID \
  CASTAWAY_BOT_API_TOKEN

namespace="$(castaway_namespace)"
export KUBECONFIG="$(castaway_kubeconfig_path)"
postgres_host="${CASTAWAY_SELFHOST_POSTGRES_HOST:-${SELFHOST_CASTAWAY_POSTGRES_HOST:-${SELFHOST_PLATFORM_POSTGRES_HOST:-us-west-stateful-400}}}"
postgres_port="${CASTAWAY_SELFHOST_POSTGRES_PORT:-${SELFHOST_CASTAWAY_POSTGRES_PORT:-${SELFHOST_PLATFORM_POSTGRES_PORT:-5432}}}"
postgres_superuser="${CASTAWAY_SELFHOST_POSTGRES_SUPERUSER:-${SELFHOST_CASTAWAY_POSTGRES_SUPERUSER:-${SELFHOST_PLATFORM_POSTGRES_SUPERUSER:-postgres}}}"
web_db_user="${CASTAWAY_SELFHOST_WEB_DB_USER:-${SELFHOST_CASTAWAY_WEB_DB_USER:-castaway_web}}"
bot_db_user="${CASTAWAY_SELFHOST_DISCORD_BOT_DB_USER:-${SELFHOST_CASTAWAY_DISCORD_BOT_DB_USER:-castaway_discord_bot}}"
web_db_name="${CASTAWAY_SELFHOST_WEB_DB_NAME:-${SELFHOST_CASTAWAY_POSTGRES_WEB_DB_NAME:-castaway_web}}"
bot_db_name="${CASTAWAY_SELFHOST_DISCORD_BOT_DB_NAME:-${SELFHOST_CASTAWAY_POSTGRES_BOT_DB_NAME:-castaway_discord_bot}}"

if [ ! -f "$KUBECONFIG" ]; then
  echo "kubeconfig not found: $KUBECONFIG" >&2
  echo "Provision/fetch the platform kubeconfig first, e.g. in ~/dev/infra: mise run selfhost:platform:kubeconfig:fetch" >&2
  exit 1
fi

web_database_url="postgres://${web_db_user}:${CASTAWAY_WEB_DB_PASSWORD}@${postgres_host}:${postgres_port}/${web_db_name}?sslmode=disable"
bot_database_url="postgres://${bot_db_user}:${CASTAWAY_DISCORD_BOT_DB_PASSWORD}@${postgres_host}:${postgres_port}/${bot_db_name}?sslmode=disable"

kubectl create namespace "$namespace" --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic castaway-postgres-secrets \
  -n "$namespace" \
  --from-literal=postgres-superuser="$postgres_superuser" \
  --from-literal=postgres-superuser-password="$CASTAWAY_POSTGRES_SUPERUSER_PASSWORD" \
  --from-literal=castaway-web-db-user="$web_db_user" \
  --from-literal=castaway-web-db-password="$CASTAWAY_WEB_DB_PASSWORD" \
  --from-literal=castaway-discord-bot-db-user="$bot_db_user" \
  --from-literal=castaway-discord-bot-db-password="$CASTAWAY_DISCORD_BOT_DB_PASSWORD" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic castaway-web-secrets \
  -n "$namespace" \
  --from-literal=DATABASE_URL="$web_database_url" \
  --from-literal=SERVICE_AUTH_BEARER_TOKENS="$CASTAWAY_BOT_API_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic castaway-discord-bot-secrets \
  -n "$namespace" \
  --from-literal=CASTAWAY_DISCORD_BOT_TOKEN="$CASTAWAY_DISCORD_BOT_TOKEN" \
  --from-literal=CASTAWAY_DISCORD_APPLICATION_ID="$CASTAWAY_DISCORD_APPLICATION_ID" \
  --from-literal=CASTAWAY_DISCORD_PUBLIC_KEY="$CASTAWAY_DISCORD_PUBLIC_KEY" \
  --from-literal=DISCORD_TARGET_SEVER_ID="$DISCORD_TARGET_SEVER_ID" \
  --from-literal=CASTAWAY_API_AUTH_TOKEN="$CASTAWAY_BOT_API_TOKEN" \
  --from-literal=BOT_STATE_DATABASE_URL="$bot_database_url" \
  --dry-run=client -o yaml | kubectl apply -f -

printf 'Synchronized Castaway Kubernetes secrets into namespace %s using kubeconfig %s\n' "$namespace" "$KUBECONFIG"
