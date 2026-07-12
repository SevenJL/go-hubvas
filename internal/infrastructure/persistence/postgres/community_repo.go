package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	canvasDomain "github.com/hubvas/internal/domain/canvas"
	communityDomain "github.com/hubvas/internal/domain/community"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// CommunityRepo implements community.CommunityRepository using PostgreSQL via pgx.
//
// Published canvases are a read-side projection with denormalized counters.
// Tag-based search uses a separate canvas_tags join table.
type CommunityRepo struct {
	pool *pgxpool.Pool
}

// NewCommunityRepo creates a CommunityRepo backed by the given pgx pool.
func NewCommunityRepo(pool *pgxpool.Pool) *CommunityRepo {
	return &CommunityRepo{pool: pool}
}

// ---- PublishedCanvas ----

const (
	communityUpsertPublishedSQL = `
		INSERT INTO published_canvases (canvas_id, author_id, title, snapshot_url, like_count, comment_count, fork_count, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (canvas_id) DO UPDATE SET
			title        = EXCLUDED.title,
			snapshot_url = EXCLUDED.snapshot_url,
			like_count   = EXCLUDED.like_count,
			comment_count = EXCLUDED.comment_count,
			fork_count   = EXCLUDED.fork_count`

	communityDeleteTagsSQL = `DELETE FROM canvas_tags WHERE canvas_id = $1`
	communityInsertTagSQL  = `INSERT INTO canvas_tags (canvas_id, tag) VALUES ($1, $2) ON CONFLICT DO NOTHING`

	communitySelectPublishedSQL = `
		SELECT canvas_id, author_id, title, snapshot_url, like_count, comment_count, fork_count, published_at
		FROM published_canvases
		WHERE canvas_id = $1 AND moderation_status = 'visible'`

	communityRemovePublishedSQL = `DELETE FROM published_canvases WHERE canvas_id = $1`
)

// SavePublished upserts a published canvas and syncs its tags.
func (r *CommunityRepo) SavePublished(ctx context.Context, pc *communityDomain.PublishedCanvas) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, communityUpsertPublishedSQL,
		pc.CanvasID(),
		pc.AuthorID(),
		pc.Title(),
		nullIfEmpty(pc.SnapshotURL()),
		pc.LikeCount(),
		pc.CommentCount(),
		pc.ForkCount(),
		pc.PublishedAt(),
	)
	if err != nil {
		return mapPgError(err)
	}

	// Sync tags: delete old, re-insert current.
	if _, err := tx.Exec(ctx, communityDeleteTagsSQL, pc.CanvasID()); err != nil {
		return err
	}
	for _, tag := range pc.Tags() {
		if _, err := tx.Exec(ctx, communityInsertTagSQL, pc.CanvasID(), tag); err != nil {
			return mapPgError(err)
		}
	}

	return tx.Commit(ctx)
}

// FindPublishedByID loads a single published canvas with its tags.
func (r *CommunityRepo) FindPublishedByID(ctx context.Context, id canvasDomain.CanvasID) (*communityDomain.PublishedCanvas, error) {
	pc, err := r.scanPublished(ctx, communitySelectPublishedSQL, id)
	if err != nil {
		return nil, err
	}
	if pc == nil {
		return nil, shared.NewDomainError(shared.ErrNotFound, "published canvas not found")
	}

	tags, err := r.loadTags(ctx, id)
	if err != nil {
		return nil, err
	}
	pc.UpdateTags(tags)
	return pc, nil
}

// FindPublished returns a paginated, sorted list of published canvases.
func (r *CommunityRepo) FindPublished(ctx context.Context, query communityDomain.SearchQuery) ([]*communityDomain.PublishedCanvas, int64, error) {
	return r.searchPublished(ctx, query, false)
}

