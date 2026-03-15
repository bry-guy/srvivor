#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
app_dir="$(cd -- "$script_dir/.." && pwd)"

find_free_port() {
  python3 - <<'PY'
import socket
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
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

wait_for_postgres() {
  local container_name="$1"
  for _ in $(seq 1 120); do
    if docker exec "$container_name" pg_isready -U castaway -d castaway >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.25
  done
  echo "timed out waiting for postgres in $container_name" >&2
  return 1
}

postgres_port="$(find_free_port)"
api_port="$(find_free_port)"
container_name="castaway-web-hurl-${postgres_port}"
database_url="postgres://castaway:castaway@127.0.0.1:${postgres_port}/castaway?sslmode=disable"
server_log="$(mktemp -t castaway-web-hurl-server.XXXXXX.log)"
seed_log="$(mktemp -t castaway-web-hurl-seed.XXXXXX.log)"
server_pid=""

cleanup() {
  local status=$?

  if [[ $status -ne 0 ]]; then
    if [[ -f "$seed_log" ]]; then
      echo "\nseed log:" >&2
      sed -n '1,200p' "$seed_log" >&2 || true
    fi
    if [[ -f "$server_log" ]]; then
      echo "\nserver log:" >&2
      sed -n '1,200p' "$server_log" >&2 || true
    fi
  fi

  if [[ -n "$server_pid" ]] && kill -0 "$server_pid" >/dev/null 2>&1; then
    kill "$server_pid" >/dev/null 2>&1 || true
    wait "$server_pid" >/dev/null 2>&1 || true
  fi
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  rm -f "$seed_log" "$server_log"

  exit "$status"
}
trap cleanup EXIT

docker run -d --rm \
  --name "$container_name" \
  -e POSTGRES_DB=castaway \
  -e POSTGRES_USER=castaway \
  -e POSTGRES_PASSWORD=castaway \
  -p "127.0.0.1:${postgres_port}:5432" \
  postgres:16 >/dev/null

wait_for_postgres "$container_name"

(
  cd "$app_dir"
  DATABASE_URL="$database_url" AUTO_MIGRATE=false go run ./cmd/seed
) >"$seed_log" 2>&1

(
  cd "$app_dir"
  DATABASE_URL="$database_url" PORT="$api_port" AUTO_MIGRATE=false go run ./cmd/server
) >"$server_log" 2>&1 &
server_pid="$!"

wait_for_http "http://127.0.0.1:${api_port}/healthz"

(
  cd "$app_dir"
  hurl --test --variable base_url="http://127.0.0.1:${api_port}" hurl/*.hurl
)
