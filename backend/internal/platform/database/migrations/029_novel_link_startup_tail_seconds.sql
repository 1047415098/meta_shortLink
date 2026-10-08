-- Each novel campaign link controls only the final 10% of its visual first-load progress.
ALTER TABLE short_links
  ADD COLUMN IF NOT EXISTS startup_tail_seconds integer NOT NULL DEFAULT 5;

ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_startup_tail_seconds_check;
ALTER TABLE short_links
  ADD CONSTRAINT short_links_startup_tail_seconds_check
  CHECK (startup_tail_seconds BETWEEN 1 AND 60);
