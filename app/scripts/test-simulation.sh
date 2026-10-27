#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/.."
container=${CC4_TEST_CONTAINER:-cc-postgres}
db_user=${DB_USER:-root}
simulation_db="cc4_test_simulation_$(date +%s)_$$"
server_pid=''
mock_pid=''
report_dir=${CC4_VERIFICATION_DIR:-}
if [[ -n $report_dir ]]; then
  [[ ! -e $report_dir ]] || { echo 'Verification evidence directory already exists.' >&2; exit 1; }
  mkdir -p "$report_dir"
fi
export_report() {
  [[ -z $report_dir ]] || docker exec "$container" psql -U "$db_user" -d "$simulation_db" -Atc "SELECT value FROM system_settings WHERE key='business_simulation_report_v1'" > "$report_dir/$1-report.json"
}
counts_sql="SELECT json_build_object('employees',(SELECT count(*) FROM employees),'users',(SELECT count(*) FROM users),'daily',(SELECT count(*) FROM daily_performance_records),'orders',(SELECT count(*) FROM daily_performance_order_details),'payments',(SELECT count(*) FROM payment_records),'finance',(SELECT count(*) FROM finance_records),'customers',(SELECT count(*) FROM customers))"
simulation_tmp=$(mktemp -d)
cleanup() {
  result=$?
  trap - EXIT
  [[ -z $server_pid ]] || kill "$server_pid" 2>/dev/null || true
  [[ -z $mock_pid ]] || kill "$mock_pid" 2>/dev/null || true
  [[ -z $server_pid ]] || wait "$server_pid" 2>/dev/null || true
  [[ -z $mock_pid ]] || wait "$mock_pid" 2>/dev/null || true
  if [[ -n $report_dir ]]; then
    for log in server mock; do
      if [[ -f $simulation_tmp/$log.log ]]; then cp "$simulation_tmp/$log.log" "$report_dir/$log.log" || result=1; fi
    done
  fi
  docker exec "$container" dropdb -U "$db_user" --if-exists "$simulation_db" >/dev/null || result=1
  rm -rf -- "$simulation_tmp" || result=1
  exit "$result"
}
trap cleanup EXIT
docker exec "$container" createdb -U "$db_user" "$simulation_db"
for migration in migrations/*.sql; do
  docker exec -i "$container" psql -q -v ON_ERROR_STOP=1 -U "$db_user" -d "$simulation_db" < "$migration" >/dev/null
done
docker exec -i "$container" psql -q -v ON_ERROR_STOP=1 -U "$db_user" -d "$simulation_db" <<'SQL' >/dev/null
INSERT INTO users(phone,password_hash,role,status) VALUES('00099999999','ci-simulation-password','admin','active');
INSERT INTO employees(user_id,employee_code,name,level,status,join_date) SELECT id,'SIM_CI_ADMIN','模拟验收管理员','a_level','active','2026-01-01' FROM users WHERE phone='00099999999';
INSERT INTO performance_records(employee_id,period_year,period_month,orders_count,sales_amount,commission_amount)
SELECT id,EXTRACT(YEAR FROM CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai'),EXTRACT(MONTH FROM CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai'),42,1234,77 FROM employees WHERE employee_code='SIM_CI_ADMIN';
SQL
sentinel_before=$(docker exec "$container" psql -U "$db_user" -d "$simulation_db" -Atc "SELECT row_to_json(p)::text FROM performance_records p JOIN employees e ON e.id=p.employee_id WHERE e.employee_code='SIM_CI_ADMIN'")
go build -o "$simulation_tmp/cc4-server" ./cmd/server
(cd ../services/lead-source-mock && go build -o "$simulation_tmp/mock" ./cmd/server)
server_port=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')
mock_port=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')
HOST=127.0.0.1 PORT="$mock_port" MOCK_USERNAME=ci-demo MOCK_PASSWORD=ci-simulation-upstream-password MOCK_TOKEN_SECRET=ci-simulation-upstream-token-secret-32 "$simulation_tmp/mock" > "$simulation_tmp/mock.log" 2>&1 &
mock_pid=$!
export PGPASSWORD="${DB_PASSWORD:-$(docker exec "$container" printenv POSTGRES_PASSWORD)}"
DATABASE_URL="postgres://${db_user}@${DB_HOST:-127.0.0.1}:${DB_PORT:-5433}/${simulation_db}?sslmode=disable" DB_NAME="$simulation_db" HOST=127.0.0.1 PORT="$server_port" ENABLE_CRON=false BUSINESS_SIMULATION_ENABLED=true JWT_SECRET=ci-simulation-jwt-secret UPLOAD_DIR="$simulation_tmp/uploads" GOOKI_HOST="http://127.0.0.1:$mock_port" GOOKI_USERNAME=ci-demo GOOKI_PASSWORD=ci-simulation-upstream-password "$simulation_tmp/cc4-server" > "$simulation_tmp/server.log" 2>&1 &
server_pid=$!
for attempt in {1..60}; do
  if curl -fsS "http://127.0.0.1:$server_port/health" >/dev/null 2>&1 && curl -fsS "http://127.0.0.1:$mock_port/health" >/dev/null 2>&1; then break; fi
  sleep 0.2
done
export CC4_SIMULATION_ALLOW_WRITE=true CC4_SIMULATION_ADMIN_PHONE=00099999999 CC4_SIMULATION_ADMIN_PASSWORD=ci-simulation-password CC4_SIMULATION_SEED=ci-business-simulation-deterministic-seed
python3 simulation/run.py --base-url "http://127.0.0.1:$server_port/api/v1" --days 7
export_report first
before=$(docker exec "$container" psql -U "$db_user" -d "$simulation_db" -Atc "$counts_sql")
python3 simulation/run.py --base-url "http://127.0.0.1:$server_port/api/v1" --days 7
export_report repeat
after=$(docker exec "$container" psql -U "$db_user" -d "$simulation_db" -Atc "$counts_sql")
[[ $before == "$after" ]] || { echo 'Repeat simulation changed business counts.' >&2; exit 1; }
sentinel_after=$(docker exec "$container" psql -U "$db_user" -d "$simulation_db" -Atc "SELECT row_to_json(p)::text FROM performance_records p JOIN employees e ON e.id=p.employee_id WHERE e.employee_code='SIM_CI_ADMIN'")
[[ $sentinel_before == "$sentinel_after" ]] || { echo 'Simulation changed an unrelated monthly record.' >&2; exit 1; }
if [[ -n $report_dir ]]; then
  printf '%s\n' "$before" > "$report_dir/counts-before.json"
  printf '%s\n' "$after" > "$report_dir/counts-after.json"
  printf '%s\n' "$sentinel_before" > "$report_dir/unrelated-month-before.json"
  printf '%s\n' "$sentinel_after" > "$report_dir/unrelated-month-after.json"
fi
echo 'Real HTTP business simulation and duplicate-run invariants passed.'
docker exec "$container" psql -q -U "$db_user" -d "$simulation_db" -c "UPDATE finance_records SET amount=amount+1 WHERE id=(SELECT id FROM finance_records WHERE reference_type='daily_performance' ORDER BY id LIMIT 1)" >/dev/null
if python3 simulation/run.py --base-url "http://127.0.0.1:$server_port/api/v1" --days 7; then
  echo 'Fault injection was not detected.' >&2
  exit 1
fi
verdict=$(docker exec "$container" psql -U "$db_user" -d "$simulation_db" -Atc "SELECT value::jsonb->>'status' FROM system_settings WHERE key='business_simulation_report_v1'")
[[ $verdict == failed ]]
export_report negative
echo 'Injected 1 yuan accounting discrepancy correctly failed the simulation and persisted its report.'
