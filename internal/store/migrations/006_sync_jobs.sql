-- DAV sync jobs: each mirrors a CalDAV/CardDAV collection between two servers.
CREATE TABLE sync_jobs (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    kind             TEXT NOT NULL,                -- caldav | carddav
    direction        TEXT NOT NULL,                -- a-to-b | b-to-a | bidirectional
    conflict         TEXT NOT NULL DEFAULT 'newest-wins',
    a_url            TEXT NOT NULL,
    a_username       TEXT NOT NULL DEFAULT '',
    a_password       TEXT NOT NULL DEFAULT '',
    b_url            TEXT NOT NULL,
    b_username       TEXT NOT NULL DEFAULT '',
    b_password       TEXT NOT NULL DEFAULT '',
    interval_seconds INTEGER NOT NULL DEFAULT 900,
    enabled          INTEGER NOT NULL DEFAULT 1,
    state            TEXT NOT NULL DEFAULT '{}',   -- dav.State JSON
    last_run_at      TEXT NOT NULL DEFAULT '',
    last_status      TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE INDEX idx_sync_jobs_user_id ON sync_jobs (user_id);
