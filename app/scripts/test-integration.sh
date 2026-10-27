#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

CC4_TEST_CONTAINER="${CC4_TEST_CONTAINER:-cc-postgres}"
CC4_TEST_USER="${DB_USER:-root}"
CC4_TEST_DB="cc4_test_go_$(date +%s)_$$"
docker exec "$CC4_TEST_CONTAINER" createdb -U "$CC4_TEST_USER" "$CC4_TEST_DB"
cleanup() {
  if [ "${CC4_KEEP_TEST_DB:-0}" = 1 ]; then
    printf '%s\n' "$CC4_TEST_DB" > "${CC4_TEST_DB_FILE:-/tmp/cc4-preview-db-name}"
    printf 'Preview database retained: %s\n' "$CC4_TEST_DB"
  else
    docker exec "$CC4_TEST_CONTAINER" dropdb -U "$CC4_TEST_USER" --if-exists "$CC4_TEST_DB" >/dev/null
  fi
}
trap cleanup EXIT
docker exec -i "$CC4_TEST_CONTAINER" psql -q -v ON_ERROR_STOP=1 -U "$CC4_TEST_USER" -d "$CC4_TEST_DB" < migrations/000_baseline_schema.sql >/dev/null
docker exec -i "$CC4_TEST_CONTAINER" psql -q -v ON_ERROR_STOP=1 -U "$CC4_TEST_USER" -d "$CC4_TEST_DB" < migrations/001_go_completion.sql >/dev/null

export PGPASSWORD="${DB_PASSWORD:-$(docker exec "$CC4_TEST_CONTAINER" printenv POSTGRES_PASSWORD)}"
export CC4_TEST_DATABASE_URL="postgres://${CC4_TEST_USER}@${DB_HOST:-localhost}:${DB_PORT:-5433}/${CC4_TEST_DB}?sslmode=disable"
if [[ -n ${CC4_TEST_JSON:-} ]]; then
  go test -race ./internal/api -run 'Test(BusinessIntegration|MCPBusinessIntegration)$' -count=1 -json | tee "$CC4_TEST_JSON"
else
  go test -race ./internal/api -run 'Test(BusinessIntegration|MCPBusinessIntegration)$' -count=1 -v
fi
