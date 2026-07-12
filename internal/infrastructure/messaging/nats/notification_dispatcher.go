package nats

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	natsgo "github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
)

const maxDeliveryAttempts = 12

type outboxItem struct {
	ID          int64
	RecipientID int64
	Payload     []byte
	Attempts    int
}

var (
	outboxPending = prometheus.NewGauge(prometheus.GaugeOpts{Namespace: "hubvas", Subsystem: "notifications", Name: "outbox_pending", Help: "Notification outbox rows waiting for delivery."})
	outboxDead    = prometheus.NewGauge(prometheus.GaugeOpts{Namespace: "hubvas", Subsystem: "notifications", Name: "outbox_dead_letter", Help: "Notification outbox rows moved to dead letter."})
	outboxRetries = prometheus.NewCounter(prometheus.CounterOpts{Namespace: "hubvas", Subsystem: "notifications", Name: "outbox_retries_total", Help: "Failed notification delivery attempts."})
)

func init() { prometheus.MustRegister(outboxPending, outboxDead, outboxRetries) }

// NotificationDispatcher reliably drains the transactional notification outbox.
// A database lease makes each row exclusive across application replicas while
// still allowing another replica to recover work after a crash.
type NotificationDispatcher struct {
	pool        *pgxpool.Pool
	nc          *natsgo.Conn
	owner       string
	lease       time.Duration
	batchSize   int
	maxAttempts int
}

func NewNotificationDispatcher(pool *pgxpool.Pool, nc *natsgo.Conn) *NotificationDispatcher {
	return &NotificationDispatcher{pool: pool, nc: nc, owner: uuid.NewString(), lease: 30 * time.Second, batchSize: 50, maxAttempts: maxDeliveryAttempts}
}

func (d *NotificationDispatcher) Run(ctx context.Context) {
	if d == nil || d.pool == nil || d.nc == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	metricsTicker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	defer metricsTicker.Stop()
	for {
		if err := d.dispatchBatch(ctx); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "notification outbox dispatch failed", "owner", d.owner, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-metricsTicker.C:
			d.updateMetrics(ctx)
		case <-ticker.C:
		}
	}
}

func (d *NotificationDispatcher) dispatchBatch(ctx context.Context) error {
	items, err := d.claim(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		subject := fmt.Sprintf("notifications.user.%d", item.RecipientID)
		err = d.nc.Publish(subject, item.Payload)
		if err == nil {
			err = d.nc.FlushTimeout(2 * time.Second)
		}
		if err != nil {
			outboxRetries.Inc()
			d.fail(ctx, item, err)
			continue
		}
		if _, err = d.pool.Exec(ctx, `UPDATE notification_outbox SET published_at=now(),last_error=NULL,lease_owner=NULL,leased_until=NULL WHERE id=$1 AND published_at IS NULL AND lease_owner=$2`, item.ID, d.owner); err != nil {
			return err
		}
	}
	return nil
}

func (d *NotificationDispatcher) claim(ctx context.Context) ([]outboxItem, error) {
	rows, err := d.pool.Query(ctx, `
		WITH candidates AS (
			SELECT id FROM notification_outbox
			WHERE published_at IS NULL
			  AND dead_lettered_at IS NULL
			  AND available_at <= now()
			  AND (leased_until IS NULL OR leased_until < now())
			ORDER BY id
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE notification_outbox o
		SET attempts=o.attempts+1,lease_owner=$2,leased_until=now()+$3::interval
		FROM candidates c
		WHERE o.id=c.id
		RETURNING o.id,o.recipient_id,o.payload,o.attempts`, d.batchSize, d.owner, d.lease.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]outboxItem, 0, d.batchSize)
	for rows.Next() {
		var item outboxItem
		if err = rows.Scan(&item.ID, &item.RecipientID, &item.Payload, &item.Attempts); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *NotificationDispatcher) fail(ctx context.Context, item outboxItem, publishErr error) {
	if item.Attempts >= d.maxAttempts {
		_, _ = d.pool.Exec(ctx, `UPDATE notification_outbox SET dead_lettered_at=now(),last_error=$1,lease_owner=NULL,leased_until=NULL WHERE id=$2 AND lease_owner=$3 AND published_at IS NULL`, publishErr.Error(), item.ID, d.owner)
		return
	}
	backoff := 2 * time.Second * time.Duration(1<<min(item.Attempts-1, 7))
	if backoff > 5*time.Minute {
		backoff = 5 * time.Minute
	}
	jitter := time.Duration(rand.Int63n(max(int64(backoff/4), 1)))
	_, _ = d.pool.Exec(ctx, `UPDATE notification_outbox SET last_error=$1,available_at=now()+$2::interval,lease_owner=NULL,leased_until=NULL WHERE id=$3 AND lease_owner=$4 AND published_at IS NULL`, publishErr.Error(), (backoff + jitter).String(), item.ID, d.owner)
}

func (d *NotificationDispatcher) updateMetrics(ctx context.Context) {
	var pending, dead float64
	_ = d.pool.QueryRow(ctx, `SELECT COUNT(*)::float8 FROM notification_outbox WHERE published_at IS NULL AND dead_lettered_at IS NULL`).Scan(&pending)
	_ = d.pool.QueryRow(ctx, `SELECT COUNT(*)::float8 FROM notification_outbox WHERE dead_lettered_at IS NOT NULL`).Scan(&dead)
	outboxPending.Set(pending)
	outboxDead.Set(dead)
}

// ReplayDeadLetters makes a bounded set of dead-letter rows eligible for a
// fresh delivery cycle. It is intended for an authenticated admin operation or
// an operational repair command.
func (d *NotificationDispatcher) ReplayDeadLetters(ctx context.Context, limit int) (int64, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	tag, err := d.pool.Exec(ctx, `UPDATE notification_outbox SET dead_lettered_at=NULL,attempts=0,last_error=NULL,available_at=now(),lease_owner=NULL,leased_until=NULL WHERE id IN (SELECT id FROM notification_outbox WHERE dead_lettered_at IS NOT NULL ORDER BY id LIMIT $1)`, limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
