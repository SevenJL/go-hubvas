package ws

import (
	"bytes"
	"testing"
)

func TestMergePersistedSnapshotAccumulatesBinaryUpdates(t *testing.T) {
	first := []byte{1, 2, 3}
	second := []byte{4, 5}

	snapshot := mergePersistedSnapshot(nil, first)
	snapshot = mergePersistedSnapshot(snapshot, second)
	frames, err := decodeYjsSnapshot(snapshot)
	if err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(frames) != 2 || !bytes.Equal(frames[0], first) || !bytes.Equal(frames[1], second) {
		t.Fatalf("unexpected frames: %#v", frames)
	}
}

func TestMergePersistedSnapshotReplacesJSONState(t *testing.T) {
	snapshot := mergePersistedSnapshot(nil, []byte{1, 2, 3})
	fullState := []byte(`{"store":{"shape":1}}`)
	snapshot = mergePersistedSnapshot(snapshot, fullState)
	if !bytes.Equal(snapshot, fullState) {
		t.Fatalf("JSON full state should replace update log: %q", snapshot)
	}
}

func TestDecodeYjsSnapshotSupportsLegacyRawUpdate(t *testing.T) {
	legacy := []byte{9, 8, 7}
	frames, err := decodeYjsSnapshot(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 || !bytes.Equal(frames[0], legacy) {
		t.Fatalf("unexpected legacy frames: %#v", frames)
	}
}

func TestDecodeYjsSnapshotRejectsTruncatedFrame(t *testing.T) {
	broken := append(append([]byte{}, yjsSnapshotMagic...), 0, 0, 0, 5, 1)
	if _, err := decodeYjsSnapshot(broken); err == nil {
		t.Fatal("expected truncated frame error")
	}
}

func TestMergePersistedSnapshotTreatsJSONScalarAsBinaryUpdate(t *testing.T) {
	update := []byte(`1`)
	snapshot := mergePersistedSnapshot(nil, update)
	frames, err := decodeYjsSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 || !bytes.Equal(frames[0], update) {
		t.Fatalf("JSON-looking binary update was not preserved: %#v", frames)
	}
}
