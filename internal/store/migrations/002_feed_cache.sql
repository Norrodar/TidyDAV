-- Cache of upstream ICS feeds: the last good copy per source URL, so a dead
-- upstream can still be served.
CREATE TABLE feed_cache (
    url        TEXT PRIMARY KEY,
    body       BLOB NOT NULL,
    etag       TEXT NOT NULL DEFAULT '',
    fetched_at TEXT NOT NULL
);
