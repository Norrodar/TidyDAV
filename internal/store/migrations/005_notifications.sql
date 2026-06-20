-- Per-feed notification configuration and the de-duplication ledger that keeps
-- a matched event from notifying on every scheduled run.
ALTER TABLE feeds ADD COLUMN notifications TEXT NOT NULL DEFAULT '{}';

CREATE TABLE notified (
    feed_id     TEXT NOT NULL,
    event_key   TEXT NOT NULL,
    notified_at TEXT NOT NULL,
    PRIMARY KEY (feed_id, event_key)
);
