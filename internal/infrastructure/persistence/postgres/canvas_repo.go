package postgres

import (
	"context"
	"database/sql"

	"github.com/hubvas/internal/domain/canvas"
)

// CanvasRepo implements canvas.CanvasRepository using PostgreSQL.
type CanvasRepo struct {
	db *sql.DB
}

// NewCanvasRepo creates a new CanvasRepo.
func NewCanvasRepo(db *sql.DB) *CanvasRepo {
	return &CanvasRepo{db: db}
}

func (r *CanvasRepo) Save(ctx context.Context, c *canvas.Canvas) error       { return nil }
func (r *CanvasRepo) FindByID(ctx context.Context, id canvas.CanvasID) (*canvas.Canvas, error) {
	return nil, nil
}
func (r *CanvasRepo) FindByOwner(ctx context.Context, ownerID uint64) ([]*canvas.Canvas, error) {
	return nil, nil
}
func (r *CanvasRepo) FindByMember(ctx context.Context, userID uint64) ([]*canvas.Canvas, error) {
	return nil, nil
}
func (r *CanvasRepo) Delete(ctx context.Context, id canvas.CanvasID) error  { return nil }