// SearchByTags returns published canvases matching the given tags.
func (r *CommunityRepo) SearchByTags(ctx context.Context, tags []string, page communityDomain.Pagination) ([]*communityDomain.PublishedCanvas, int64, error) {
	query := communityDomain.SearchQuery{
		Tags:     tags,
		SortBy:   communityDomain.SortByLatest,
		Page:     page.Page,
		PageSize: page.PageSize,
	}
	return r.searchPublished(ctx, query, true)
}

// RemovePublished deletes a published canvas (CASCADE handles tags).
func (r *CommunityRepo) RemovePublished(ctx context.Context, id canvasDomain.CanvasID) error {
	tag, err := r.pool.Exec(ctx, communityRemovePublishedSQL, id)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "published canvas not found")
	}
	return nil
}

// ---- searchPublished ----

func (r *CommunityRepo) searchPublished(ctx context.Context, query communityDomain.SearchQuery, tagsOnly bool) ([]*communityDomain.PublishedCanvas, int64, error) {
	whereClauses := []string{"pc.moderation_status = 'visible'", "EXISTS (SELECT 1 FROM users u WHERE u.id = pc.author_id AND u.status = 'active')"}
	args := []any{}
	argIdx := 1
	if query.ViewerID != 0 {
		whereClauses = append(whereClauses, fmt.Sprintf(`NOT EXISTS (
			SELECT 1 FROM blocks b
			WHERE (b.blocker_id=$%d AND b.blocked_id=pc.author_id)
			   OR (b.blocker_id=pc.author_id AND b.blocked_id=$%d)
		)`, argIdx, argIdx))
		args = append(args, query.ViewerID)
		argIdx++
	}

	// Keyword filter (title ILIKE)
	if query.Keyword != "" && !tagsOnly {
		whereClauses = append(whereClauses, fmt.Sprintf("pc.title ILIKE $%d", argIdx))
		args = append(args, "%"+query.Keyword+"%")
		argIdx++
	}

	// Tag filtering uses EXISTS instead of a JOIN so each published canvas
	// appears at most once. This also removes the need for SELECT DISTINCT,
	// allowing computed expressions such as the trending score in ORDER BY.
	if len(query.Tags) > 0 {
		placeholders := make([]string, len(query.Tags))
		for i, tag := range query.Tags {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, tag)
			argIdx++
		}
		whereClauses = append(whereClauses, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM canvas_tags ct WHERE ct.canvas_id = pc.canvas_id AND ct.tag IN (%s))",
			strings.Join(placeholders, ","),
		))
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count query
	countSQL := fmt.Sprintf(
		"SELECT COUNT(*) FROM published_canvases pc %s",
		whereSQL,
	)
	var total int64
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, mapPgError(err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	// Sort
	orderBy := "pc.published_at DESC"
	switch query.SortBy {
	case communityDomain.SortByPopular:
		orderBy = "pc.like_count DESC"
	case communityDomain.SortByTrending:
		// Trending = weighted by recency and likes.
		orderBy = "(pc.like_count * 10 + EXTRACT(EPOCH FROM pc.published_at) / 86400) DESC"
	}

	// Pagination
	limit := query.PageSize
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	offset := (query.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	dataSQL := buildPublishedDataSQL(whereSQL, orderBy, argIdx)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, mapPgError(err)
	}
	defer rows.Close()

	var results []*communityDomain.PublishedCanvas
	var canvasIDs []canvasDomain.CanvasID
	for rows.Next() {
		pc, err := scanPublishedRow(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, pc)
		canvasIDs = append(canvasIDs, pc.CanvasID())
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Batch-load tags.
	tagMap, err := r.batchLoadTags(ctx, canvasIDs)
	if err != nil {
		return nil, 0, err
	}
	for _, pc := range results {
		if tags, ok := tagMap[pc.CanvasID()]; ok {
			pc.UpdateTags(tags)
		}
	}

	return results, total, nil
}

func buildPublishedDataSQL(whereSQL, orderBy string, argIdx int) string {
	return fmt.Sprintf(
		`SELECT pc.canvas_id, pc.author_id, pc.title, pc.snapshot_url,
		        pc.like_count, pc.comment_count, pc.fork_count, pc.published_at
		 FROM published_canvases pc %s
		 ORDER BY %s, pc.canvas_id DESC
		 LIMIT $%d OFFSET $%d`,
		whereSQL, orderBy, argIdx, argIdx+1,
	)
}

// ---- Like ----

const (
	communityInsertLikeSQL = `
		INSERT INTO likes (canvas_id, user_id, created_at)
		VALUES ($1, $2, $3)`
	communityDeleteLikeSQL    = `DELETE FROM likes WHERE canvas_id = $1 AND user_id = $2`
	communityIncrementLikeSQL = `
		UPDATE published_canvases
		SET like_count = like_count + 1
		WHERE canvas_id = $1
		RETURNING like_count`
	communityDecrementLikeSQL = `
		UPDATE published_canvases
		SET like_count = GREATEST(like_count - 1, 0)
		WHERE canvas_id = $1
		RETURNING like_count`
	communityHasLikedSQL   = `SELECT EXISTS(SELECT 1 FROM likes WHERE canvas_id = $1 AND user_id = $2)`
	communityCountLikesSQL = `SELECT COUNT(*) FROM likes WHERE canvas_id = $1`
)

// LikeCanvas inserts a like and updates the projection counter in one transaction.
func (r *CommunityRepo) LikeCanvas(ctx context.Context, like *communityDomain.Like) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var recipient identity.UserID
	err = tx.QueryRow(ctx, `SELECT author_id FROM published_canvases WHERE canvas_id=$1 AND moderation_status='visible'`, like.CanvasID).Scan(&recipient)
	if err != nil {
		return 0, mapPgError(err)
	}
	if blocked, err := interactionBlocked(ctx, tx, like.UserID, recipient); err != nil {
		return 0, err
	} else if blocked {
		return 0, shared.NewDomainError(shared.ErrForbidden, "interaction blocked")
	}
	if _, err := tx.Exec(ctx, communityInsertLikeSQL, like.CanvasID, like.UserID, like.CreatedAt); err != nil {
		return 0, mapPgError(err)
	}
	var count int64
	if err := tx.QueryRow(ctx, communityIncrementLikeSQL, like.CanvasID).Scan(&count); err != nil {
		return 0, mapPgError(err)
	}
	if err := createNotificationTx(ctx, tx, recipient, like.UserID, "like", "canvas", int64(like.CanvasID), fmt.Sprintf("like:%d:%d", like.CanvasID, like.UserID), nil); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

// UnlikeCanvas removes a like and updates the projection counter in one transaction.
func (r *CommunityRepo) UnlikeCanvas(ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, communityDeleteLikeSQL, canvasID, userID)
	if err != nil {
		return 0, mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return 0, shared.NewDomainError(shared.ErrNotFound, "like not found")
	}
	var count int64
	if err := tx.QueryRow(ctx, communityDecrementLikeSQL, canvasID).Scan(&count); err != nil {
		return 0, mapPgError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

// HasLiked checks whether a user has already liked a canvas.
func (r *CommunityRepo) HasLiked(ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, communityHasLikedSQL, canvasID, userID).Scan(&exists)
	return exists, mapPgError(err)
}

// CountLikes returns the source-of-truth count from the likes table.
func (r *CommunityRepo) CountLikes(ctx context.Context, canvasID canvasDomain.CanvasID) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, communityCountLikesSQL, canvasID).Scan(&count)
	return count, mapPgError(err)
}

