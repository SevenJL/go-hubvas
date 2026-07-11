package ws

import (
	"encoding/json"
	"testing"

	"github.com/hubvas/internal/domain/identity"
)

func TestCanSubmitOperation(t *testing.T) {
	if canSubmitOperation(false, MsgTypeSync) {
		t.Fatal("read-only clients must not be allowed to submit sync operations")
	}
	if !canSubmitOperation(true, MsgTypeSync) {
		t.Fatal("editors should be allowed to submit sync operations")
	}
	if canSubmitOperation(false, MsgTypeLock) || canSubmitOperation(false, MsgTypeUnlock) {
		t.Fatal("read-only clients must not be allowed to manage object locks")
	}
	if !canSubmitOperation(false, MsgTypeAwareness) {
		t.Fatal("read-only clients should still be allowed to submit awareness updates")
	}
	if !isServerOnlyMessage(MsgTypeLockState) || isServerOnlyMessage(MsgTypeLock) {
		t.Fatal("lock state must be server-only while lock requests remain client-submittable")
	}
}

func TestAuthoritativePayloadOverridesClientIdentity(t *testing.T) {
	payload, err := authoritativePayload(
		json.RawMessage(`{"user_id":999,"username":"spoofed","cursor":{"x":1,"y":2}}`),
		identity.UserID(42),
		"alice",
	)
	if err != nil {
		t.Fatalf("authoritativePayload returned error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got["user_id"] != float64(42) || got["username"] != "alice" {
		t.Fatalf("client identity was not overridden: %#v", got)
	}
}
