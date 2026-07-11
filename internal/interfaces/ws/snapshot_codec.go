package ws

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
)

var yjsSnapshotMagic = []byte{'H', 'U', 'B', 'Y', 'J', 'S', '1', 0}

// mergePersistedSnapshot replaces full JSON snapshots, while binary Yjs
// updates are accumulated as length-prefixed frames so a cold client can
// replay every incremental update in order.
func mergePersistedSnapshot(current, update []byte) []byte {
	if len(update) == 0 {
		return cloneBytes(current)
	}
	if isJSONSnapshot(update) {
		return cloneBytes(update)
	}

	// The normal hot path is an already-framed update log. Append one frame
	// directly instead of decoding and re-encoding the complete history.
	if bytes.HasPrefix(current, yjsSnapshotMagic) {
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(update)))
		current = append(current, length[:]...)
		current = append(current, update...)
		return current
	}

	// Empty, legacy raw, and JSON states are converted once.
	frames := make([][]byte, 0, 2)
	if len(current) > 0 && !isJSONSnapshot(current) {
		frames = append(frames, current)
	}
	frames = append(frames, update)
	return encodeYjsSnapshot(frames)
}

func encodeYjsSnapshot(frames [][]byte) []byte {
	size := len(yjsSnapshotMagic)
	for _, frame := range frames {
		size += 4 + len(frame)
	}
	result := make([]byte, 0, size)
	result = append(result, yjsSnapshotMagic...)
	var length [4]byte
	for _, frame := range frames {
		binary.BigEndian.PutUint32(length[:], uint32(len(frame)))
		result = append(result, length[:]...)
		result = append(result, frame...)
	}
	return result
}

// decodeYjsSnapshot accepts both the framed format and legacy raw binary
// snapshots. JSON snapshots are rejected because they use the text protocol.
func decodeYjsSnapshot(snapshot []byte) ([][]byte, error) {
	if len(snapshot) == 0 {
		return nil, nil
	}
	if isJSONSnapshot(snapshot) {
		return nil, errors.New("text snapshot is not a Yjs update log")
	}
	if !bytes.HasPrefix(snapshot, yjsSnapshotMagic) {
		return [][]byte{cloneBytes(snapshot)}, nil
	}

	remaining := snapshot[len(yjsSnapshotMagic):]
	frames := make([][]byte, 0)
	for len(remaining) > 0 {
		if len(remaining) < 4 {
			return nil, errors.New("truncated Yjs snapshot frame length")
		}
		length := int(binary.BigEndian.Uint32(remaining[:4]))
		remaining = remaining[4:]
		if length > len(remaining) {
			return nil, errors.New("truncated Yjs snapshot frame")
		}
		frames = append(frames, cloneBytes(remaining[:length]))
		remaining = remaining[length:]
	}
	return frames, nil
}

func isJSONSnapshot(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && trimmed[0] == '{' && json.Valid(trimmed)
}

// isRealtimeTldrawDiff identifies the browser's animation-frame diff protocol.
// These messages are ephemeral live updates; durable full snapshots continue to
// arrive separately and must remain the only JSON state stored for cold replay.
func isRealtimeTldrawDiff(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return bytes.HasPrefix(trimmed, []byte(`{"kind":"tldraw-diff-v1","diffs":`))
}
