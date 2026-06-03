package community

import (
	"testing"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
)

func TestNewLike(t *testing.T) {
	cid := canvas.CanvasID(1)
	uid := identity.UserID(10)

	like := NewLike(cid, uid)
	if like.CanvasID != cid {
		t.Fatalf("expected canvas id %d, got %d", cid, like.CanvasID)
	}
	if like.UserID != uid {
		t.Fatalf("expected user id %d, got %d", uid, like.UserID)
	}
	if like.CreatedAt.IsZero() {
		t.Fatal("expected non-zero created time")
	}
}

func TestNewFork(t *testing.T) {
	originalID := canvas.CanvasID(1)
	forkID := canvas.CanvasID(2)
	uid := identity.UserID(10)

	fork := NewFork(originalID, forkID, uid)
	if fork.OriginalCanvasID() != originalID {
		t.Fatalf("expected original id %d, got %d", originalID, fork.OriginalCanvasID())
	}
	if fork.NewCanvasID() != forkID {
		t.Fatalf("expected fork id %d, got %d", forkID, fork.NewCanvasID())
	}
	if fork.UserID() != uid {
		t.Fatalf("expected user id %d, got %d", uid, fork.UserID())
	}
}

func TestNewComment(t *testing.T) {
	cid := canvas.CanvasID(1)
	uid := identity.UserID(10)

	c, err := NewComment(CommentID(1), cid, uid, "Nice work!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.CanvasID() != cid {
		t.Fatalf("expected canvas id %d, got %d", cid, c.CanvasID())
	}
	if c.AuthorID() != uid {
		t.Fatalf("expected author id %d, got %d", uid, c.AuthorID())
	}
	if c.Content() != "Nice work!" {
		t.Fatalf("expected content 'Nice work!', got '%s'", c.Content())
	}
}

func TestNewComment_Empty(t *testing.T) {
	_, err := NewComment(1, 1, 1, "")
	if err == nil {
		t.Fatal("expected error for empty comment")
	}
}

func TestNewComment_TooLong(t *testing.T) {
	longContent := make([]byte, 5001)
	for i := range longContent {
		longContent[i] = 'x'
	}
	_, err := NewComment(1, 1, 1, string(longContent))
	if err == nil {
		t.Fatal("expected error for overly long comment")
	}
}

func TestComment_EditContent(t *testing.T) {
	c, _ := NewComment(1, 1, 10, "original")
	err := c.EditContent("updated")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Content() != "updated" {
		t.Fatalf("expected content 'updated', got '%s'", c.Content())
	}
}

func TestComment_EditEmpty(t *testing.T) {
	c, _ := NewComment(1, 1, 10, "original")
	err := c.EditContent("")
	if err == nil {
		t.Fatal("expected error for empty edit")
	}
}

func TestNewPublishedCanvas(t *testing.T) {
	cid := canvas.CanvasID(1)
	uid := identity.UserID(10)

	pc := NewPublishedCanvas(cid, uid, "My Art", "s3://snapshots/1/latest.bin", []string{"art", "sketch"})
	if pc.CanvasID() != cid {
		t.Fatalf("expected canvas id %d, got %d", cid, pc.CanvasID())
	}
	if pc.AuthorID() != uid {
		t.Fatalf("expected author id %d, got %d", uid, pc.AuthorID())
	}
	if pc.Title() != "My Art" {
		t.Fatalf("expected title 'My Art', got '%s'", pc.Title())
	}
	if pc.LikeCount() != 0 || pc.CommentCount() != 0 || pc.ForkCount() != 0 {
		t.Fatal("expected zero counts on new published canvas")
	}
	if len(pc.Tags()) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(pc.Tags()))
	}
}

func TestNewPublishedCanvas_NilTags(t *testing.T) {
	pc := NewPublishedCanvas(1, 10, "Title", "s3://k", nil)
	if pc.Tags() == nil || len(pc.Tags()) != 0 {
		t.Fatalf("expected empty tags slice, got %v", pc.Tags())
	}
}

func TestPublishedCanvas_CounterMutations(t *testing.T) {
	pc := NewPublishedCanvas(1, 10, "T", "s3://k", nil)

	pc.IncrementLike()
	pc.IncrementLike()
	if pc.LikeCount() != 2 {
		t.Fatalf("expected 2 likes, got %d", pc.LikeCount())
	}

	pc.DecrementLike()
	if pc.LikeCount() != 1 {
		t.Fatalf("expected 1 like after decrement, got %d", pc.LikeCount())
	}

	// Should not go below zero.
	pc.DecrementLike()
	pc.DecrementLike()
	if pc.LikeCount() != 0 {
		t.Fatalf("expected 0 likes, got %d", pc.LikeCount())
	}

	pc.IncrementComment()
	pc.IncrementComment()
	pc.IncrementComment()
	if pc.CommentCount() != 3 {
		t.Fatalf("expected 3 comments, got %d", pc.CommentCount())
	}

	pc.IncrementFork()
	if pc.ForkCount() != 1 {
		t.Fatalf("expected 1 fork, got %d", pc.ForkCount())
	}
}

func TestPublishedCanvas_UpdateTags(t *testing.T) {
	pc := NewPublishedCanvas(1, 10, "T", "s3://k", []string{"old"})
	pc.UpdateTags([]string{"new1", "new2"})
	if len(pc.Tags()) != 2 || pc.Tags()[0] != "new1" {
		t.Fatalf("expected tags [new1 new2], got %v", pc.Tags())
	}

	pc.UpdateTags(nil)
	if pc.Tags() == nil || len(pc.Tags()) != 0 {
		t.Fatalf("expected empty tags after nil update, got %v", pc.Tags())
	}
}

func TestPagination(t *testing.T) {
	p := Pagination{Page: 1, PageSize: 20}
	if p.Offset() != 0 {
		t.Fatalf("expected offset 0, got %d", p.Offset())
	}
	if p.Limit(20) != 20 {
		t.Fatalf("expected limit 20, got %d", p.Limit(20))
	}

	p2 := Pagination{Page: 3, PageSize: 10}
	if p2.Offset() != 20 {
		t.Fatalf("expected offset 20, got %d", p2.Offset())
	}

	// Zero page should default.
	p3 := Pagination{Page: 0, PageSize: 10}
	if p3.Offset() != 0 {
		t.Fatalf("expected offset 0 for page 0, got %d", p3.Offset())
	}

	// Over-limit should clamp.
	p4 := Pagination{Page: 1, PageSize: 200}
	if p4.Limit(20) != 20 {
		t.Fatalf("expected clamped limit 20, got %d", p4.Limit(20))
	}
}
