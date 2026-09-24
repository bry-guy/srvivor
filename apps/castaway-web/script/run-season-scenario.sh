#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
app_dir="$(cd -- "$script_dir/.." && pwd)"
scenario="${1:-$app_dir/scenarios/two-episode.yaml}"

if [[ "$scenario" != /* ]]; then
  scenario="$PWD/$scenario"
fi
if [[ ! -f "$scenario" ]]; then
  echo "scenario file not found: $scenario" >&2
  exit 1
fi
if [[ "$#" -gt 1 ]]; then
  echo "usage: $0 [scenario.yaml]" >&2
  exit 2
fi

find_free_port() {
  python3 - <<'PY'
import socket
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
}

wait_for_postgres() {
  local container_id="$1"
  for _ in $(seq 1 120); do
    if docker exec "$container_id" pg_isready -h 127.0.0.1 -U postgres -d postgres >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.25
  done
  echo "timed out waiting for postgres in $container_id" >&2
  return 1
}

wait_for_http() {
  local url="$1"
  python3 - "$url" <<'PY'
import sys, time, urllib.request
url = sys.argv[1]
last_error = None
for _ in range(120):
    try:
        with urllib.request.urlopen(url, timeout=1) as response:
            if response.status == 200:
                sys.exit(0)
    except Exception as exc:  # noqa: BLE001
        last_error = exc
        time.sleep(0.25)
print(f"timed out waiting for {url}: {last_error}", file=sys.stderr)
sys.exit(1)
PY
}

postgres_port="$(find_free_port)"
api_port="$(find_free_port)"
container_name="castaway-web-season-scenario-${postgres_port}"
database_url="postgres://postgres:postgres@127.0.0.1:${postgres_port}/postgres?sslmode=disable"
service_token="${CASTAWAY_SCENARIO_SERVICE_TOKEN:-scenario-token}"
operator_id="${CASTAWAY_SCENARIO_OPERATOR_ID:-scenario-admin}"
server_log="$(mktemp -t castaway-season-scenario-server.XXXXXX.log)"
migration_log="$(mktemp -t castaway-season-scenario-migrate.XXXXXX.log)"
generated_file="$(mktemp -t castaway-season-scenario.XXXXXX.hurl)"
bin_dir="$(mktemp -d -t castaway-season-scenario-bin.XXXXXX)"
migrate_bin="$bin_dir/migrate"
server_bin="$bin_dir/server"
server_pid=""
container_id=""

cleanup() {
  local status=$?

  if [[ "$status" -ne 0 ]]; then
    if [[ -f "$migration_log" ]]; then
      printf '\nmigration log:\n' >&2
      sed -n '1,200p' "$migration_log" >&2 || true
    fi
    if [[ -f "$server_log" ]]; then
      printf '\nserver log:\n' >&2
      sed -n '1,200p' "$server_log" >&2 || true
    fi
  fi

  if [[ -n "$server_pid" ]] && kill -0 "$server_pid" >/dev/null 2>&1; then
    kill "$server_pid" >/dev/null 2>&1 || true
    wait "$server_pid" >/dev/null 2>&1 || true
  fi
  if [[ -n "$container_id" ]]; then
    docker rm -f "$container_id" >/dev/null 2>&1 || true
  fi
  rm -f "$migration_log" "$server_log" "$generated_file"
  rm -rf "$bin_dir"

  exit "$status"
}
trap cleanup EXIT

(
  cd "$app_dir"
  go run ./cmd/season-scenario -scenario "$scenario" -output "$generated_file"
  go build -o "$migrate_bin" ./cmd/migrate
  go build -o "$server_bin" ./cmd/server
)
hurlfmt --check "$generated_file"

if ! container_id="$(docker run -d --rm \
  --name "$container_name" \
  -e POSTGRES_PASSWORD=postgres \
  -p "127.0.0.1:${postgres_port}:5432" \
  postgres:16 2>"$migration_log")"; then
  echo "failed to start PostgreSQL container" >&2
  exit 1
fi

wait_for_postgres "$container_id"
(
  cd "$app_dir"
  env \
    DATABASE_URL="$database_url" \
    CASTAWAY_TEST_DATABASE_URL="$database_url" \
    AUTO_MIGRATE=false \
    MIGRATIONS_DIR="$app_dir/db/migrations" \
    "$migrate_bin"
) >>"$migration_log" 2>&1

(
  cd "$app_dir"
  exec env \
    DATABASE_URL="$database_url" \
    CASTAWAY_TEST_DATABASE_URL="$database_url" \
    AUTO_MIGRATE=false \
    MIGRATIONS_DIR="$app_dir/db/migrations" \
    SERVICE_AUTH_ENABLED=true \
    SERVICE_AUTH_BEARER_TOKENS="$service_token" \
    SERVICE_AUTH_PRINCIPAL=castaway-season-scenario \
    PORT="$api_port" \
    "$server_bin"
) >"$server_log" 2>&1 &
server_pid="$!"

wait_for_http "http://127.0.0.1:${api_port}/healthz"

hurl --test --jobs 1 --retry 0 \
  --variable "base_url=http://127.0.0.1:${api_port}" \
  --secret "service_token=${service_token}" \
  --variable "operator_discord_user_id=${operator_id}" \
  "$generated_file"