// ---- Comment ----

const (
	communityInsertCommentSQL  = `INSERT INTO comments (canvas_id,author_id,parent_comment_id,content,moderation_status,created_at) VALUES ($1,$2,$3,$4,'visible',$5) RETURNING id`
	communitySelectCommentsSQL = `WITH root_page AS (
		SELECT c.id,c.created_at
		FROM comments c
		WHERE c.canvas_id=$1 AND c.parent_comment_id IS NULL
		  AND ($2=0 OR NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=$2 AND b.blocked_id=c.author_id) OR (b.blocker_id=c.author_id AND b.blocked_id=$2)))
		ORDER BY c.created_at DESC,c.id DESC
		LIMIT $3 OFFSET $4
	)
	SELECT c.id,c.canvas_id,c.author_id,c.parent_comment_id,c.content,c.deleted_at,c.moderation_status,c.created_at
	FROM root_page rp
	JOIN comments c ON c.id=rp.id OR c.parent_comment_id=rp.id
	WHERE ($2=0 OR NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=$2 AND b.blocked_id=c.author_id) OR (b.blocker_id=c.author_id AND b.blocked_id=$2)))
	ORDER BY rp.created_at DESC,rp.id DESC,c.parent_comment_id NULLS FIRST,c.created_at ASC,c.id ASC`
	communityCountCommentsSQL = `SELECT COUNT(*) FROM comments c WHERE c.canvas_id=$1 AND c.parent_comment_id IS NULL AND ($2=0 OR NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=$2 AND b.blocked_id=c.author_id) OR (b.blocker_id=c.author_id AND b.blocked_id=$2)))`
	communityDeleteCommentSQL = `DELETE FROM comments WHERE id=$1`
)

