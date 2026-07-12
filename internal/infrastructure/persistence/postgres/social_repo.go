package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	appsocial "github.com/hubvas/internal/application/social"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type SocialRepo struct{ pool *pgxpool.Pool }

func NewSocialRepo(p *pgxpool.Pool) *SocialRepo { return &SocialRepo{pool: p} }
func (r *SocialRepo) PublicProfile(ctx context.Context, username string, viewer identity.UserID) (*appsocial.PublicProfileDTO, error) {
	var p appsocial.PublicProfileDTO
	err := r.pool.QueryRow(ctx, `SELECT u.id,u.username,u.display_name,COALESCE(u.avatar_url,''),u.bio,u.website,u.created_at,
 (SELECT COUNT(*) FROM published_canvases pc WHERE pc.author_id=u.id AND pc.moderation_status='visible'),
 (SELECT COUNT(*) FROM follows f JOIN users rel ON rel.id=f.follower_id WHERE f.followed_id=u.id AND rel.status='active' AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=$2 AND b.blocked_id=rel.id) OR (b.blocker_id=rel.id AND b.blocked_id=$2))),
 (SELECT COUNT(*) FROM follows f JOIN users rel ON rel.id=f.followed_id WHERE f.follower_id=u.id AND rel.status='active' AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=$2 AND b.blocked_id=rel.id) OR (b.blocker_id=rel.id AND b.blocked_id=$2))),
 EXISTS(SELECT 1 FROM follows WHERE follower_id=$2 AND followed_id=u.id),
 EXISTS(SELECT 1 FROM blocks WHERE blocker_id=$2 AND blocked_id=u.id),
 EXISTS(SELECT 1 FROM blocks WHERE blocker_id=u.id AND blocked_id=$2)
 FROM users u WHERE LOWER(u.username)=LOWER($1) AND u.status='active'`, username, viewer).Scan(&p.ID, &p.Username, &p.DisplayName, &p.AvatarURL, &p.Bio, &p.Website, &p.JoinedAt, &p.PublishedCount, &p.FollowersCount, &p.FollowingCount, &p.IsFollowing, &p.IsBlocked, &p.IsBlockedBy)
	if err != nil {
		return nil, mapPgError(err)
	}
	return &p, nil
}
func (r *SocialRepo) Follow(ctx context.Context, actor, target identity.UserID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var targetActive, blocked bool
	if err = tx.QueryRow(ctx, `SELECT status='active' FROM users WHERE id=$1`, target).Scan(&targetActive); err != nil {
		return mapPgError(err)
	}
	if !targetActive {
		return shared.NewDomainError(shared.ErrForbidden, "cannot follow a suspended user")
	}
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))`, actor, target).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return shared.NewDomainError(shared.ErrForbidden, "follow is not allowed because a block exists")
	}
	tag, err := tx.Exec(ctx, `INSERT INTO follows(follower_id,followed_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, actor, target)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() > 0 {
		if err = createNotificationTx(ctx, tx, target, actor, "follow", "user", int64(actor), fmt.Sprintf("follow:%d", actor), map[string]any{}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (r *SocialRepo) Unfollow(ctx context.Context, a, t identity.UserID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM follows WHERE follower_id=$1 AND followed_id=$2`, a, t)
	return mapPgError(err)
}
func (r *SocialRepo) Block(ctx context.Context, a, t identity.UserID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO blocks(blocker_id,blocked_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, a, t); err != nil {
		return mapPgError(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM follows WHERE (follower_id=$1 AND followed_id=$2) OR (follower_id=$2 AND followed_id=$1)`, a, t); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *SocialRepo) Unblock(ctx context.Context, a, t identity.UserID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM blocks WHERE blocker_id=$1 AND blocked_id=$2`, a, t)
	return mapPgError(err)
}
func (r *SocialRepo) Relationships(ctx context.Context, viewer, user identity.UserID, kind string, page, size int) (*appsocial.RelationshipPage, error) {
	col, join := "f.followed_id", "f.follower_id"
	if kind == "following" {
		col, join = "f.follower_id", "f.followed_id"
	}
	visibility := fmt.Sprintf(`NOT EXISTS (
		SELECT 1 FROM blocks b
		WHERE (b.blocker_id=$2 AND b.blocked_id=u.id)
		   OR (b.blocker_id=u.id AND b.blocked_id=$2)
	)`)
	q := fmt.Sprintf(`SELECT u.id,u.username,u.display_name,COALESCE(u.avatar_url,'')
		FROM follows f JOIN users u ON u.id=%s
		WHERE %s=$1 AND u.status='active' AND %s
		ORDER BY f.created_at DESC LIMIT $3 OFFSET $4`, join, col, visibility)
	rows, err := r.pool.Query(ctx, q, user, viewer, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []appsocial.ActorDTO{}
	for rows.Next() {
		var a appsocial.ActorDTO
		if err = rows.Scan(&a.ID, &a.Username, &a.DisplayName, &a.AvatarURL); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM follows f JOIN users u ON u.id=%s WHERE %s=$1 AND u.status='active' AND %s`, join, col, visibility)
	var total int64
	err = r.pool.QueryRow(ctx, countQ, user, viewer).Scan(&total)
	return &appsocial.RelationshipPage{Items: items, Total: total, Page: page, PageSize: size}, err
}

