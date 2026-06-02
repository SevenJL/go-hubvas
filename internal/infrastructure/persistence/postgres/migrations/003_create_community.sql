-- 003_create_community.sql
-- Community tables: published canvases, tags, likes, comments, forks.

-- Read-side projection: a canvas that has been published to the community.
-- Denormalized counters (like_count, comment_count, fork_count) are
-- maintained by the application layer.
CREATE TABLE IF NOT EXISTS published_canvases (
    canvas_id     BIGINT       PRIMARY KEY REFERENCES canvases(id) ON DELETE CASCADE,
    author_id     BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title         VARCHAR(200) NOT NULL,
    snapshot_url  TEXT,
    like_count    BIGINT       NOT NULL DEFAULT 0,
    comment_count BIGINT       NOT NULL DEFAULT 0,
    fork_count    BIGINT       NOT NULL DEFAULT 0,
    published_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_published_author    ON published_canvases (author_id);
CREATE INDEX IF NOT EXISTS idx_published_date      ON published_canvases (published_at DESC);
CREATE INDEX IF NOT EXISTS idx_published_likes     ON published_canvases (like_count DESC);

-- Tags for published canvases (many-to-many).
CREATE TABLE IF NOT EXISTS canvas_tags (
    canvas_id BIGINT      NOT NULL REFERENCES canvases(id) ON DELETE CASCADE,
    tag       VARCHAR(50) NOT NULL,
    PRIMARY KEY (canvas_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_canvas_tags_tag ON canvas_tags (tag);

-- Likes: composite primary key prevents duplicates.
CREATE TABLE IF NOT EXISTS likes (
    canvas_id  BIGINT      NOT NULL REFERENCES canvases(id) ON DELETE CASCADE,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (canvas_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_likes_user ON likes (user_id);

-- Comments on published canvases.
CREATE TABLE IF NOT EXISTS comments (
    id         BIGSERIAL    PRIMARY KEY,
    canvas_id  BIGINT       NOT NULL REFERENCES canvases(id) ON DELETE CASCADE,
    author_id  BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT         NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_comments_canvas ON comments (canvas_id, created_at DESC);

-- Fork lineage: records which canvas was forked from which.
CREATE TABLE IF NOT EXISTS forks (
    original_canvas_id BIGINT      NOT NULL REFERENCES canvases(id) ON DELETE CASCADE,
    new_canvas_id      BIGINT      NOT NULL REFERENCES canvases(id) ON DELETE CASCADE,
    user_id            BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (original_canvas_id, new_canvas_id)
);

CREATE INDEX IF NOT EXISTS idx_forks_original ON forks (original_canvas_id);
CREATE INDEX IF NOT EXISTS idx_forks_new      ON forks (new_canvas_id);
