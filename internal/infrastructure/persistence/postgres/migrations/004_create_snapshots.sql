-- canvas_snapshots: stores tldraw store JSON snapshots for persistence.
-- One row per canvas, upserted on every save.
CREATE TABLE IF NOT EXISTS canvas_snapshots (
    canvas_id  BIGINT PRIMARY KEY REFERENCES canvases(id) ON DELETE CASCADE,
    data       JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_canvas_snapshots_updated ON canvas_snapshots(updated_at);