func (r *SocialRepo) Blocks(ctx context.Context, user identity.UserID, page, size int) (*appsocial.RelationshipPage, error) {
	rows, err := r.pool.Query(ctx, `SELECT u.id,u.username,u.display_name,COALESCE(u.avatar_url,'') FROM blocks b JOIN users u ON u.id=b.blocked_id WHERE b.blocker_id=$1 ORDER BY b.created_at DESC LIMIT $2 OFFSET $3`, user, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []appsocial.ActorDTO{}
	for rows.Next() {
		var a appsocial.ActorDTO
		if err = rows.Scan(&a.ID, &a.Username, &a.DisplayName, &a.AvatarURL); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	var total int64
	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM blocks WHERE blocker_id=$1`, user).Scan(&total)
	return &appsocial.RelationshipPage{Items: items, Total: total, Page: page, PageSize: size}, err
}
func (r *SocialRepo) Notifications(ctx context.Context, user identity.UserID, page, size int) (*appsocial.NotificationPage, error) {
	rows, err := r.pool.Query(ctx, `SELECT n.id,n.event_type,n.actor_id,u.username,u.display_name,COALESCE(u.avatar_url,''),n.target_type,n.target_id,n.data,n.read_at,n.created_at FROM notifications n LEFT JOIN users u ON u.id=n.actor_id WHERE n.recipient_id=$1 ORDER BY n.created_at DESC LIMIT $2 OFFSET $3`, user, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []appsocial.NotificationDTO{}
	for rows.Next() {
		var n appsocial.NotificationDTO
		var actorID *int64
		var username, display, avatar *string
		var raw []byte
		if err = rows.Scan(&n.ID, &n.EventType, &actorID, &username, &display, &avatar, &n.TargetType, &n.TargetID, &raw, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &n.Data)
		if actorID != nil {
			n.Actor = &appsocial.ActorDTO{ID: *actorID, Username: derefString(username), DisplayName: derefString(display), AvatarURL: derefString(avatar)}
		}
		items = append(items, n)
	}
	var total int64
	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE recipient_id=$1`, user).Scan(&total)
	return &appsocial.NotificationPage{Items: items, Total: total, Page: page, PageSize: size}, err
}
func (r *SocialRepo) UnreadCount(ctx context.Context, u identity.UserID) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE recipient_id=$1 AND read_at IS NULL`, u).Scan(&n)
	return n, err
}
func (r *SocialRepo) MarkRead(ctx context.Context, u identity.UserID, id int64) error {
	tag, err := r.pool.Exec(ctx, `UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE id=$1 AND recipient_id=$2`, id, u)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "notification not found")
	}
	return nil
}
func (r *SocialRepo) MarkAllRead(ctx context.Context, u identity.UserID) error {
	_, err := r.pool.Exec(ctx, `UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE recipient_id=$1 AND read_at IS NULL`, u)
	return err
}
func (r *SocialRepo) CreateReport(ctx context.Context, u identity.UserID, in appsocial.ReportRequest) (*appsocial.ReportDTO, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	switch in.TargetType {
	case "user":
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active')`, in.TargetID).Scan(&exists)
	case "canvas":
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM published_canvases WHERE canvas_id=$1 AND moderation_status='visible')`, in.TargetID).Scan(&exists)
	case "comment":
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM comments WHERE id=$1 AND deleted_at IS NULL AND moderation_status='visible')`, in.TargetID).Scan(&exists)
	default:
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "unsupported report target")
	}
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, shared.NewDomainError(shared.ErrNotFound, "report target not found")
	}

	report, err := scanReport(tx.QueryRow(ctx, `INSERT INTO reports(reporter_id,target_type,target_id,reason,details) VALUES($1,$2,$3,$4,$5) RETURNING id,reporter_id,target_type,target_id,reason,details,status,reviewer_id,review_note,created_at,reviewed_at`, u, in.TargetType, in.TargetID, in.Reason, in.Details))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return report, nil
}
func (r *SocialRepo) Reports(ctx context.Context, status string, page, size int) ([]appsocial.ReportDTO, int64, error) {
	where := ""
	args := []any{size, (page - 1) * size}
	if status != "" {
		where = " WHERE status=$3"
		args = append(args, status)
	}
	rows, err := r.pool.Query(ctx, `SELECT id,reporter_id,target_type,target_id,reason,details,status,reviewer_id,review_note,created_at,reviewed_at FROM reports`+where+` ORDER BY created_at ASC LIMIT $1 OFFSET $2`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []appsocial.ReportDTO{}
	for rows.Next() {
		v, err := scanReport(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *v)
	}
	var total int64
	if status == "" {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports`).Scan(&total)
	} else {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports WHERE status=$1`, status).Scan(&total)
	}
	return out, total, err
}
func (r *SocialRepo) ReviewReport(ctx context.Context, admin identity.UserID, id int64, in appsocial.ReviewReportRequest) (*appsocial.ReportDTO, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	item, err := scanReport(tx.QueryRow(ctx, `UPDATE reports SET status=$1::varchar,reviewer_id=$2,review_note=$3,reviewed_at=CASE WHEN $1::text IN ('resolved','dismissed') THEN now() ELSE reviewed_at END WHERE id=$4 RETURNING id,reporter_id,target_type,target_id,reason,details,status,reviewer_id,review_note,created_at,reviewed_at`, in.Status, admin, in.Note, id))
	if err != nil {
		return nil, err
	}
	if err = insertAdminAudit(ctx, tx, admin, "report.review", "report", id, map[string]any{"status": in.Status, "note": in.Note}); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return item, nil
}
func (r *SocialRepo) SetUserStatus(ctx context.Context, admin, id identity.UserID, status string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET status=$1,updated_at=now() WHERE id=$2`, status, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "user not found")
	}
	if err = insertAdminAudit(ctx, tx, admin, "user.status.update", "user", int64(id), map[string]any{"status": status}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *SocialRepo) ModerateComment(ctx context.Context, admin identity.UserID, id int64, status string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE comments SET moderation_status=$1 WHERE id=$2`, status, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "comment not found")
	}
	if err = insertAdminAudit(ctx, tx, admin, "comment.moderate", "comment", id, map[string]any{"status": status}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *SocialRepo) ModerateCanvas(ctx context.Context, admin identity.UserID, id int64, status string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE published_canvases SET moderation_status=$1 WHERE canvas_id=$2`, status, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "published canvas not found")
	}
	if err = insertAdminAudit(ctx, tx, admin, "canvas.moderate", "canvas", id, map[string]any{"status": status}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertAdminAudit(ctx context.Context, tx pgx.Tx, admin identity.UserID, action, targetType string, targetID int64, metadata map[string]any) error {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO admin_audit_logs(admin_id,action,target_type,target_id,metadata) VALUES($1,$2,$3,$4,$5)`, admin, action, targetType, targetID, raw)
	return err
}

