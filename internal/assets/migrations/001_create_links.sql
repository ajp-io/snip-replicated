CREATE TABLE IF NOT EXISTS links (
    id           BIGSERIAL    PRIMARY KEY,
    slug         VARCHAR(64)  NOT NULL UNIQUE,
    destination  TEXT         NOT NULL,
    label        TEXT,
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_links_slug ON links (slug);
