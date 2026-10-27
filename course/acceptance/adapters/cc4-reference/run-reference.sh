#!/usr/bin/env bash
# Run the acceptance scenarios against the instructor's CC4 system through the
# reference adapter, on a disposable PostgreSQL. Nothing outside this run is touched.
# Usage: bash course/acceptance/adapters/cc4-reference/run-reference.sh [accept.py args...]
set -Eeuo pipefail
here=$(cd "$(dirname "$0")" && pwd)
acceptance=$(cd "$here/../.." && pwd)
app=$(cd "$acceptance/../../app" && pwd)
run_id=$(python3 -c 'import secrets;print(secrets.token_hex(4))')
container="cc4-acceptance-$run_id"
tmp=$(mktemp -d)
if [[ -n ${CC4_REFERENCE_REV:-} ]]; then
  # Build a committed revision instead of the working tree (e.g. while app/ has unfinished edits).
  repo=$(git -C "$app" rev-parse --show-toplevel)
  git -C "$repo" archive "$CC4_REFERENCE_REV" app | tar -x -C "$tmp"
  app="$tmp/app"
  echo "[参考] 使用已提交版本 $(git -C "$repo" rev-parse --short "$CC4_REFERENCE_REV") 构建" >&2
fi
server_pid=''
free_port() { python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()'; }
cleanup() {
  result=$?
  trap - EXIT
  [[ -z $server_pid ]] || { kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true; }
  docker rm -f -v "$container" >/dev/null 2>&1 || result=1
  [[ ! -f $tmp/server.log ]] || cp "$tmp/server.log" "${CC4_ACCEPTANCE_SERVER_LOG:-$acceptance/results/cc4-reference-server-$run_id.log}" 2>/dev/null || true
  rm -rf -- "$tmp"
  exit "$result"
}
trap cleanup EXIT
mkdir -p "$acceptance/results"

db_password=$(python3 -c 'import secrets;print(secrets.token_hex(16))')
docker run -d --name "$container" --label cc4.acceptance="$run_id" -e POSTGRES_USER=root -e POSTGRES_PASSWORD="$db_password" \
  -e POSTGRES_DB=cc4_acceptance -p 127.0.0.1::5432 "${CC4_POSTGRES_IMAGE:-postgres:14}" >/dev/null
for _ in {1..60}; do docker exec "$container" pg_isready -h 127.0.0.1 -U root -d cc4_acceptance >/dev/null 2>&1 && break; sleep 1; done
db_port=$(docker port "$container" 5432/tcp | head -1 | awk -F: '{print $NF}')
for migration in "$app"/migrations/*.sql; do
  docker exec -i "$container" psql -q -v ON_ERROR_STOP=1 -U root -d cc4_acceptance < "$migration" >/dev/null
done
docker exec -i "$container" psql -q -v ON_ERROR_STOP=1 -U root -d cc4_acceptance <<'SQL' >/dev/null
INSERT INTO users(phone,password_hash,role,status) VALUES('00088888888','acceptance-admin-password','admin','active');
INSERT INTO employees(user_id,employee_code,name,level,status,join_date) SELECT id,'ACC_ADMIN','验收管理员','a_level','active','2025-01-01' FROM users WHERE phone='00088888888';
SQL

(cd "$app" && go build -o "$tmp/cc4-server" ./cmd/server)
server_port=$(free_port)
adapter_port=$(free_port)
DATABASE_URL="postgres://root:${db_password}@127.0.0.1:${db_port}/cc4_acceptance?sslmode=disable" DB_NAME=cc4_acceptance \
  HOST=127.0.0.1 PORT="$server_port" ENABLE_CRON=false BUSINESS_TIMEZONE=Asia/Shanghai JWT_SECRET="acceptance-$run_id" \
  UPLOAD_DIR="$tmp/uploads" "$tmp/cc4-server" > "$tmp/server.log" 2>&1 &
server_pid=$!
for _ in {1..100}; do curl -fsS "http://127.0.0.1:$server_port/health" >/dev/null 2>&1 && break; sleep 0.2; done

export CC4_BASE_URL="http://127.0.0.1:$server_port/api/v1" CC4_ADMIN_PHONE=00088888888 CC4_ADMIN_PASSWORD=acceptance-admin-password \
  CC4_ADAPTER_SECRET="acceptance-$run_id"
python3 "$acceptance/accept.py" \
  --adapter "http://127.0.0.1:$adapter_port/acceptance/act" \
  --adapter-cmd "python3 $here/adapter.py --port $adapter_port" \
  --manifest "$here/adapter.json" --adapter-src "$here/adapter.py" "$@"
