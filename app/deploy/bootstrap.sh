#!/usr/bin/env bash
# One-time root setup. PostgreSQL is installed separately using apt.
set -Eeuo pipefail
[[ $(id -u) -eq 0 ]]
script_dir=$(cd "$(dirname "$0")" && pwd)
cd /
public_key=${1:?Pass the dedicated Actions Ed25519 public key file}
python3 - "$public_key" <<'PY'
import pathlib,re,sys
if not re.fullmatch(r'ssh-ed25519 [A-Za-z0-9+/=]+(?: [^\n]+)?',pathlib.Path(sys.argv[1]).read_text().strip()):
    raise SystemExit('Expected one Ed25519 public key')
PY
id cc4 >/dev/null 2>&1 || useradd --system --create-home --home-dir /var/lib/cc4 --shell /usr/sbin/nologin cc4
install -d -m 0755 /opt/cc4/releases /usr/local/lib/cc4
install -d -o cc4 -g cc4 -m 0700 /var/lib/cc4/uploads /var/lib/cc4/backups
install -m 0750 "$script_dir/deploy.sh" /usr/local/sbin/cc4-deploy
install -m 0755 "$script_dir/backup.sh" /usr/local/lib/cc4/backup.sh
install -m 0644 "$script_dir/verify_release.py" "$script_dir/migrate.py" /usr/local/lib/cc4/
install -m 0644 "$script_dir/cc4.service" "$script_dir/cc4-backup.service" "$script_dir/cc4-backup.timer" /etc/systemd/system/
if [[ ! -f /etc/cc4.env ]]; then
python3 - <<'PY'
import os,secrets,subprocess
os.umask(0o077)
password=secrets.token_hex(32)
sql="CREATE ROLE cc4 LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '"+password+"';"
subprocess.run(['runuser','-u','postgres','--','psql','-X','-q','-v','ON_ERROR_STOP=1'],input=sql,text=True,check=True)
subprocess.run(['runuser','-u','postgres','--','createdb','--owner=cc4','cc4'],check=True)
with open('/etc/cc4.env','x') as target:
    target.write('HOST=127.0.0.1\nPORT=3101\nDB_HOST=127.0.0.1\nDB_PORT=5432\nDB_NAME=cc4\nDB_USER=cc4\nDB_PASSWORD='+password+'\nJWT_SECRET='+secrets.token_hex(48)+'\nUPLOAD_DIR=/var/lib/cc4/uploads\nBUSINESS_TIMEZONE=Asia/Shanghai\nENABLE_CRON=true\nGOMEMLIMIT=320MiB\nGOMAXPROCS=2\n')
PY
fi
chmod 0600 /etc/cc4.env
install -d -m 0700 /root/.ssh
touch /root/.ssh/authorized_keys
chmod 0600 /root/.ssh/authorized_keys
key=$(cat "$public_key")
if ! grep -Fq -- "$key" /root/.ssh/authorized_keys; then
  printf '\nrestrict,command="/usr/local/sbin/cc4-deploy" %s\n' "$key" >> /root/.ssh/authorized_keys
fi
install -m 0644 "$script_dir/nginx-location.conf" /etc/nginx/snippets/cc4.conf
install -m 0644 "$script_dir/nginx-port.conf" /etc/nginx/sites-available/cc4-port
ln -sfn /etc/nginx/sites-available/cc4-port /etc/nginx/sites-enabled/cc4-port
site=$(readlink -f /etc/nginx/sites-enabled/ai-video-studio)
backup=$(mktemp /var/backups/cc4-nginx-before.XXXXXX)
cp -p "$site" "$backup"
python3 - "$site" <<'PY'
import pathlib,sys
path=pathlib.Path(sys.argv[1]);source=path.read_text()
include='    include /etc/nginx/snippets/cc4.conf;'
if include not in source:
    anchor='    server_name _;'
    if source.count(anchor)!=1:raise SystemExit('Inspect the HTTP default server before adding the include')
    path.write_text(source.replace(anchor,anchor+'\n'+include,1))
PY
if ! nginx -t; then cp -p "$backup" "$site"; exit 1; fi
systemctl reload nginx
systemctl daemon-reload
systemctl enable cc4.service
systemctl enable --now cc4-backup.timer
echo 'CC4 infrastructure prepared; import the initial database or let the first release create its schema.'
