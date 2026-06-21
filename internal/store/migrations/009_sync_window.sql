-- Optional CalDAV sync date window: only events whose start lies within
-- [window_start, window_end] are synced. Empty = unbounded on that side.
ALTER TABLE sync_jobs ADD COLUMN window_start TEXT NOT NULL DEFAULT '';
ALTER TABLE sync_jobs ADD COLUMN window_end TEXT NOT NULL DEFAULT '';
