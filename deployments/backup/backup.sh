#!/bin/sh
set -eu
: "${BACKUP_DIR:?BACKUP_DIR is required}"
: "${DATABASE_URL:?DATABASE_URL is required}"
: "${MINIO_ALIAS:?MINIO_ALIAS is required}"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
target="$BACKUP_DIR/$timestamp"
mkdir -p "$target"
pg_dump --format=custom --no-owner --dbname="$DATABASE_URL" --file="$target/postgres.dump"
mc mirror --overwrite "$MINIO_ALIAS/hubvas-snapshots" "$target/hubvas-snapshots"
mc mirror --overwrite "$MINIO_ALIAS/hubvas-media" "$target/hubvas-media"
(
  cd "$target"
  find . -type f ! -name SHA256SUMS -exec sha256sum '{}' \; | LC_ALL=C sort > SHA256SUMS
  sha256sum -c SHA256SUMS >/dev/null
)
echo "backup completed and verified: $target"
