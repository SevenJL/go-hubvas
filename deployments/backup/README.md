# Backup and restore

Run `backup.sh` from a scheduled, isolated operations host that has `pg_dump`, GNU `sha256sum`, and the MinIO `mc` client installed. `BACKUP_DIR` must point to encrypted storage outside the production host. Configure the MinIO alias before execution.

Required variables: `DATABASE_URL`, `MINIO_ALIAS`, and `BACKUP_DIR`. The backup contains a PostgreSQL custom-format dump, both object-storage buckets, and a SHA-256 manifest covering every copied file. The script verifies the manifest before reporting success.

Restore drills use `restore.sh` with `BACKUP_PATH`, `DATABASE_URL`, and `MINIO_ALIAS` against a non-production environment first. Restore verifies all checksums, recreates the database objects, and mirrors the backup back to both buckets with `--remove`; objects not present in the selected backup are intentionally deleted from the restore target.

Production policy: daily database/object backups, PostgreSQL WAL/PITR at the managed database layer, object versioning/replication, backup-failure alerts, and a monthly restore drill. These scripts are the portable baseline, not a replacement for provider-managed PITR or storage-native snapshots. For a strict point-in-time application backup, quiesce writes or coordinate database and object-store snapshots at the platform layer.
