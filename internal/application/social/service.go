package social

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

type Repository interface {
	PublicProfile(context.Context, string, identity.UserID) (*PublicProfileDTO, error)
	PublishedByUser(context.Context, string, identity.UserID, int, int) (*PublishedPage, error)
	FollowingFeed(context.Context, identity.UserID, int, int) (*PublishedPage, error)
	Follow(context.Context, identity.UserID, identity.UserID) error
	Unfollow(context.Context, identity.UserID, identity.UserID) error
	Block(context.Context, identity.UserID, identity.UserID) error
	Unblock(context.Context, identity.UserID, identity.UserID) error
	Relationships(context.Context, identity.UserID, identity.UserID, string, int, int) (*RelationshipPage, error)
	Blocks(context.Context, identity.UserID, int, int) (*RelationshipPage, error)
	Notifications(context.Context, identity.UserID, int, int) (*NotificationPage, error)
	UnreadCount(context.Context, identity.UserID) (int64, error)
	MarkRead(context.Context, identity.UserID, int64) error
	MarkAllRead(context.Context, identity.UserID) error
	CreateReport(context.Context, identity.UserID, ReportRequest) (*ReportDTO, error)
	Reports(context.Context, string, int, int) ([]ReportDTO, int64, error)
	ReviewReport(context.Context, identity.UserID, int64, ReviewReportRequest) (*ReportDTO, error)
	SetUserStatus(context.Context, identity.UserID, identity.UserID, string) error
	ModerateComment(context.Context, identity.UserID, int64, string) error
	ModerateCanvas(context.Context, identity.UserID, int64, string) error
	AuditLogs(context.Context, int, int) ([]AdminAuditLogDTO, int64, error)
	ReplayNotificationOutbox(context.Context, identity.UserID, int) (int64, error)
}
type Service struct {
	repo  Repository
	users identity.UserRepository
}

