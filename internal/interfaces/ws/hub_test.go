package ws

import (
	"context"
	"testing"

	"github.com/hubvas/internal/domain/collaboration"
)

type snapshotRepositoryStub struct {
	data []byte
}

func (r *snapshotRepositoryStub) Save(context.Context, collaboration.RoomID, []byte) error {
	return nil
}
func (r *snapshotRepositoryStub) Load(context.Context, collaboration.RoomID) ([]byte, error) {
	return r.data, nil
}
func (r *snapshotRepositoryStub) Delete(context.Context, collaboration.RoomID) error { return nil }

func TestHubLoadsPersistedSnapshotWhenRoomIsCreated(t *testing.T) {
	repo := &snapshotRepositoryStub{data: []byte(`{"store":{"shape":1}}`)}
	hub := NewHub(repo)
	defer hub.Shutdown()

	room := hub.GetOrCreate(collaboration.RoomID(7))
	if string(room.Snapshot()) != string(repo.data) {
		t.Fatalf("expected persisted snapshot %q, got %q", repo.data, room.Snapshot())
	}
}
