# shellcheck shell=bash
# shellcheck disable=SC2154 # app_dir is set by the sourcing script
# Sourced by the integration, regression and scenario scripts. Callers set
# app_dir first.
#
# DATABASE_URL set: use that server (laptop score-compose Postgres or workspace
# sidecar) and never start containers. DATABASE_URL unset: callers start a
# disposable Docker Postgres (CI).

use_shared_database() {
  [[ -n "${DATABASE_URL:-}" ]]
}

require_shared_database() {
  if ! python3 -c '
import socket, sys
from urllib.parse import urlparse
u = urlparse(sys.argv[1])
socket.create_connection((u.hostname, u.port or 5432), timeout=2).close()
' "$DATABASE_URL" 2>/dev/null; then
    echo "DATABASE_URL is set but unreachable. Start the dev database: mise run //apps/castaway-web:db:up" >&2
    exit 1
  fi
}

# Prints the URL of a new database named $1 on the DATABASE_URL server.
create_test_database() {
  (cd "$app_dir" && go run ./cmd/testdb create "$1")
}

drop_test_database() {
  (cd "$app_dir" && go run ./cmd/testdb drop "$1") >/dev/null 2>&1 || true
}

new_test_database_name() {
  python3 -c 'import uuid; print("castaway_test_" + uuid.uuid4().hex)'
}
