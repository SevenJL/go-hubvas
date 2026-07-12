#!/bin/sh
set -eu
: "${BACKUP_PATH:?BACKUP_PATH is required}"
: "${DATABASE_URL:?DATABASE_URL is required}"
: "${MINIO_ALIAS:?MINIO_ALIAS is required}"
(
  cd "$BACKUP_PATH"
  sha256sum -c SHA256SUMS
)
pg_restore --clean --if-exists --no-owner --dbname="$DATABASE_URL" "$BACKUP_PATH/postgres.dump"
mc mirror --overwrite --remove "$BACKUP_PATH/hubvas-snapshots" "$MINIO_ALIAS/hubvas-snapshots"
mc mirror --overwrite --remove "$BACKUP_PATH/hubvas-media" "$MINIO_ALIAS/hubvas-media"
echo "restore completed from $BACKUP_PATH"
