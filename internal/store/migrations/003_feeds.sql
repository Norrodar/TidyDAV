-- Output feed definitions: each is a transformed ICS endpoint served at
-- /ics/<secret>, owned by a user.
CREATE TABLE feeds (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    secret          TEXT NOT NULL UNIQUE,
    sources         TEXT NOT NULL DEFAULT '[]', -- JSON array of {url,username,password}
    rules           TEXT NOT NULL DEFAULT '[]', -- JSON array of pipeline rule configs
    ttl_seconds     INTEGER NOT NULL DEFAULT 900,
    basic_auth_user TEXT NOT NULL DEFAULT '',
    basic_auth_hash TEXT NOT NULL DEFAULT '', -- bcrypt; empty means no basic auth
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE INDEX idx_feeds_user_id ON feeds (user_id);
