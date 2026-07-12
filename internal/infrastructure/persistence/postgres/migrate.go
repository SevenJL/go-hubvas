package postgres

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext('hubvas_schema_migrations'))`); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext('hubvas_schema_migrations'))`)
	if _, err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations(
			version BIGINT PRIMARY KEY,
			name TEXT NOT NULL,
			checksum TEXT,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum TEXT;
	`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return fmt.Errorf("invalid migration filename %s", entry.Name())
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid migration version %s: %w", entry.Name(), err)
		}
		sql, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		digest := sha256.Sum256(sql)
		checksum := hex.EncodeToString(digest[:])

		var appliedName string
		var appliedChecksum *string
		err = conn.QueryRow(ctx, `SELECT name,checksum FROM schema_migrations WHERE version=$1`, version).Scan(&appliedName, &appliedChecksum)
		switch {
		case err == nil:
			if appliedName != entry.Name() {
				return fmt.Errorf("migration version %d is recorded as %q, expected %q", version, appliedName, entry.Name())
			}
			if appliedChecksum == nil || *appliedChecksum == "" {
				// Upgrade databases created by the first version of the migration
				// runner. Future edits are detected after this one-time baseline.
				if _, err = conn.Exec(ctx, `UPDATE schema_migrations SET checksum=$1 WHERE version=$2 AND checksum IS NULL`, checksum, version); err != nil {
					return err
				}
				continue
			}
			if *appliedChecksum != checksum {
				return fmt.Errorf("migration %s checksum changed after it was applied", entry.Name())
			}
			continue
		case err != pgx.ErrNoRows:
			return err
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version,name,checksum) VALUES($1,$2,$3)`, version, entry.Name(), checksum); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
