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

// Save persists a tldraw store JSON snapshot with an optional PNG thumbnail (base64 data URL).
func (s *SnapshotStore) Save(ctx context.Context, canvasID canvas.CanvasID, data []byte, thumbnail string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO canvas_snapshots (canvas_id, data, thumbnail, updated_at)
		 VALUES ($1, $2, NULLIF($3, ''), NOW())
		 ON CONFLICT (canvas_id) DO UPDATE SET data = $2, thumbnail = NULLIF($3, ''), updated_at = NOW()`,
		int64(canvasID), data, thumbnail,
	)
	if err != nil {
		return fmt.Errorf("save snapshot for canvas %d: %w", canvasID, err)
	}
	return nil
}

// Load retrieves the snapshot data and thumbnail. Returns empty strings for none.
func (s *SnapshotStore) Load(ctx context.Context, canvasID canvas.CanvasID) ([]byte, string, error) {
	var data []byte
	var thumbnail *string
	err := s.pool.QueryRow(ctx,
		`SELECT data, thumbnail FROM canvas_snapshots WHERE canvas_id = $1`, int64(canvasID),
	).Scan(&data, &thumbnail)
	if err != nil {
		return nil, "", nil // no rows → no snapshot
	}
	t := ""
	if thumbnail != nil {
		t = *thumbnail
	}
	return data, t, nil
}

var _ canvas.SnapshotRepository = (*SnapshotStore)(nil)