func (r *CommunityRepo) SaveComment(ctx context.Context, c *communityDomain.Comment) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var recipient identity.UserID
	event := "comment"
	if c.ParentID() != nil {
		event = "reply"
		err = tx.QueryRow(ctx, `SELECT author_id FROM comments WHERE id=$1 AND canvas_id=$2`, *c.ParentID(), c.CanvasID()).Scan(&recipient)
	} else {
		err = tx.QueryRow(ctx, `SELECT author_id FROM published_canvases WHERE canvas_id=$1 AND moderation_status='visible'`, c.CanvasID()).Scan(&recipient)
	}
	if err != nil {
		return mapPgError(err)
	}
	if blocked, err := interactionBlocked(ctx, tx, c.AuthorID(), recipient); err != nil {
		return err
	} else if blocked {
		return shared.NewDomainError(shared.ErrForbidden, "interaction blocked")
	}
	var id int64
	err = tx.QueryRow(ctx, communityInsertCommentSQL, c.CanvasID(), c.AuthorID(), c.ParentID(), c.Content(), c.CreatedAt()).Scan(&id)
	if err != nil {
		return mapPgError(err)
	}
	c.SetID(communityDomain.CommentID(id))
	if _, err = tx.Exec(ctx, `UPDATE published_canvases SET comment_count=comment_count+1 WHERE canvas_id=$1`, c.CanvasID()); err != nil {
		return err
	}
	if err = createNotificationTx(ctx, tx, recipient, c.AuthorID(), event, "comment", id, "", map[string]any{"canvas_id": c.CanvasID()}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *CommunityRepo) FindComment(ctx context.Context, id communityDomain.CommentID) (*communityDomain.Comment, error) {
	var cid, aid int64
	var parent *int64
	var content, status string
	var deleted *time.Time
	var created time.Time
	err := r.pool.QueryRow(ctx, `SELECT canvas_id,author_id,parent_comment_id,content,deleted_at,moderation_status,created_at FROM comments WHERE id=$1`, id).Scan(&cid, &aid, &parent, &content, &deleted, &status, &created)
	if err != nil {
		return nil, mapPgError(err)
	}
	var p *communityDomain.CommentID
	if parent != nil {
		v := communityDomain.CommentID(*parent)
		p = &v
	}
	return communityDomain.ReconstituteCommentFull(id, canvasDomain.CanvasID(cid), identity.UserID(aid), p, content, deleted, status, created), nil
}
func (r *CommunityRepo) SoftDeleteComment(ctx context.Context, id communityDomain.CommentID, author identity.UserID) error {
	tag, err := r.pool.Exec(ctx, `UPDATE comments SET deleted_at=COALESCE(deleted_at,now()),content='' WHERE id=$1 AND author_id=$2`, id, author)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "comment not found or not owned by user")
	}
	return nil
}
func (r *CommunityRepo) FindComments(ctx context.Context, canvasID canvasDomain.CanvasID, viewerID identity.UserID, page communityDomain.Pagination) ([]*communityDomain.Comment, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, communityCountCommentsSQL, canvasID, viewerID).Scan(&total); err != nil {
		return nil, 0, mapPgError(err)
	}
	if total == 0 {
		return []*communityDomain.Comment{}, 0, nil
	}
	rows, err := r.pool.Query(ctx, communitySelectCommentsSQL, canvasID, viewerID, page.Limit(20), page.Offset())
	if err != nil {
		return nil, 0, mapPgError(err)
	}
	defer rows.Close()
	out := []*communityDomain.Comment{}
	for rows.Next() {
		var id, cid, aid int64
		var parent *int64
		var content, status string
		var deleted *time.Time
		var created time.Time
		if err = rows.Scan(&id, &cid, &aid, &parent, &content, &deleted, &status, &created); err != nil {
			return nil, 0, err
		}
		var p *communityDomain.CommentID
		if parent != nil {
			v := communityDomain.CommentID(*parent)
			p = &v
		}
		out = append(out, communityDomain.ReconstituteCommentFull(communityDomain.CommentID(id), canvasDomain.CanvasID(cid), identity.UserID(aid), p, content, deleted, status, created))
	}
	return out, total, rows.Err()
}
func (r *CommunityRepo) DeleteComment(ctx context.Context, id communityDomain.CommentID) error {
	tag, err := r.pool.Exec(ctx, communityDeleteCommentSQL, id)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "comment not found")
	}
	return nil
}

