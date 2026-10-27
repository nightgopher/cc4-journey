#!/usr/bin/env bash
# Fixed installer for the signed receiver and optional SSH forced command.
# Never executes uploaded code as root; systemd uses the dedicated service user.
set -Eeuo pipefail
umask 077
if [[ ${SSH_ORIGINAL_COMMAND:-} =~ ^deploy\ ([0-9a-f]{40})$ ]]; then
  revision="${BASH_REMATCH[1]}"
else
  echo 'Only deploy <40-character commit SHA> is accepted.' >&2
  exit 64
fi
base=/opt/cc4-lead-source-mock
exec 9>"$base/.deploy.lock"
flock -w 120 9
incoming=$(mktemp -d "$base/.incoming.XXXXXX")
previous=$(readlink "$base/current" || true)
switched=0
rollback() {
  status=$?
  trap - EXIT
  if [[ $status -ne 0 && $switched -eq 1 ]]; then
    if [[ -n $previous ]]; then
      ln -s "$previous" "$base/rollback.next"
      mv -Tf "$base/rollback.next" "$base/current"
      timeout --foreground 50 systemctl restart cc4-lead-source-mock.service || true
      echo 'Deployment failed; previous release restored.' >&2
    else
      timeout --foreground 50 systemctl stop cc4-lead-source-mock.service || true
      rm -f "$base/current"
      echo 'First deployment failed; service stopped.' >&2
    fi
  fi
  rm -rf -- "$incoming"
  exit "$status"
}
trap rollback EXIT
trap 'exit 124' TERM INT
timeout 60 head -c 33554433 > "$incoming/bundle.tar.gz"
[[ $(stat -c %s "$incoming/bundle.tar.gz") -le 33554432 ]]
python3 /usr/local/lib/cc4-lead-source-mock/verify_release.py "$incoming"
release="$base/releases/$revision"
if [[ -d $release ]]; then
  cmp "$incoming/SHA256SUMS" "$release/SHA256SUMS"
else
  install -d -m 0755 "$release"
  install -m 0755 "$incoming/lead-source-mock" "$release/lead-source-mock"
  install -m 0644 "$incoming/SHA256SUMS" "$release/SHA256SUMS"
fi
ln -s "$release" "$base/current.next"
mv -Tf "$base/current.next" "$base/current"
switched=1
timeout --foreground 50 systemctl restart cc4-lead-source-mock.service
healthy=0
for attempt in {1..40}; do
  if curl --fail --silent --max-time 2 http://127.0.0.1:18081/health > "$incoming/health.json" &&
    python3 - "$incoming/health.json" "$revision" <<'PY'
import json,sys
data=json.load(open(sys.argv[1]))
raise SystemExit(0 if data.get('status')=='ok' and data.get('mock') is True and data.get('revision')==sys.argv[2] else 1)
PY
  then healthy=1; break; fi
  sleep 0.5
done
if [[ $healthy -ne 1 ]]; then
  journalctl -u cc4-lead-source-mock.service -n 12 --no-pager >&2
  exit 1
fi
printf 'Deployed cc4-lead-source-mock revision %s\n' "$revision"
