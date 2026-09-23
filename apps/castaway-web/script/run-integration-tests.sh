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

wait_for_postgres() {
  local container_id="$1"
  for _ in $(seq 1 120); do
    if docker exec "$container_id" pg_isready -U postgres -d postgres >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.25
  done
  echo "timed out waiting for postgres in $container_id" >&2
  return 1
}

postgres_port="$(find_free_port)"
container_name="castaway-web-integration-${postgres_port}"
database_url="postgres://postgres:postgres@127.0.0.1:${postgres_port}/postgres?sslmode=disable"
container_id=""

cleanup() {
  local status=$?
  if [[ -n "$container_id" ]]; then
    docker rm -f "$container_id" >/dev/null 2>&1 || true
  fi
  exit "$status"
}
trap cleanup EXIT

container_id="$({ docker run -d --rm \
  --name "$container_name" \
  -e POSTGRES_PASSWORD=postgres \
  -p "127.0.0.1:${postgres_port}:5432" \
  postgres:16; })"

wait_for_postgres "$container_id"

(
  cd "$app_dir"
  DATABASE_URL="$database_url" \
  CASTAWAY_TEST_DATABASE_URL="$database_url" \
  go test -v "$@" ./internal/app ./internal/gameplay ./internal/httpapi
)