func (r *SocialRepo) AuditLogs(ctx context.Context, page, size int) ([]appsocial.AdminAuditLogDTO, int64, error) {
	offset := (page - 1) * size
	rows, err := r.pool.Query(ctx, `SELECT l.id,l.admin_id,COALESCE(u.username,''),COALESCE(u.display_name,''),COALESCE(u.avatar_url,''),l.action,l.target_type,l.target_id,l.metadata,l.created_at FROM admin_audit_logs l LEFT JOIN users u ON u.id=l.admin_id ORDER BY l.created_at DESC,l.id DESC LIMIT $1 OFFSET $2`, size, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]appsocial.AdminAuditLogDTO, 0, size)
	for rows.Next() {
		var item appsocial.AdminAuditLogDTO
		var raw []byte
		if err = rows.Scan(&item.ID, &item.AdminID, &item.AdminUsername, &item.AdminDisplayName, &item.AdminAvatarURL, &item.Action, &item.TargetType, &item.TargetID, &raw, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if err = json.Unmarshal(raw, &item.Metadata); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int64
	if err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM admin_audit_logs`).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *SocialRepo) ReplayNotificationOutbox(ctx context.Context, admin identity.UserID, limit int) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE notification_outbox SET dead_lettered_at=NULL,attempts=0,last_error=NULL,available_at=now(),lease_owner=NULL,leased_until=NULL WHERE id IN (SELECT id FROM notification_outbox WHERE dead_lettered_at IS NOT NULL ORDER BY id LIMIT $1)`, limit)
	if err != nil {
		return 0, err
	}
	count := tag.RowsAffected()
	if err = insertAdminAudit(ctx, tx, admin, "notification_outbox.replay", "notification_outbox", 0, map[string]any{"count": count, "limit": limit}); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

type scanner interface{ Scan(...any) error }

func scanReport(row scanner) (*appsocial.ReportDTO, error) {
	var v appsocial.ReportDTO
	err := row.Scan(&v.ID, &v.ReporterID, &v.TargetType, &v.TargetID, &v.Reason, &v.Details, &v.Status, &v.ReviewerID, &v.ReviewNote, &v.CreatedAt, &v.ReviewedAt)
	if err != nil {
		return nil, mapPgError(err)
	}
	return &v, nil
}
func createNotificationTx(ctx context.Context, tx pgx.Tx, recipient, actor identity.UserID, event, targetType string, targetID int64, dedupe string, data map[string]any) error {
	if recipient == actor {
		return nil
	}
	var blocked bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))`, recipient, actor).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return nil
	}
	raw, _ := json.Marshal(data)
	var id int64
	var created time.Time
	err := tx.QueryRow(ctx, `INSERT INTO notifications(recipient_id,actor_id,event_type,target_type,target_id,dedupe_key,data) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7) ON CONFLICT (recipient_id,dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING RETURNING id,created_at`, recipient, actor, event, targetType, targetID, dedupe, raw).Scan(&id, &created)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var username, displayName, avatarURL string
	_ = tx.QueryRow(ctx, `SELECT username,display_name,COALESCE(avatar_url,'') FROM users WHERE id=$1`, actor).Scan(&username, &displayName, &avatarURL)
	payload, _ := json.Marshal(map[string]any{
		"type": "notification.created",
		"payload": map[string]any{
			"id": id, "event_type": event,
			"actor":       map[string]any{"id": actor, "username": username, "display_name": displayName, "avatar_url": avatarURL},
			"target_type": targetType, "target_id": targetID, "data": data, "created_at": created,
		},
	})
	_, err = tx.Exec(ctx, `INSERT INTO notification_outbox(notification_id,recipient_id,payload) VALUES($1,$2,$3)`, id, recipient, payload)
	return err
}

func (r *SocialRepo) PublishedByUser(ctx context.Context, username string, viewer identity.UserID, page, size int) (*appsocial.PublishedPage, error) {
	return r.publishedPage(ctx, `u.username=$1`, []any{username}, viewer, page, size)
}
func (r *SocialRepo) FollowingFeed(ctx context.Context, viewer identity.UserID, page, size int) (*appsocial.PublishedPage, error) {
	return r.publishedPage(ctx, `EXISTS(SELECT 1 FROM follows f WHERE f.follower_id=$1 AND f.followed_id=pc.author_id)`, []any{viewer}, viewer, page, size)
}
func (r *SocialRepo) publishedPage(ctx context.Context, predicate string, args []any, viewer identity.UserID, page, size int) (*appsocial.PublishedPage, error) {
	// viewer is always the first predicate argument in current callers; the extra block filter uses the next placeholder.
	viewerPos := len(args) + 1
	limitPos := viewerPos + 1
	offsetPos := viewerPos + 2
	q := fmt.Sprintf(`SELECT pc.canvas_id,pc.author_id,u.display_name,u.username,COALESCE(u.avatar_url,''),pc.title,COALESCE(pc.snapshot_url,''),pc.like_count,pc.comment_count,pc.fork_count,EXTRACT(EPOCH FROM pc.published_at)::bigint
	FROM published_canvases pc JOIN users u ON u.id=pc.author_id WHERE %s AND pc.moderation_status='visible' AND u.status='active'
	AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=$%d AND b.blocked_id=pc.author_id) OR (b.blocker_id=pc.author_id AND b.blocked_id=$%d))
	ORDER BY pc.published_at DESC LIMIT $%d OFFSET $%d`, predicate, viewerPos, viewerPos, limitPos, offsetPos)
	queryArgs := append(append([]any{}, args...), viewer, size, (page-1)*size)
	rows, err := r.pool.Query(ctx, q, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []appsocial.PublishedItemDTO{}
	for rows.Next() {
		var v appsocial.PublishedItemDTO
		if err = rows.Scan(&v.CanvasID, &v.AuthorID, &v.AuthorName, &v.AuthorUsername, &v.AuthorAvatarURL, &v.Title, &v.SnapshotURL, &v.LikeCount, &v.CommentCount, &v.ForkCount, &v.PublishedAt); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM published_canvases pc JOIN users u ON u.id=pc.author_id WHERE %s AND pc.moderation_status='visible' AND u.status='active' AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=$%d AND b.blocked_id=pc.author_id) OR (b.blocker_id=pc.author_id AND b.blocked_id=$%d))`, predicate, viewerPos, viewerPos)
	var total int64
	if err = r.pool.QueryRow(ctx, countQ, append(args, viewer)...).Scan(&total); err != nil {
		return nil, err
	}
	return &appsocial.PublishedPage{Items: items, TotalCount: total, Page: page, PageSize: size}, nil
}
