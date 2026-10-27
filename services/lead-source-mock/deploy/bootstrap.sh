#!/usr/bin/env bash
# One-time infrastructure preparation as root. The application binary is shipped
# by GitHub Actions, never by this bootstrap script.
set -Eeuo pipefail
[[ $(id -u) -eq 0 ]] || { echo 'Run bootstrap as root.' >&2; exit 1; }
script_dir=$(cd "$(dirname "$0")" && pwd)
public_key=${1:-}
for command in systemctl nginx curl python3 flock; do command -v "$command" >/dev/null; done
if [[ -n $public_key ]]; then python3 - "$public_key" <<'PY'
import pathlib,re,sys
key=pathlib.Path(sys.argv[1]).read_text().strip()
if not re.fullmatch(r'ssh-ed25519 [A-Za-z0-9+/=]+(?: [^\n]+)?',key):
    raise SystemExit('Expected one Ed25519 public key')
PY
fi
id cc4-lead-mock >/dev/null 2>&1 || useradd --system --no-create-home --home-dir /var/lib/cc4-lead-source-mock --shell /usr/sbin/nologin cc4-lead-mock
install -d -m 0755 /opt/cc4-lead-source-mock/releases
install -m 0750 "$script_dir/deploy.sh" /usr/local/sbin/cc4-lead-source-deploy
install -d -m 0755 /usr/local/lib/cc4-lead-source-mock
install -m 0644 "$script_dir/verify_release.py" /usr/local/lib/cc4-lead-source-mock/verify_release.py
install -m 0644 "$script_dir/deploy_http.py" /usr/local/lib/cc4-lead-source-mock/deploy_http.py
install -m 0644 "$script_dir/cc4-lead-source-deploy.service" /etc/systemd/system/cc4-lead-source-deploy.service
install -m 0644 "$script_dir/cc4-lead-source-mock.service" /etc/systemd/system/cc4-lead-source-mock.service
if [[ ! -f /etc/cc4-lead-source-mock.env ]]; then
  python3 - <<'PY'
import os,secrets
os.umask(0o077)
with open('/etc/cc4-lead-source-mock.env','x') as file:
    file.write('HOST=127.0.0.1\nPORT=18081\nBUSINESS_TIMEZONE=Asia/Shanghai\nMOCK_RECORDS_PER_DAY=120\nMOCK_SCENARIO=normal\nMOCK_USERNAME=cc4-demo\n')
    file.write('MOCK_PASSWORD='+secrets.token_urlsafe(32)+'\n')
    file.write('MOCK_TOKEN_SECRET='+secrets.token_hex(32)+'\n')
PY
fi
chmod 0600 /etc/cc4-lead-source-mock.env
if [[ ! -f /etc/cc4-lead-source-deploy.env ]]; then
  python3 - <<'PY'
import os,secrets
os.umask(0o077)
with open('/etc/cc4-lead-source-deploy.env','x') as file:
    file.write('CC4_MOCK_DEPLOY_SECRET='+secrets.token_hex(32)+'\n')
PY
fi
chmod 0600 /etc/cc4-lead-source-deploy.env
if [[ -n $public_key ]]; then
install -d -m 0700 /root/.ssh
touch /root/.ssh/authorized_keys
chmod 0600 /root/.ssh/authorized_keys
key=$(cat "$public_key")
if ! grep -Fq -- "$key" /root/.ssh/authorized_keys; then
  printf '\nrestrict,command="/usr/local/sbin/cc4-lead-source-deploy" %s\n' "$key" >> /root/.ssh/authorized_keys
fi
fi
install -m 0644 "$script_dir/nginx-location.conf" /etc/nginx/snippets/cc4-lead-source-mock.conf
site=$(readlink -f /etc/nginx/sites-enabled/orionstar)
backup=$(mktemp /var/backups/cc4-nginx-before.XXXXXX)
cp -p "$site" "$backup"
python3 - "$site" <<'PY'
import pathlib,sys
path=pathlib.Path(sys.argv[1]);source=path.read_text()
include='    include /etc/nginx/snippets/cc4-lead-source-mock.conf;'
if include not in source:
    anchor='    server_name _;'
    if source.count(anchor)!=1:raise SystemExit('Expected exactly one HTTP default server; inspect before editing')
    path.write_text(source.replace(anchor,anchor+'\n'+include,1))
PY
if ! nginx -t; then cp -p "$backup" "$site"; exit 1; fi
systemctl reload nginx
systemctl daemon-reload
systemctl enable cc4-lead-source-deploy.service
systemctl restart cc4-lead-source-deploy.service
systemctl enable cc4-lead-source-mock.service
echo 'Infrastructure prepared. Waiting for the first GitHub Actions deployment.'