// ---- Fork ----

const (
	communityInsertForkSQL = `
		INSERT INTO forks (original_canvas_id, new_canvas_id, user_id, created_at)
		VALUES ($1, $2, $3, $4)`

	communitySelectForksSQL = `
		SELECT original_canvas_id, new_canvas_id, user_id, created_at
		FROM forks
		WHERE original_canvas_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	communityCountForksSQL = `SELECT COUNT(*) FROM forks WHERE original_canvas_id = $1`
)

// SaveFork records a fork relationship and increments the published source's
// denormalized fork counter in the same transaction.
func (r *CommunityRepo) SaveFork(ctx context.Context, fork *communityDomain.Fork) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var recipient identity.UserID
	if err = tx.QueryRow(ctx, `SELECT author_id FROM published_canvases WHERE canvas_id=$1 AND moderation_status='visible'`, fork.OriginalCanvasID()).Scan(&recipient); err != nil {
		return mapPgError(err)
	}
	if blocked, err := interactionBlocked(ctx, tx, fork.UserID(), recipient); err != nil {
		return err
	} else if blocked {
		return shared.NewDomainError(shared.ErrForbidden, "interaction blocked")
	}
	if _, err := tx.Exec(ctx, communityInsertForkSQL, fork.OriginalCanvasID(), fork.NewCanvasID(), fork.UserID(), fork.CreatedAt()); err != nil {
		return mapPgError(err)
	}
	tag, err := tx.Exec(ctx, `UPDATE published_canvases SET fork_count=fork_count+1 WHERE canvas_id=$1`, fork.OriginalCanvasID())
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "published canvas not found")
	}
	if err = createNotificationTx(ctx, tx, recipient, fork.UserID(), "fork", "canvas", int64(fork.OriginalCanvasID()), "", map[string]any{"fork_canvas_id": fork.NewCanvasID()}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FindForks returns paginated forks for a canvas.
func (r *CommunityRepo) FindForks(ctx context.Context, canvasID canvasDomain.CanvasID, page communityDomain.Pagination) ([]*communityDomain.Fork, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, communityCountForksSQL, canvasID).Scan(&total); err != nil {
		return nil, 0, mapPgError(err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	limit := page.Limit(20)
	rows, err := r.pool.Query(ctx, communitySelectForksSQL, canvasID, limit, page.Offset())
	if err != nil {
		return nil, 0, mapPgError(err)
	}
	defer rows.Close()

	var forks []*communityDomain.Fork
	for rows.Next() {
		var origID, newID, userID int64
		var createdAt time.Time
		if err := rows.Scan(&origID, &newID, &userID, &createdAt); err != nil {
			return nil, 0, err
		}
		forks = append(forks, communityDomain.ReconstituteFork(
			canvasDomain.CanvasID(origID),
			canvasDomain.CanvasID(newID),
			identity.UserID(userID),
			createdAt,
		))
	}
	return forks, total, rows.Err()
}

// CountForks returns the total number of forks for a canvas.
func (r *CommunityRepo) CountForks(ctx context.Context, canvasID canvasDomain.CanvasID) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, communityCountForksSQL, canvasID).Scan(&count)
	return count, mapPgError(err)
}

func interactionBlocked(ctx context.Context, tx pgx.Tx, a, b identity.UserID) (bool, error) {
	var blocked bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))`, a, b).Scan(&blocked)
	return blocked, err
}

