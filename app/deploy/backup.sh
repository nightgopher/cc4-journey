#!/usr/bin/env bash
set -Eeuo pipefail
umask 077
backup_root=/var/lib/cc4/backups
backup_dir=$(mktemp -d "$backup_root/$(date -u +%Y%m%dT%H%M%SZ).XXXXXX")
trap 'rm -f "$backup_dir/.incomplete"' EXIT
touch "$backup_dir/.incomplete"
pg_dump -d cc4 --no-owner --no-acl --format=custom --file="$backup_dir/database.dump"
tar -czf "$backup_dir/uploads.tar.gz" -C /var/lib/cc4 uploads
touch "$backup_dir/COMPLETE"
find "$backup_root" -mindepth 1 -maxdepth 1 -type d -mtime +14 -exec rm -rf -- {} +
echo 'Database and uploads backup completed.'
