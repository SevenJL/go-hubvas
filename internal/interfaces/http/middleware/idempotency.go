package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hubvas/internal/domain/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

type idempotencyRecord struct {
	Hash        []byte
	State       string
	Status      *int
	ContentType *string
	Body        []byte
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type captureWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *captureWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *captureWriter) WriteString(data string) (int, error) {
	w.body.WriteString(data)
	return w.ResponseWriter.WriteString(data)
}

// Idempotency provides durable request deduplication for authenticated write
// operations. Clients that do not send Idempotency-Key keep the normal API
// behaviour; clients that do are protected against retries and double clicks.
func Idempotency(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}
		if pool == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": "idempotency_unavailable", "message": "request reliability service unavailable"})
			return
		}
		if !idempotencyKeyPattern.MatchString(key) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": "invalid_idempotency_key", "message": "Idempotency-Key must be 1-128 safe characters"})
			return
		}
		value, ok := c.Get("userID")
		userID, valid := value.(identity.UserID)
		if !ok || !valid || userID <= 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "authentication required"})
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": "invalid_body", "message": "request body could not be read"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		scope := c.FullPath()
		if scope == "" {
			scope = c.Request.URL.Path
		}
		scope = c.Request.Method + " " + scope
		sum := sha256.Sum256(append([]byte(scope+"\x00"), body...))

		claimed, err := claimIdempotency(c, pool, int64(userID), scope, key, sum[:])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": "idempotency_unavailable", "message": "request reliability service unavailable"})
			return
		}
		if claimed != nil {
			if !bytes.Equal(claimed.Hash, sum[:]) {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": "idempotency_conflict", "message": "Idempotency-Key was already used with a different request"})
				return
			}
			if claimed.State == "completed" && claimed.Status != nil {
				if claimed.ContentType != nil && *claimed.ContentType != "" {
					c.Header("Content-Type", *claimed.ContentType)
				}
				c.Header("Idempotency-Replayed", "true")
				c.Status(*claimed.Status)
				_, _ = c.Writer.Write(claimed.Body)
				c.Abort()
				return
			}
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": "request_in_progress", "message": "an identical request is still being processed"})
			return
		}

		writer := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()
		status := c.Writer.Status()
		if status >= 500 {
			_, _ = pool.Exec(c.Request.Context(), `DELETE FROM idempotency_keys WHERE user_id=$1 AND scope=$2 AND idempotency_key=$3 AND state='processing'`, userID, scope, key)
			return
		}
		contentType := c.Writer.Header().Get("Content-Type")
		_, err = pool.Exec(c.Request.Context(), `UPDATE idempotency_keys SET state='completed',status_code=$1,response_content_type=$2,response_body=$3 WHERE user_id=$4 AND scope=$5 AND idempotency_key=$6 AND request_hash=$7`, status, contentType, writer.body.Bytes(), userID, scope, key, sum[:])
		if err != nil {
			// The business operation already completed; do not replace its response.
			c.Header("Idempotency-Store-Error", hex.EncodeToString(sum[:4]))
		}
	}
}

// claimIdempotency returns nil when the caller owns a newly inserted record.
func claimIdempotency(c *gin.Context, pool *pgxpool.Pool, userID int64, scope, key string, hash []byte) (*idempotencyRecord, error) {
	for attempt := 0; attempt < 2; attempt++ {
		tag, err := pool.Exec(c.Request.Context(), `INSERT INTO idempotency_keys(user_id,scope,idempotency_key,request_hash) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, userID, scope, key, hash)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 1 {
			return nil, nil
		}
		var record idempotencyRecord
		err = pool.QueryRow(c.Request.Context(), `SELECT request_hash,state,status_code,response_content_type,response_body,created_at,expires_at FROM idempotency_keys WHERE user_id=$1 AND scope=$2 AND idempotency_key=$3`, userID, scope, key).Scan(&record.Hash, &record.State, &record.Status, &record.ContentType, &record.Body, &record.CreatedAt, &record.ExpiresAt)
		if err != nil {
			if err == pgx.ErrNoRows {
				continue
			}
			return nil, err
		}
		now := time.Now()
		expired := record.ExpiresAt.Before(now)
		staleProcessing := record.State == "processing" && record.CreatedAt.Before(now.Add(-5*time.Minute))
		if expired || staleProcessing {
			command := `DELETE FROM idempotency_keys WHERE user_id=$1 AND scope=$2 AND idempotency_key=$3 AND expires_at<now()`
			if staleProcessing {
				command = `DELETE FROM idempotency_keys WHERE user_id=$1 AND scope=$2 AND idempotency_key=$3 AND state='processing' AND created_at<now()-interval '5 minutes'`
			}
			if _, err = pool.Exec(c.Request.Context(), command, userID, scope, key); err != nil {
				return nil, err
			}
			continue
		}
		return &record, nil
	}
	return nil, pgx.ErrNoRows
}
