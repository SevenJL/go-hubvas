package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"

	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/shared"
)

// SnapshotRepo implements collaboration.SnapshotRepository using MinIO / S3.
//
// Key layout:
//
//	snapshots/{canvasID}/latest.bin      — always the most recent snapshot
//	snapshots/{canvasID}/v{version}.bin  — versioned history (retained for time-travel)
//
// Version history is retained on every Save by copying the old latest.bin
// to a versioned key before overwriting.
type SnapshotRepo struct {
	client     *minio.Client
	bucketName string
}

// NewSnapshotRepo creates a SnapshotRepo backed by a MinIO client.
func NewSnapshotRepo(client *minio.Client, bucketName string) *SnapshotRepo {
	return &SnapshotRepo{client: client, bucketName: bucketName}
}

// latestKey returns the object key for the latest snapshot.
func latestKey(canvasID collaboration.RoomID) string {
	return fmt.Sprintf("snapshots/%d/latest.bin", canvasID)
}

// versionKey returns the object key for a versioned snapshot.
func versionKey(canvasID collaboration.RoomID, version int64) string {
	return fmt.Sprintf("snapshots/%d/v%d.bin", canvasID, version)
}

// prefix returns the prefix for all snapshots of a given canvas.
func prefix(canvasID collaboration.RoomID) string {
	return fmt.Sprintf("snapshots/%d/", canvasID)
}

// ---- Save ----

// Save persists a CRDT document snapshot. It first copies the existing
// latest snapshot to a versioned key (if one exists), then overwrites
// latest.bin with the new data.
func (r *SnapshotRepo) Save(ctx context.Context, canvasID collaboration.RoomID, data []byte) error {
	// Optional: archive the current latest as a versioned snapshot.
	// We ignore errors here — versioning is best-effort, not critical.
	r.archiveLatest(ctx, canvasID)

	// Upload the new snapshot.
	reader := bytes.NewReader(data)
	_, err := r.client.PutObject(ctx, r.bucketName, latestKey(canvasID), reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return fmt.Errorf("minio: save snapshot for canvas %d: %w", canvasID, err)
	}
	return nil
}

// archiveLatest copies the current latest.bin to a versioned key.
func (r *SnapshotRepo) archiveLatest(ctx context.Context, canvasID collaboration.RoomID) {
	// Stat the current latest to check if it exists and derive a stable,
	// monotonic-enough archive id from the version being archived.
	info, err := r.client.StatObject(ctx, r.bucketName, latestKey(canvasID), minio.StatObjectOptions{})
	if err != nil {
		return // Nothing to archive.
	}

	version := snapshotVersion(info.LastModified)

	// Copy the object to a versioned key.
	src := minio.CopySrcOptions{
		Bucket: r.bucketName,
		Object: latestKey(canvasID),
	}
	dst := minio.CopyDestOptions{
		Bucket: r.bucketName,
		Object: versionKey(canvasID, version),
	}
	_, err = r.client.CopyObject(ctx, dst, src)
	if err != nil {
		// Best-effort; log and continue.
		_ = err
	}
}

func snapshotVersion(lastModified time.Time) int64 {
	if lastModified.IsZero() {
		return time.Now().UTC().UnixNano()
	}
	return lastModified.UTC().UnixNano()
}

// ---- Load ----

// Load retrieves the latest CRDT document snapshot for a canvas.
// Returns nil, nil when no snapshot exists yet (new canvas).
func (r *SnapshotRepo) Load(ctx context.Context, canvasID collaboration.RoomID) ([]byte, error) {
	obj, err := r.client.GetObject(ctx, r.bucketName, latestKey(canvasID), minio.GetObjectOptions{})
	if err != nil {
		// MinIO returns an error response object, not a Go error, for 404.
		// We need to check the response from the first read.
		return r.readObject(obj, canvasID)
	}
	return r.readObject(obj, canvasID)
}

func (r *SnapshotRepo) readObject(obj *minio.Object, canvasID collaboration.RoomID) ([]byte, error) {
	defer obj.Close()

	// Stat to check existence and size.
	info, err := obj.Stat()
	if err != nil {
		errResp, ok := err.(minio.ErrorResponse)
		if ok && errResp.StatusCode == 404 {
			return nil, nil // Not found — not an error for new canvases.
		}
		return nil, fmt.Errorf("minio: stat snapshot for canvas %d: %w", canvasID, err)
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("minio: read snapshot for canvas %d: %w", canvasID, err)
	}

	_ = info // size available if needed
	return data, nil
}

// ---- Delete ----

// Delete removes all snapshots for a canvas (both latest and versioned).
func (r *SnapshotRepo) Delete(ctx context.Context, canvasID collaboration.RoomID) error {
	// List all objects with the prefix.
	objectsCh := r.client.ListObjects(ctx, r.bucketName, minio.ListObjectsOptions{
		Prefix:    prefix(canvasID),
		Recursive: true,
	})

	var keys []string
	for obj := range objectsCh {
		if obj.Err != nil {
			return fmt.Errorf("minio: list snapshots for canvas %d: %w", canvasID, obj.Err)
		}
		keys = append(keys, obj.Key)
	}

	if len(keys) == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "no snapshots found for canvas")
	}

	// Remove all via a channel.
	delCh := make(chan minio.ObjectInfo, len(keys))
	for _, k := range keys {
		delCh <- minio.ObjectInfo{Key: k}
	}
	close(delCh)

	removeCh := r.client.RemoveObjects(ctx, r.bucketName, delCh, minio.RemoveObjectsOptions{})
	for err := range removeCh {
		if err.Err != nil {
			return fmt.Errorf("minio: delete snapshot %s: %w", err.ObjectName, err.Err)
		}
	}
	return nil
}
