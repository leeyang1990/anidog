#!/usr/bin/env bash
set -euo pipefail

project_dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
backup_dir="${ANIDOG_BACKUP_DIR:-$project_dir/backups}"
retention_days="${ANIDOG_BACKUP_RETENTION_DAYS:-14}"

case "$backup_dir" in
  ""|"/")
    echo "拒绝使用不安全的备份目录: $backup_dir" >&2
    exit 1
    ;;
esac
case "$retention_days" in
  *[!0-9]*|"")
    echo "ANIDOG_BACKUP_RETENTION_DAYS 必须是非负整数" >&2
    exit 1
    ;;
esac

umask 077
mkdir -p "$backup_dir"
stamp="$(date +%Y%m%d-%H%M%S)"
tmp_dir="$(mktemp -d "$backup_dir/.anidog-backup-$stamp.XXXXXX")"
trap 'rm -rf -- "$tmp_dir"' EXIT

db_file="$tmp_dir/anidog-db-$stamp.sql.gz"
qbit_file="$tmp_dir/anidog-qbit-$stamp.tar.gz"

docker exec anidog-postgres sh -c 'exec pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' | gzip -9 > "$db_file"
docker exec anidog-qbittorrent tar -czf - -C /config . > "$qbit_file"

gzip -t "$db_file"
tar -tzf "$qbit_file" >/dev/null

if [[ -f "$project_dir/.env" ]]; then
  install -m 600 "$project_dir/.env" "$tmp_dir/anidog-env-$stamp"
fi

(
  cd "$tmp_dir"
  sha256sum anidog-* > "anidog-checksums-$stamp.sha256"
)

find "$tmp_dir" -maxdepth 1 -type f -name 'anidog-*' -exec mv -- {} "$backup_dir/" \;
find "$backup_dir" -maxdepth 1 -type f -name 'anidog-*' -mtime "+$retention_days" -delete

echo "AniDog 备份完成: $backup_dir ($stamp)"
