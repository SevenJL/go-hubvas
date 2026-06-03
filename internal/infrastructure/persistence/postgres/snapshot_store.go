package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SnapshotStore persists tldraw store snapshots in PostgreSQL.
// Each canvas has one row in canvas_snapshots with its JSON snapshot.
type SnapshotStore struct {
	pool *pgxpool.Pool
}

// NewSnapshotStore creates a SnapshotStore.
func NewSnapshotStore(pool *pgxpool.Pool) *SnapshotStore {
	return &SnapshotStore{pool: pool}
}

// SaveSnapshot upserts the snapshot for a canvas.
func (s *SnapshotStore) SaveSnapshot(ctx context.Context, canvasID int64, data json.RawMessage) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO canvas_snapshots (canvas_id, data, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (canvas_id) DO UPDATE SET data = $2, updated_at = NOW()`,
		canvasID, data,
	)
	if err != nil {
		return fmt.Errorf("save snapshot for canvas %d: %w", canvasID, err)
	}
	return nil
}

// LoadSnapshot retrieves the snapshot for a canvas.
// Returns nil, nil if no snapshot exists.
func (s *SnapshotStore) LoadSnapshot(ctx context.Context, canvasID int64) (json.RawMessage, error) {
	var data json.RawMessage
	err := s.pool.QueryRow(ctx,
		`SELECT data FROM canvas_snapshots WHERE canvas_id = $1`, canvasID,
	).Scan(&data)
	if err != nil {
		// No rows → no snapshot.
		return nil, nil
	}
	return data, nil
}
