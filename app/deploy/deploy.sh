#!/usr/bin/env bash
# Root's SSH forced command accepts only a verified release. Binaries and SQL
# run as cc4. A failed health check rolls the executable/static files back;
# committed database migrations are retained and must remain backward compatible.
set -Eeuo pipefail
umask 077
if [[ ${SSH_ORIGINAL_COMMAND:-} =~ ^deploy\ ([0-9a-f]{40})$ ]]; then
  revision="${BASH_REMATCH[1]}"
else
  echo 'Only deploy <40-character commit SHA> is accepted.' >&2
  exit 64
fi
base=/opt/cc4
exec 9>"$base/.deploy.lock"
flock -w 120 9
incoming=$(mktemp -d "$base/.incoming.XXXXXX")
previous=$(readlink "$base/current" || true)
switched=0
finish() {
  result=$?
  trap - EXIT
  if [[ $result -ne 0 && $switched -eq 1 ]]; then
    if [[ -n $previous ]]; then
      ln -s "$previous" "$base/rollback.next"
      mv -Tf "$base/rollback.next" "$base/current"
      timeout --foreground 45 systemctl restart cc4.service || true
      echo 'Deployment failed; previous application release restored. Database backup is retained.' >&2
    else
      timeout --foreground 45 systemctl stop cc4.service || true
      rm -f "$base/current"
    fi
  fi
  rm -rf -- "$incoming"
  exit "$result"
}
trap finish EXIT
trap 'exit 124' TERM INT HUP
timeout 90 head -c 33554433 > "$incoming/bundle.tar.gz"
[[ $(stat -c %s "$incoming/bundle.tar.gz") -le 33554432 ]]
python3 /usr/local/lib/cc4/verify_release.py "$incoming" "$revision"
release="$base/releases/$revision"
if [[ -d $release ]]; then
  cmp "$incoming/verified/SHA256SUMS" "$release/SHA256SUMS"
else
  mv "$incoming/verified" "$release"
  find "$release" -type d -exec chmod 0755 {} +
  find "$release" -type f -exec chmod 0644 {} +
  chmod 0755 "$release/bin/cc4-server"
fi
timeout --foreground 90 runuser -u cc4 -- /usr/local/lib/cc4/backup.sh
timeout --foreground 150 runuser -u cc4 -- python3 /usr/local/lib/cc4/migrate.py "$release/migrations"
ln -s "$release" "$base/current.next"
mv -Tf "$base/current.next" "$base/current"
switched=1
timeout --foreground 45 systemctl restart cc4.service
healthy=0
for attempt in {1..30}; do
  if curl --fail --silent --max-time 2 http://127.0.0.1:3101/health > "$incoming/health.json" &&
    python3 - "$incoming/health.json" "$revision" <<'PY'
import json,sys
data=json.load(open(sys.argv[1]))
raise SystemExit(0 if data.get('status')=='OK' and data.get('revision')==sys.argv[2] else 1)
PY
  then healthy=1; break; fi
  sleep 0.5
done
[[ $healthy -eq 1 ]] || { journalctl -u cc4.service -n 10 --no-pager >&2; exit 1; }
printf 'Deployed CC4 revision %s\n' "$revision"
