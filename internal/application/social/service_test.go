package social

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

type socialRepoStub struct {
	followCalls int
	report      ReportRequest
	status      string
}

func (*socialRepoStub) PublicProfile(context.Context, string, identity.UserID) (*PublicProfileDTO, error) {
	return nil, nil
}
func (*socialRepoStub) PublishedByUser(context.Context, string, identity.UserID, int, int) (*PublishedPage, error) {
	return nil, nil
}
func (*socialRepoStub) FollowingFeed(context.Context, identity.UserID, int, int) (*PublishedPage, error) {
	return nil, nil
}
func (r *socialRepoStub) Follow(context.Context, identity.UserID, identity.UserID) error {
	r.followCalls++
	return nil
}
func (*socialRepoStub) Unfollow(context.Context, identity.UserID, identity.UserID) error { return nil }
func (*socialRepoStub) Block(context.Context, identity.UserID, identity.UserID) error    { return nil }
func (*socialRepoStub) Unblock(context.Context, identity.UserID, identity.UserID) error  { return nil }
func (*socialRepoStub) Relationships(context.Context, identity.UserID, identity.UserID, string, int, int) (*RelationshipPage, error) {
	return &RelationshipPage{}, nil
}
func (*socialRepoStub) Blocks(context.Context, identity.UserID, int, int) (*RelationshipPage, error) {
	return nil, nil
}
func (*socialRepoStub) Notifications(context.Context, identity.UserID, int, int) (*NotificationPage, error) {
	return nil, nil
}
func (*socialRepoStub) UnreadCount(context.Context, identity.UserID) (int64, error) { return 0, nil }
func (*socialRepoStub) MarkRead(context.Context, identity.UserID, int64) error      { return nil }
func (*socialRepoStub) MarkAllRead(context.Context, identity.UserID) error          { return nil }
func (r *socialRepoStub) CreateReport(_ context.Context, _ identity.UserID, request ReportRequest) (*ReportDTO, error) {
	r.report = request
	return &ReportDTO{}, nil
}
func (*socialRepoStub) Reports(context.Context, string, int, int) ([]ReportDTO, int64, error) {
	return nil, 0, nil
}
func (*socialRepoStub) ReviewReport(context.Context, identity.UserID, int64, ReviewReportRequest) (*ReportDTO, error) {
	return &ReportDTO{}, nil
}
func (r *socialRepoStub) SetUserStatus(_ context.Context, _ identity.UserID, status string) error {
	r.status = status
	return nil
}
func (*socialRepoStub) ModerateComment(context.Context, int64, string) error { return nil }
func (*socialRepoStub) ModerateCanvas(context.Context, int64, string) error  { return nil }

type userRepoStub struct {
	users map[identity.UserID]*identity.User
}

func (*userRepoStub) Save(context.Context, *identity.User) error { return nil }
func (r *userRepoStub) FindByID(_ context.Context, id identity.UserID) (*identity.User, error) {
	if user := r.users[id]; user != nil {
		return user, nil
	}
	return nil, shared.NewDomainError(shared.ErrNotFound, "user not found")
}
func (*userRepoStub) FindByUsername(context.Context, string) (*identity.User, error) { return nil, nil }
func (*userRepoStub) FindByEmail(context.Context, string) (*identity.User, error)    { return nil, nil }
func (*userRepoStub) ExistsByUsername(context.Context, string) (bool, error)         { return false, nil }
func (*userRepoStub) ExistsByEmail(context.Context, string) (bool, error)            { return false, nil }
func (*userRepoStub) Delete(context.Context, identity.UserID) error                  { return nil }

func socialUser(id identity.UserID, role string) *identity.User {
	return identity.ReconstituteUserProfile(id, "user", "user@example.com", "hash", "User", "", "", "", "", 0, role, "active", time.Now(), time.Now())
}

func TestFollowAndBlockRejectSelf(t *testing.T) {
	repo := &socialRepoStub{}
	service := NewService(repo, &userRepoStub{})
	if err := service.Follow(context.Background(), 4, 4); !errors.Is(err, shared.ErrInvalidArgument) {
		t.Fatalf("expected self-follow error, got %v", err)
	}
	if err := service.Block(context.Background(), 4, 4); !errors.Is(err, shared.ErrInvalidArgument) {
		t.Fatalf("expected self-block error, got %v", err)
	}
	if repo.followCalls != 0 {
		t.Fatal("repository should not be called for self-follow")
	}
}

func TestReportValidationAndNormalization(t *testing.T) {
	repo := &socialRepoStub{}
	service := NewService(repo, &userRepoStub{})
	if _, err := service.CreateReport(context.Background(), 1, ReportRequest{TargetType: "unknown", TargetID: 2, Reason: "spam"}); !errors.Is(err, shared.ErrInvalidArgument) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if _, err := service.CreateReport(context.Background(), 1, ReportRequest{TargetType: " user ", TargetID: 2, Reason: " other ", Details: " detail "}); err != nil {
		t.Fatal(err)
	}
	if repo.report.TargetType != "user" || repo.report.Reason != "other" || repo.report.Details != "detail" {
		t.Fatalf("report was not normalized: %#v", repo.report)
	}
	if _, err := service.CreateReport(context.Background(), 2, ReportRequest{TargetType: "user", TargetID: 2, Reason: "spam"}); !errors.Is(err, shared.ErrInvalidArgument) {
		t.Fatalf("expected self-report error, got %v", err)
	}
}

func TestAdminAuthorizationAndModerationValidation(t *testing.T) {
	repo := &socialRepoStub{}
	users := &userRepoStub{users: map[identity.UserID]*identity.User{1: socialUser(1, "user"), 2: socialUser(2, "admin")}}
	service := NewService(repo, users)
	if err := service.SetUserStatus(context.Background(), 1, 3, "suspended"); !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("expected admin authorization error, got %v", err)
	}
	if err := service.SetUserStatus(context.Background(), 2, 3, "disabled"); !errors.Is(err, shared.ErrInvalidArgument) {
		t.Fatalf("expected status validation error, got %v", err)
	}
	if err := service.SetUserStatus(context.Background(), 2, 3, "suspended"); err != nil {
		t.Fatal(err)
	}
	if repo.status != "suspended" {
		t.Fatalf("unexpected stored status %q", repo.status)
	}
	if err := service.ModerateComment(context.Background(), 2, 1, "removed"); !errors.Is(err, shared.ErrInvalidArgument) {
		t.Fatalf("expected moderation validation error, got %v", err)
	}
}