// ---- Scanning helpers ----

func (r *CommunityRepo) scanPublished(ctx context.Context, query string, arg any) (*communityDomain.PublishedCanvas, error) {
	rows, err := r.pool.Query(ctx, query, arg)
	if err != nil {
		return nil, mapPgError(err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, shared.NewDomainError(shared.ErrNotFound, "published canvas not found")
	}
	return scanPublishedRow(rows)
}

// scanPublishedRow scans the current row (the caller must have advanced the cursor via rows.Next()).
func scanPublishedRow(rows pgx.Rows) (*communityDomain.PublishedCanvas, error) {
	var (
		canvasID, authorID                 int64
		title, snapshotURL                 *string
		likeCount, commentCount, forkCount int64
		publishedAt                        time.Time
	)

	if err := rows.Scan(&canvasID, &authorID, &title, &snapshotURL, &likeCount, &commentCount, &forkCount, &publishedAt); err != nil {
		return nil, mapPgError(err)
	}

	return communityDomain.ReconstitutePublishedCanvas(
		canvasDomain.CanvasID(canvasID),
		identity.UserID(authorID),
		derefString(title),
		derefString(snapshotURL),
		nil, // tags loaded separately
		likeCount,
		commentCount,
		forkCount,
		publishedAt,
	), nil
}

// ---- Tag loading ----

const communitySelectTagsSQL = `SELECT canvas_id, tag FROM canvas_tags WHERE canvas_id = ANY($1)`

func (r *CommunityRepo) loadTags(ctx context.Context, canvasID canvasDomain.CanvasID) ([]string, error) {
	m, err := r.batchLoadTags(ctx, []canvasDomain.CanvasID{canvasID})
	if err != nil {
		return nil, err
	}
	if tags, ok := m[canvasID]; ok {
		return tags, nil
	}
	return []string{}, nil
}

func (r *CommunityRepo) batchLoadTags(ctx context.Context, ids []canvasDomain.CanvasID) (map[canvasDomain.CanvasID][]string, error) {
	result := make(map[canvasDomain.CanvasID][]string)
	if len(ids) == 0 {
		return result, nil
	}

	rows, err := r.pool.Query(ctx, communitySelectTagsSQL, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cID canvasDomain.CanvasID
		var tag string
		if err := rows.Scan(&cID, &tag); err != nil {
			return nil, err
		}
		result[cID] = append(result[cID], tag)
	}
	return result, rows.Err()
}
