-- The cache was keyed by URL alone, so a copy fetched with credentials could be
-- served to a request that supplied none (and vice versa). Re-key it by an
-- opaque key that also covers the credentials; the URL is kept for health
-- lookups. The old rows are dropped rather than migrated: they cannot be
-- attributed to a credential set, and the cache refills on the next fetch.
DROP TABLE IF EXISTS feed_cache;

CREATE TABLE feed_cache (
    cache_key  TEXT PRIMARY KEY,
    url        TEXT NOT NULL,
    body       BLOB NOT NULL,
    etag       TEXT NOT NULL DEFAULT '',
    fetched_at TEXT NOT NULL
);

CREATE INDEX idx_feed_cache_url ON feed_cache (url);
