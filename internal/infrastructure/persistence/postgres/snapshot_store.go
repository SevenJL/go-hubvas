package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hubvas/internal/domain/canvas"
)

// SnapshotStore implements canvas.SnapshotRepository using PostgreSQL.
type SnapshotStore struct {
	pool *pgxpool.Pool
}

// NewSnapshotStore creates a SnapshotStore.
func NewSnapshotStore(pool *pgxpool.Pool) *SnapshotStore {
	return &SnapshotStore{pool: pool}
}

// Save persists a tldraw store JSON snapshot, upserting by canvas ID.
func (s *SnapshotStore) Save(ctx context.Context, canvasID canvas.CanvasID, data []byte) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO canvas_snapshots (canvas_id, data, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (canvas_id) DO UPDATE SET data = $2, updated_at = NOW()`,
		int64(canvasID), data,
	)
	if err != nil {
		return fmt.Errorf("save snapshot for canvas %d: %w", canvasID, err)
	}
	return nil
}

// Load retrieves the snapshot for a canvas. Returns nil, nil if none exists.
func (s *SnapshotStore) Load(ctx context.Context, canvasID canvas.CanvasID) ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(ctx,
		`SELECT data FROM canvas_snapshots WHERE canvas_id = $1`, int64(canvasID),
	).Scan(&data)
	if err != nil {
		return nil, nil // no rows → no snapshot
	}
	return data, nil
}

// Ensure it satisfies the domain interface.
var _ canvas.SnapshotRepository = (*SnapshotStore)(nil)
