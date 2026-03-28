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
  local container_name="$1"
  for _ in $(seq 1 120); do
    if docker exec "$container_name" pg_isready -U postgres -d postgres >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.25
  done
  echo "timed out waiting for postgres in $container_name" >&2
  return 1
}

postgres_port="$(find_free_port)"
container_name="castaway-web-integration-${postgres_port}"
database_url="postgres://postgres:postgres@127.0.0.1:${postgres_port}/postgres?sslmode=disable"

cleanup() {
  local status=$?
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  exit "$status"
}
trap cleanup EXIT

docker run -d --rm \
  --name "$container_name" \
  -e POSTGRES_PASSWORD=postgres \
  -p "127.0.0.1:${postgres_port}:5432" \
  postgres:16 >/dev/null

wait_for_postgres "$container_name"

(
  cd "$app_dir"
  CASTAWAY_TEST_DATABASE_URL="$database_url" go test -v ./internal/app ./internal/gameplay ./internal/httpapi
)
