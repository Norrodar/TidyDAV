-- Subscriber stats: when a calendar client last fetched /ics/<secret> and how
-- often it has been served overall.
ALTER TABLE feeds ADD COLUMN last_served_at TEXT NOT NULL DEFAULT '';
ALTER TABLE feeds ADD COLUMN serve_count INTEGER NOT NULL DEFAULT 0;
