package nats

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	natsgo "github.com/nats-io/nats.go"
)

type outboxItem struct {
	ID          int64
	RecipientID int64
	Payload     []byte
}

// NotificationDispatcher reliably drains the transactional notification outbox.
// Rows are leased before publishing so multiple API replicas can run dispatchers safely.
type NotificationDispatcher struct {
	pool *pgxpool.Pool
	nc   *natsgo.Conn
}

func NewNotificationDispatcher(pool *pgxpool.Pool, nc *natsgo.Conn) *NotificationDispatcher {
	return &NotificationDispatcher{pool: pool, nc: nc}
}

func (d *NotificationDispatcher) Run(ctx context.Context) {
	if d == nil || d.pool == nil || d.nc == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := d.dispatchBatch(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[notifications] outbox dispatch failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *NotificationDispatcher) dispatchBatch(ctx context.Context) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		SELECT id, recipient_id, payload
		FROM notification_outbox
		WHERE published_at IS NULL AND available_at <= now()
		ORDER BY id
		FOR UPDATE SKIP LOCKED
		LIMIT 50`)
	if err != nil {
		return err
	}
	var items []outboxItem
	for rows.Next() {
		var item outboxItem
		if err := rows.Scan(&item.ID, &item.RecipientID, &item.Payload); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, `UPDATE notification_outbox SET attempts=attempts+1, available_at=now()+interval '30 seconds' WHERE id=$1`, item.ID); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	for _, item := range items {
		subject := fmt.Sprintf("notifications.user.%d", item.RecipientID)
		if err := d.nc.Publish(subject, item.Payload); err != nil {
			_, _ = d.pool.Exec(ctx, `UPDATE notification_outbox SET last_error=$1, available_at=now()+LEAST(interval '5 minutes', interval '2 seconds' * GREATEST(attempts,1)) WHERE id=$2 AND published_at IS NULL`, err.Error(), item.ID)
			continue
		}
		if err := d.nc.FlushTimeout(2 * time.Second); err != nil {
			_, _ = d.pool.Exec(ctx, `UPDATE notification_outbox SET last_error=$1 WHERE id=$2 AND published_at IS NULL`, err.Error(), item.ID)
			continue
		}
		_, _ = d.pool.Exec(ctx, `UPDATE notification_outbox SET published_at=now(),last_error=NULL WHERE id=$1 AND published_at IS NULL`, item.ID)
	}
	return nil
}
