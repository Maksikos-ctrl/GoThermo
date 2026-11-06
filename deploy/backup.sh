#!/usr/bin/env bash
# Encrypted backup of the Postgres database.
#
#   sudo ./deploy/backup.sh                 # one backup now
#
# Setup once (see SERVER.md, "Backups"):
#   openssl rand -base64 32 | sudo tee /opt/gothermo-backup.pass >/dev/null
#   sudo chmod 600 /opt/gothermo-backup.pass
# KEEP A COPY OF THAT PASSPHRASE SOMEWHERE ELSE: without it the backups are useless.
#
# Variables: BACKUP_DIR (default /opt/gothermo-backups), BACKUP_PASSPHRASE_FILE
# (default /opt/gothermo-backup.pass), KEEP_DAYS (default 14).
set -euo pipefail
cd "$(dirname "$0")/.."

DEST="${BACKUP_DIR:-/opt/gothermo-backups}"
PASS="${BACKUP_PASSPHRASE_FILE:-/opt/gothermo-backup.pass}"
KEEP_DAYS="${KEEP_DAYS:-14}"

if [ ! -s "$PASS" ]; then
  echo "No passphrase file at $PASS (see the header of this script)." >&2
  exit 1
fi

umask 077
mkdir -p "$DEST"
out="$DEST/gothermo-$(date +%Y%m%d-%H%M%S).sql.gz.enc"
trap 'rm -f "$out"' ERR

docker compose exec -T postgres pg_dump -U gothermo -d gothermo --no-owner \
  | gzip \
  | openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt -pass "file:$PASS" -out "$out"

trap - ERR
find "$DEST" -name 'gothermo-*.sql.gz.enc' -mtime "+$KEEP_DAYS" -delete
echo "Backup written: $out ($(du -h "$out" | cut -f1))"
