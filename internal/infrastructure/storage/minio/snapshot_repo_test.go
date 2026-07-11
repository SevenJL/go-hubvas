package minio

import (
	"testing"
	"time"

	"github.com/hubvas/internal/domain/collaboration"
)

func TestSnapshotVersionUsesLastModifiedNanoseconds(t *testing.T) {
	modified := time.Date(2026, time.July, 12, 10, 30, 45, 123456789, time.FixedZone("CST", 8*60*60))
	got := snapshotVersion(modified)
	want := modified.UTC().UnixNano()
	if got != want {
		t.Fatalf("snapshotVersion() = %d, want %d", got, want)
	}
}

func TestVersionKeyDoesNotOverwriteEveryArchive(t *testing.T) {
	canvasID := collaboration.RoomID(42)
	first := versionKey(canvasID, snapshotVersion(time.Unix(100, 1)))
	second := versionKey(canvasID, snapshotVersion(time.Unix(101, 2)))
	if first == second {
		t.Fatalf("distinct snapshots produced the same archive key %q", first)
	}
	if first == versionKey(canvasID, 0) || second == versionKey(canvasID, 0) {
		t.Fatal("archive key must not use the old v0 placeholder")
	}
}
