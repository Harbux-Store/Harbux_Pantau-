#!/bin/bash
# Backup harian database Harbux (MySQL/MariaDB).
#
# Pasang:
#   cp deploy/backup-harbux.sh /usr/local/bin/backup-harbux
#   chmod +x /usr/local/bin/backup-harbux
#   crontab -e   ->   0 2 * * * /usr/local/bin/backup-harbux
#
# Kredensial dibaca dari /root/.my.cnf supaya sandi tidak tampil di daftar proses:
#   [client]
#   user=harbux
#   password=SANDI
set -euo pipefail

DB="${HARBUX_DB_NAME:-harbux}"
DIR="${HARBUX_BACKUP_DIR:-/var/backups/harbux}"
KEEP_DAYS="${HARBUX_BACKUP_KEEP:-14}"
FILE="$DIR/harbux-$(date +%F-%H%M).sql.gz"

mkdir -p "$DIR"
mysqldump --single-transaction --quick --routines "$DB" | gzip -9 > "$FILE"

# buang backup yang lebih tua dari KEEP_DAYS hari
find "$DIR" -name 'harbux-*.sql.gz' -mtime "+$KEEP_DAYS" -delete

echo "Backup selesai: $FILE ($(du -h "$FILE" | cut -f1))"
