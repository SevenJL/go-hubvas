package postgres

import (
	"context"
	appmedia "github.com/hubvas/internal/application/media"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MediaRepo struct{ pool *pgxpool.Pool }

func NewMediaRepo(p *pgxpool.Pool) *MediaRepo { return &MediaRepo{pool: p} }
func (r *MediaRepo) CreateUpload(ctx context.Context, u appmedia.Upload) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO media_uploads(id,user_id,kind,temp_key,content_type,expected_size,state,expires_at) VALUES($1,$2,'avatar',$3,$4,$5,'pending',$6)`, u.ID, u.UserID, u.TempKey, u.ContentType, u.ExpectedSize, u.ExpiresAt)
	return mapPgError(err)
}
func (r *MediaRepo) GetUpload(ctx context.Context, id string, user identity.UserID) (appmedia.Upload, error) {
	var u appmedia.Upload
	u.ID = id
	u.UserID = user
	err := r.pool.QueryRow(ctx, `SELECT temp_key,content_type,expected_size,state,expires_at FROM media_uploads WHERE id=$1 AND user_id=$2`, id, user).Scan(&u.TempKey, &u.ContentType, &u.ExpectedSize, &u.State, &u.ExpiresAt)
	return u, mapPgError(err)
}
func (r *MediaRepo) FinalizeAvatar(ctx context.Context, id string, user identity.UserID, key, url string, version int64) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var old *string
	if err = tx.QueryRow(ctx, `SELECT avatar_key FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&old); err != nil {
		return "", mapPgError(err)
	}
	tag, err := tx.Exec(ctx, `UPDATE media_uploads SET state='completed',completed_at=now() WHERE id=$1 AND user_id=$2 AND state='pending'`, id, user)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "", shared.NewDomainError(shared.ErrConflict, "upload is not pending")
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET avatar_key=$1,avatar_url=$2,avatar_version=$3,updated_at=now() WHERE id=$4`, key, url, version, user); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return derefString(old), nil
}
func (r *MediaRepo) RemoveAvatar(ctx context.Context, user identity.UserID) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var old *string
	if err = tx.QueryRow(ctx, `SELECT avatar_key FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&old); err != nil {
		return "", mapPgError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET avatar_key=NULL,avatar_url=NULL,avatar_version=0,updated_at=now() WHERE id=$1`, user); err != nil {
		return "", mapPgError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return derefString(old), nil
}

func (r *MediaRepo) CurrentAvatar(ctx context.Context, user identity.UserID) (*appmedia.AvatarResponse, error) {
	var url string
	var version int64
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(avatar_url,''), avatar_version FROM users WHERE id=$1`, user).Scan(&url, &version)
	if err != nil {
		return nil, mapPgError(err)
	}
	return &appmedia.AvatarResponse{AvatarURL: url, AvatarVersion: version}, nil
}

func (r *MediaRepo) ExpiredUploads(ctx context.Context, limit int) ([]appmedia.Upload, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text,user_id,temp_key,content_type,expected_size,state,expires_at FROM media_uploads WHERE state IN ('pending','failed') AND expires_at < now() ORDER BY expires_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []appmedia.Upload
	for rows.Next() {
		var u appmedia.Upload
		if err := rows.Scan(&u.ID, &u.UserID, &u.TempKey, &u.ContentType, &u.ExpectedSize, &u.State, &u.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (r *MediaRepo) DeleteUpload(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM media_uploads WHERE id=$1 AND state IN ('pending','failed')`, id)
	return err
}
