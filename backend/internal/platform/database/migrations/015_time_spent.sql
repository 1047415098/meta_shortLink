-- TimeSpent is configured by code and frozen on each visit so later edits do
-- not rewrite the contract of a page that is already open in a browser.
ALTER TABLE short_links
  ADD COLUMN IF NOT EXISTS time_spent_threshold integer NOT NULL DEFAULT 0;

ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_time_spent_threshold_check;
ALTER TABLE short_links ADD CONSTRAINT short_links_time_spent_threshold_check
  CHECK (time_spent_threshold = 0 OR time_spent_threshold BETWEEN 5 AND 3600);

ALTER TABLE click_events
  ADD COLUMN IF NOT EXISTS time_spent_threshold integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS time_spent_reported_at timestamptz;

ALTER TABLE click_events DROP CONSTRAINT IF EXISTS click_events_time_spent_threshold_check;
ALTER TABLE click_events ADD CONSTRAINT click_events_time_spent_threshold_check
  CHECK (time_spent_threshold = 0 OR time_spent_threshold BETWEEN 5 AND 3600);

CREATE INDEX IF NOT EXISTS clicks_time_spent
  ON click_events(link_id,time_spent_reported_at)
  WHERE time_spent_reported_at IS NOT NULL;
