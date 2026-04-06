CREATE TABLE IF NOT EXISTS click_events (
    id         BIGSERIAL   PRIMARY KEY,
    link_id    BIGINT      NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    referrer   TEXT
);

CREATE INDEX IF NOT EXISTS idx_click_events_link_id  ON click_events (link_id);
CREATE INDEX IF NOT EXISTS idx_click_events_clicked_at ON click_events (clicked_at);
