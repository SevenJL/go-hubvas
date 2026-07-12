-- 005_social_foundation.sql
-- Production social profiles, relationships, moderation, notifications, and media uploads.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS display_name VARCHAR(50),
    ADD COLUMN IF NOT EXISTS bio VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS website VARCHAR(2048) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS avatar_key TEXT,
    ADD COLUMN IF NOT EXISTS avatar_version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS account_role VARCHAR(16) NOT NULL DEFAULT 'user',
    ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE users SET display_name = username WHERE display_name IS NULL OR display_name = '';
ALTER TABLE users ALTER COLUMN display_name SET NOT NULL;

DO $$ BEGIN
    ALTER TABLE users ADD CONSTRAINT chk_users_account_role CHECK (account_role IN ('user','admin'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE users ADD CONSTRAINT chk_users_status CHECK (status IN ('active','suspended'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS follows (
    follower_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followed_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, followed_id),
    CONSTRAINT chk_follows_not_self CHECK (follower_id <> followed_id)
);
CREATE INDEX IF NOT EXISTS idx_follows_followed ON follows (followed_id, created_at DESC);

CREATE TABLE IF NOT EXISTS blocks (
    blocker_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CONSTRAINT chk_blocks_not_self CHECK (blocker_id <> blocked_id)
);
CREATE INDEX IF NOT EXISTS idx_blocks_blocked ON blocks (blocked_id);

ALTER TABLE comments
    ADD COLUMN IF NOT EXISTS parent_comment_id BIGINT REFERENCES comments(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS moderation_status VARCHAR(16) NOT NULL DEFAULT 'visible';
DO $$ BEGIN
    ALTER TABLE comments ADD CONSTRAINT chk_comments_moderation CHECK (moderation_status IN ('visible','hidden'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
CREATE INDEX IF NOT EXISTS idx_comments_parent ON comments (parent_comment_id, created_at ASC);

CREATE TABLE IF NOT EXISTS media_uploads (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind VARCHAR(32) NOT NULL,
    temp_key TEXT NOT NULL UNIQUE,
    content_type VARCHAR(100) NOT NULL,
    expected_size BIGINT NOT NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    CONSTRAINT chk_media_upload_state CHECK (state IN ('pending','processing','completed','failed'))
);
CREATE INDEX IF NOT EXISTS idx_media_upload_expiry ON media_uploads (state, expires_at);

CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    recipient_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    event_type VARCHAR(32) NOT NULL,
    target_type VARCHAR(32) NOT NULL,
    target_id BIGINT NOT NULL,
    dedupe_key VARCHAR(255),
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_notifications_dedupe ON notifications (recipient_id, dedupe_key) WHERE dedupe_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_recipient ON notifications (recipient_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications (recipient_id, created_at DESC) WHERE read_at IS NULL;

CREATE TABLE IF NOT EXISTS notification_outbox (
    id BIGSERIAL PRIMARY KEY,
    notification_id BIGINT NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    recipient_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_outbox_pending ON notification_outbox (available_at, id) WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS reports (
    id BIGSERIAL PRIMARY KEY,
    reporter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type VARCHAR(16) NOT NULL,
    target_id BIGINT NOT NULL,
    reason VARCHAR(32) NOT NULL,
    details VARCHAR(1000) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    reviewer_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    review_note VARCHAR(1000) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at TIMESTAMPTZ,
    CONSTRAINT chk_reports_target CHECK (target_type IN ('user','canvas','comment')),
    CONSTRAINT chk_reports_reason CHECK (reason IN ('spam','harassment','inappropriate','copyright','other')),
    CONSTRAINT chk_reports_status CHECK (status IN ('pending','reviewing','resolved','dismissed'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_reports_open ON reports (reporter_id, target_type, target_id) WHERE status IN ('pending','reviewing');
CREATE INDEX IF NOT EXISTS idx_reports_queue ON reports (status, created_at ASC);

ALTER TABLE published_canvases ADD COLUMN IF NOT EXISTS moderation_status VARCHAR(16) NOT NULL DEFAULT 'visible';
DO $$ BEGIN
    ALTER TABLE published_canvases ADD CONSTRAINT chk_published_moderation CHECK (moderation_status IN ('visible','hidden'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
CREATE INDEX IF NOT EXISTS idx_published_moderation ON published_canvases (moderation_status, published_at DESC);
