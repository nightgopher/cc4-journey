#!/usr/bin/env bash
# Install Certbot >=5.4 in /opt/cc4-certbot beforehand. Uses the existing port
# 80 ACME webroot; neither Nginx nor the business application is stopped.
set -Eeuo pipefail
[[ $(id -u) -eq 0 ]]
script_dir=$(cd "$(dirname "$0")" && pwd)
install -d -m 0755 /var/lib/cc4-acme
install -m 0750 "$script_dir/tls-reload.sh" /usr/local/sbin/cc4-tls-reload
/opt/cc4-certbot/bin/certbot certonly --non-interactive --agree-tos --register-unsafely-without-email \
  --preferred-profile shortlived --webroot --webroot-path /var/lib/cc4-acme \
  --ip-address 150.5.143.32 --cert-name cc4-ip --deploy-hook /usr/local/sbin/cc4-tls-reload
install -m 0644 "$script_dir/nginx-tls.conf" /etc/nginx/sites-available/cc4-tls
ln -sfn /etc/nginx/sites-available/cc4-tls /etc/nginx/sites-enabled/cc4-tls
if ! nginx -t; then rm -f /etc/nginx/sites-enabled/cc4-tls; exit 1; fi
systemctl reload nginx
install -m 0644 "$script_dir/cc4-certbot-renew.service" "$script_dir/cc4-certbot-renew.timer" /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now cc4-certbot-renew.timer
