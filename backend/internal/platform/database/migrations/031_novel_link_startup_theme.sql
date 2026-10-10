-- Each novel campaign link independently selects its first-screen presentation.
ALTER TABLE short_links
  ADD COLUMN IF NOT EXISTS startup_theme text NOT NULL DEFAULT 'countdown';

-- Reader visits freeze the selected presentation so later admin edits affect only new visits.
ALTER TABLE click_events
  ADD COLUMN IF NOT EXISTS startup_theme text NOT NULL DEFAULT 'countdown';

ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_startup_theme_check;
ALTER TABLE short_links
  ADD CONSTRAINT short_links_startup_theme_check
  CHECK (startup_theme IN ('countdown','cover_wall'));

ALTER TABLE click_events DROP CONSTRAINT IF EXISTS click_events_startup_theme_check;
ALTER TABLE click_events
  ADD CONSTRAINT click_events_startup_theme_check
  CHECK (startup_theme IN ('countdown','cover_wall'));