func NewService(r Repository, u identity.UserRepository) *Service { return &Service{repo: r, users: u} }
func (s *Service) Profile(ctx context.Context, username string, viewer identity.UserID) (*PublicProfileDTO, error) {
	return s.repo.PublicProfile(ctx, username, viewer)
}
func (s *Service) PublishedByUser(ctx context.Context, username string, viewer identity.UserID, page, size int) (*PublishedPage, error) {
	page, size = normalize(page, size)
	return s.repo.PublishedByUser(ctx, username, viewer, page, size)
}
func (s *Service) FollowingFeed(ctx context.Context, viewer identity.UserID, page, size int) (*PublishedPage, error) {
	page, size = normalize(page, size)
	return s.repo.FollowingFeed(ctx, viewer, page, size)
}
func (s *Service) Follow(ctx context.Context, actor, target identity.UserID) error {
	if actor == target {
		return shared.NewDomainError(shared.ErrInvalidArgument, "cannot follow yourself")
	}
	return s.repo.Follow(ctx, actor, target)
}
func (s *Service) Unfollow(ctx context.Context, actor, target identity.UserID) error {
	return s.repo.Unfollow(ctx, actor, target)
}
func (s *Service) Block(ctx context.Context, actor, target identity.UserID) error {
	if actor == target {
		return shared.NewDomainError(shared.ErrInvalidArgument, "cannot block yourself")
	}
	return s.repo.Block(ctx, actor, target)
}
func (s *Service) Unblock(ctx context.Context, actor, target identity.UserID) error {
	return s.repo.Unblock(ctx, actor, target)
}
func (s *Service) Relationships(ctx context.Context, viewer, user identity.UserID, kind string, page, size int) (*RelationshipPage, error) {
	if kind != "followers" && kind != "following" {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "relationship kind must be followers or following")
	}
	page, size = normalize(page, size)
	return s.repo.Relationships(ctx, viewer, user, kind, page, size)
}
func (s *Service) Blocks(ctx context.Context, user identity.UserID, page, size int) (*RelationshipPage, error) {
	page, size = normalize(page, size)
	return s.repo.Blocks(ctx, user, page, size)
}
func (s *Service) Notifications(ctx context.Context, user identity.UserID, page, size int) (*NotificationPage, error) {
	page, size = normalize(page, size)
	return s.repo.Notifications(ctx, user, page, size)
}
func (s *Service) UnreadCount(ctx context.Context, user identity.UserID) (int64, error) {
	return s.repo.UnreadCount(ctx, user)
}
func (s *Service) MarkRead(ctx context.Context, user identity.UserID, id int64) error {
	return s.repo.MarkRead(ctx, user, id)
}
func (s *Service) MarkAllRead(ctx context.Context, user identity.UserID) error {
	return s.repo.MarkAllRead(ctx, user)
}
func (s *Service) CreateReport(ctx context.Context, user identity.UserID, r ReportRequest) (*ReportDTO, error) {
	r.TargetType = strings.TrimSpace(r.TargetType)
	r.Reason = strings.TrimSpace(r.Reason)
	r.Details = strings.TrimSpace(r.Details)
	if r.TargetID <= 0 || !oneOf(r.TargetType, "user", "canvas", "comment") || !oneOf(r.Reason, "spam", "harassment", "inappropriate", "copyright", "other") {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "invalid report target or reason")
	}
	if r.TargetType == "user" && identity.UserID(r.TargetID) == user {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "cannot report yourself")
	}
	if utf8.RuneCountInString(r.Details) > 1000 {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "report details must not exceed 1000 characters")
	}
	return s.repo.CreateReport(ctx, user, r)
}
func (s *Service) Reports(ctx context.Context, admin identity.UserID, status string, page, size int) ([]ReportDTO, int64, error) {
	if status != "" && !oneOf(status, "pending", "reviewing", "resolved", "dismissed") {
		return nil, 0, shared.NewDomainError(shared.ErrInvalidArgument, "invalid report status")
	}
	if err := s.requireAdmin(ctx, admin); err != nil {
		return nil, 0, err
	}
	page, size = normalize(page, size)
	return s.repo.Reports(ctx, status, page, size)
}
func (s *Service) ReviewReport(ctx context.Context, admin identity.UserID, id int64, r ReviewReportRequest) (*ReportDTO, error) {
	r.Note = strings.TrimSpace(r.Note)
	if id <= 0 || !oneOf(r.Status, "reviewing", "resolved", "dismissed") || utf8.RuneCountInString(r.Note) > 1000 {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "invalid report review")
	}
	if err := s.requireAdmin(ctx, admin); err != nil {
		return nil, err
	}
	return s.repo.ReviewReport(ctx, admin, id, r)
}
func (s *Service) SetUserStatus(ctx context.Context, admin, target identity.UserID, status string) error {
	if target <= 0 || !oneOf(status, "active", "suspended") {
		return shared.NewDomainError(shared.ErrInvalidArgument, "invalid user status")
	}
	if err := s.requireAdmin(ctx, admin); err != nil {
		return err
	}
	return s.repo.SetUserStatus(ctx, admin, target, status)
}
func (s *Service) ModerateComment(ctx context.Context, admin identity.UserID, id int64, status string) error {
	if id <= 0 || !oneOf(status, "visible", "hidden") {
		return shared.NewDomainError(shared.ErrInvalidArgument, "invalid comment moderation status")
	}
	if err := s.requireAdmin(ctx, admin); err != nil {
		return err
	}
	return s.repo.ModerateComment(ctx, admin, id, status)
}
func (s *Service) ModerateCanvas(ctx context.Context, admin identity.UserID, id int64, status string) error {
	if id <= 0 || !oneOf(status, "visible", "hidden") {
		return shared.NewDomainError(shared.ErrInvalidArgument, "invalid canvas moderation status")
	}
	if err := s.requireAdmin(ctx, admin); err != nil {
		return err
	}
	return s.repo.ModerateCanvas(ctx, admin, id, status)
}

func (s *Service) AuditLogs(ctx context.Context, admin identity.UserID, page, size int) ([]AdminAuditLogDTO, int64, error) {
	if err := s.requireAdmin(ctx, admin); err != nil {
		return nil, 0, err
	}
	page, size = normalize(page, size)
	return s.repo.AuditLogs(ctx, page, size)
}
func (s *Service) ReplayNotificationOutbox(ctx context.Context, admin identity.UserID, limit int) (int64, error) {
	if err := s.requireAdmin(ctx, admin); err != nil {
		return 0, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		return 0, shared.NewDomainError(shared.ErrInvalidArgument, "replay limit must not exceed 1000")
	}
	return s.repo.ReplayNotificationOutbox(ctx, admin, limit)
}

func (s *Service) requireAdmin(ctx context.Context, id identity.UserID) error {
	u, err := s.users.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !u.IsAdmin() {
		return shared.NewDomainError(shared.ErrForbidden, "administrator access required")
	}
	return nil
}
func normalize(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
