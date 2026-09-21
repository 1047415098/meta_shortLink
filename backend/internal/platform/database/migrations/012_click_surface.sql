-- 区分原短链接与小说站入口；历史记录统一归入原短链接。
ALTER TABLE click_events
ADD COLUMN IF NOT EXISTS surface text NOT NULL DEFAULT 'short_link';

ALTER TABLE click_events DROP CONSTRAINT IF EXISTS click_events_surface_check;
ALTER TABLE click_events ADD CONSTRAINT click_events_surface_check
CHECK (surface IN ('short_link', 'novel'));

CREATE INDEX IF NOT EXISTS clicks_surface_time
ON click_events(surface, occurred_at DESC);
