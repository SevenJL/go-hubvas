package canvas

import (
	"testing"

	"github.com/hubvas/internal/domain/identity"
)

func TestNewCanvas(t *testing.T) {
	id := CanvasID(1)
	ownerID := identity.UserID(10)

	c, err := NewCanvas(id, ownerID, "My Canvas")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID() != id {
		t.Fatalf("expected id %d, got %d", id, c.ID())
	}
	if c.OwnerID() != ownerID {
		t.Fatalf("expected owner %d, got %d", ownerID, c.OwnerID())
	}
	if c.Title() != "My Canvas" {
		t.Fatalf("expected title 'My Canvas', got %s", c.Title())
	}
	if c.Visibility() != VisibilityPrivate {
		t.Fatalf("expected private visibility, got %d", c.Visibility())
	}
	if len(c.Members()) != 1 {
		t.Fatalf("expected 1 member (owner), got %d", len(c.Members()))
	}
	if c.GetRole(ownerID) != RoleOwner {
		t.Fatalf("expected owner role for owner, got %d", c.GetRole(ownerID))
	}
}

func TestNewCanvas_EmptyTitle(t *testing.T) {
	_, err := NewCanvas(1, 1, "")
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestNewCanvas_TooLongTitle(t *testing.T) {
	longTitle := make([]byte, 201)
	for i := range longTitle {
		longTitle[i] = 'a'
	}
	_, err := NewCanvas(1, 1, string(longTitle))
	if err == nil {
		t.Fatal("expected error for too-long title")
	}
}

func TestNewCanvas_TitleAtBoundary(t *testing.T) {
	maxTitle := make([]byte, 200)
	for i := range maxTitle {
		maxTitle[i] = 'a'
	}
	c, err := NewCanvas(1, 1, string(maxTitle))
	if err != nil {
		t.Fatalf("unexpected error for 200-char title: %v", err)
	}
	if c.Title() != string(maxTitle) {
		t.Fatal("title mismatch")
	}
}

func TestCanvas_AddMember(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Test")
	uid := identity.UserID(20)

	c.AddMember(uid, RoleEditor)
	if len(c.Members()) != 2 {
		t.Fatalf("expected 2 members, got %d", len(c.Members()))
	}
	if c.GetRole(uid) != RoleEditor {
		t.Fatalf("expected editor role, got %d", c.GetRole(uid))
	}
}

func TestCanvas_AddMemberDuplicate(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Test")
	uid := identity.UserID(20)

	c.AddMember(uid, RoleEditor)
	c.AddMember(uid, RoleViewer) // Update role

	if len(c.Members()) != 2 {
		t.Fatalf("expected still 2 members, got %d", len(c.Members()))
	}
	if c.GetRole(uid) != RoleViewer {
		t.Fatalf("expected role updated to viewer, got %d", c.GetRole(uid))
	}
}

func TestCanvas_RemoveMember(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Test")
	uid := identity.UserID(20)

	c.AddMember(uid, RoleEditor)
	err := c.RemoveMember(uid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Members()) != 1 {
		t.Fatalf("expected 1 member after removal, got %d", len(c.Members()))
	}
}

func TestCanvas_RemoveOwner(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Test")
	err := c.RemoveMember(c.OwnerID())
	if err == nil {
		t.Fatal("expected error when removing owner")
	}
}

func TestCanvas_RemoveUnknownMember(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Test")
	err := c.RemoveMember(identity.UserID(999))
	if err == nil {
		t.Fatal("expected error for unknown member")
	}
}

func TestCanvas_Publish(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Test")

	err := c.Publish()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.Visibility().IsPublished() {
		t.Fatal("expected published visibility")
	}
}

func TestCanvas_PublishAlreadyPublished(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Test")
	c.Publish()

	err := c.Publish()
	if err == nil {
		t.Fatal("expected error when publishing already-published canvas")
	}
}

func TestCanvas_ForkCanvas(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Original")
	c.UpdateSnapshotKey(SnapshotKey("snapshots/1/latest.bin"))

	uid := identity.UserID(20)
	fork, err := c.ForkCanvas(CanvasID(2), uid, "My Fork")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fork.ID() != 2 {
		t.Fatalf("expected fork id 2, got %d", fork.ID())
	}
	if fork.Title() != "My Fork" {
		t.Fatalf("expected title 'My Fork', got %s", fork.Title())
	}
	if fork.OwnerID() != uid {
		t.Fatalf("expected fork owner %d, got %d", uid, fork.OwnerID())
	}
	if fork.ForkedFrom() == nil || *fork.ForkedFrom() != 1 {
		t.Fatal("expected fork to reference original canvas")
	}
	// Snapshot key should be copied.
	if fork.SnapshotKey() != "snapshots/1/latest.bin" {
		t.Fatalf("expected snapshot key copied, got %s", fork.SnapshotKey())
	}
	if fork.GetRole(uid) != RoleOwner {
		t.Fatalf("expected fork owner to have owner role, got %d", fork.GetRole(uid))
	}
}

func TestCanvas_ForkDefaultTitle(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Masterpiece")
	fork, _ := c.ForkCanvas(2, 20, "")
	if fork.Title() != "Fork of Masterpiece" {
		t.Fatalf("expected default fork title, got %s", fork.Title())
	}
}

func TestCanvas_Rename(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Old Title")
	err := c.Rename("New Title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Title() != "New Title" {
		t.Fatalf("expected 'New Title', got %s", c.Title())
	}
}

func TestCanvas_RenameEmpty(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Title")
	err := c.Rename("")
	if err == nil {
		t.Fatal("expected error for empty rename")
	}
}

func TestCanvas_IsMember(t *testing.T) {
	c, _ := NewCanvas(1, 10, "Test")

	if !c.IsMember(identity.UserID(10)) {
		t.Fatal("expected owner to be a member")
	}
	if c.IsMember(identity.UserID(999)) {
		t.Fatal("expected non-member to not be a member")
	}
}

func TestVisibility_Helpers(t *testing.T) {
	if !VisibilityPublished.IsPublished() {
		t.Fatal("expected Published to be published")
	}
	if VisibilityPrivate.IsPublished() {
		t.Fatal("expected Private not to be published")
	}
}

func TestRole_CanEdit(t *testing.T) {
	if !RoleOwner.CanEdit() {
		t.Fatal("owner should be able to edit")
	}
	if !RoleEditor.CanEdit() {
		t.Fatal("editor should be able to edit")
	}
	if RoleViewer.CanEdit() {
		t.Fatal("viewer should not be able to edit")
	}
}

func TestRole_CanComment(t *testing.T) {
	if !RoleOwner.CanComment() {
		t.Fatal("owner should be able to comment")
	}
	if !RoleEditor.CanComment() {
		t.Fatal("editor should be able to comment")
	}
	if !RoleCommenter.CanComment() {
		t.Fatal("commenter should be able to comment")
	}
	if RoleViewer.CanComment() {
		t.Fatal("viewer should not be able to comment")
	}
}
