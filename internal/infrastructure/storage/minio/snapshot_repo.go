package minio

import (
	"context"

	"github.com/hubvas/internal/domain/collaboration"
)

// SnapshotRepo implements collaboration.SnapshotRepository using MinIO / S3.
// Snapshots are stored as binary blobs keyed by canvas ID:
//
//	snapshots/{canvasID}/latest.bin
//
// For version history, snapshots can be archived with a version suffix:
//
//	snapshots/{canvasID}/v{version}.bin
type SnapshotRepo struct {
	// client     *minio.Client — to be wired in during implementation.
	// bucketName string
}

// NewSnapshotRepo creates a new SnapshotRepo.
func NewSnapshotRepo( /* client *minio.Client, bucketName string */ ) *SnapshotRepo {
	return &SnapshotRepo{}
}

// Save persists a CRDT document snapshot.
func (r *SnapshotRepo) Save(ctx context.Context, canvasID collaboration.RoomID, data []byte) error {
	// TODO: Upload to MinIO/S3
	return nil
}

// Load retrieves the latest CRDT document snapshot.
func (r *SnapshotRepo) Load(ctx context.Context, canvasID collaboration.RoomID) ([]byte, error) {
	// TODO: Download from MinIO/S3
	return nil, nil
}

// Delete removes all snapshots for a canvas.
func (r *SnapshotRepo) Delete(ctx context.Context, canvasID collaboration.RoomID) error {
	// TODO: Delete from MinIO/S3
	return nil
}
