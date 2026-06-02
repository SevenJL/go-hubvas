-- 002_create_canvases.sql
-- Canvas metadata and membership tables.

CREATE TABLE IF NOT EXISTS canvases (
    id           BIGSERIAL    PRIMARY KEY,
    owner_id     BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title        VARCHAR(200) NOT NULL,
    snapshot_key TEXT,
    visibility   SMALLINT     NOT NULL DEFAULT 0,  -- 0=private, 1=published
    forked_from  BIGINT       REFERENCES canvases(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_canvases_owner_id ON canvases (owner_id);
CREATE INDEX IF NOT EXISTS idx_canvases_visibility ON canvases (visibility) WHERE visibility = 1;

-- Canvas members: which users have what role on which canvas.
CREATE TABLE IF NOT EXISTS canvas_members (
    canvas_id BIGINT  NOT NULL REFERENCES canvases(id) ON DELETE CASCADE,
    user_id   BIGINT  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      SMALLINT NOT NULL,  -- 0=owner, 1=editor, 2=viewer, 3=commenter
    PRIMARY KEY (canvas_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_canvas_members_user_id ON canvas_members (user_id);
