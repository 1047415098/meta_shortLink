-- Audio campaign links bind one playable audio novel while preserving every
-- historical legacy link that still opens the public audio archive.
ALTER TABLE audio_novels
  ADD COLUMN audio_duration_seconds integer NOT NULL DEFAULT 0
  CHECK (audio_duration_seconds BETWEEN 0 AND 86400);

-- Backfill only durations that fit the new bounded integer contract. Unknown
-- legacy values remain 0 and are shown as duration-not-collected.
UPDATE audio_novels
SET audio_duration_seconds = CASE
  WHEN audio_duration ~ '^[0-5][0-9]:[0-5][0-9]$'
    THEN split_part(audio_duration, ':', 1)::integer * 60
       + split_part(audio_duration, ':', 2)::integer
  WHEN audio_duration ~ '^[0-9]+:[0-5][0-9]:[0-5][0-9]$'
    AND split_part(audio_duration, ':', 1)::integer * 3600
      + split_part(audio_duration, ':', 2)::integer * 60
      + split_part(audio_duration, ':', 3)::integer <= 86400
    THEN split_part(audio_duration, ':', 1)::integer * 3600
       + split_part(audio_duration, ':', 2)::integer * 60
       + split_part(audio_duration, ':', 3)::integer
  ELSE 0
END
WHERE audio_duration_seconds = 0;

ALTER TABLE audio_novels DROP CONSTRAINT IF EXISTS audio_novels_audio_fields_check;
-- NOT VALID keeps an unparseable historical duration readable while every new
-- or edited Podcast must provide the complete four-field metadata group.
ALTER TABLE audio_novels ADD CONSTRAINT audio_novels_audio_fields_check CHECK (
  (audio_path = '' AND audio_duration = '' AND audio_duration_seconds = 0 AND audio_size_bytes = 0)
  OR
  (audio_path ~ '^/audio-novel-audio/[a-f0-9]{32}\.mp3$'
   AND audio_duration ~ '^([0-9]+:)?[0-5][0-9]:[0-5][0-9]$'
   AND audio_duration_seconds BETWEEN 1 AND 86400
   AND audio_size_bytes > 0)
) NOT VALID;

ALTER TABLE short_links
  ADD COLUMN audio_novel_id bigint REFERENCES audio_novels(id);

ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_novel_binding_check;
ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_content_binding_check;
ALTER TABLE short_links ADD CONSTRAINT short_links_content_binding_check CHECK (
  (product_type = 'novel' AND novel_id IS NOT NULL AND audio_novel_id IS NULL)
  OR
  (product_type = 'audio_novel' AND audio_novel_id IS NOT NULL AND novel_id IS NULL)
  OR
  (product_type NOT IN ('novel','audio_novel') AND novel_id IS NULL AND audio_novel_id IS NULL)
) NOT VALID;

ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_novel_platform_binding_check;
ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_distribution_platform_binding_check;
ALTER TABLE short_links ADD CONSTRAINT short_links_distribution_platform_binding_check CHECK (
  product_type NOT IN ('novel','audio_novel')
  OR
  (ad_platform = 'meta'
    AND meta_pixel_id IS NOT NULL
    AND meta_connection_id IS NOT NULL
    AND tiktok_pixel_id IS NULL)
  OR
  (ad_platform = 'tiktok'
    AND tiktok_pixel_id IS NOT NULL
    AND meta_pixel_id IS NULL
    AND meta_connection_id IS NULL)
) NOT VALID;

-- Audio playback thresholds may start at one second; existing text and generic
-- link validation keeps its stricter minimum in application code.
ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_time_spent_threshold_check;
ALTER TABLE short_links ADD CONSTRAINT short_links_time_spent_threshold_check CHECK (
  time_spent_threshold = 0
  OR
  (product_type = 'audio_novel' AND time_spent_threshold BETWEEN 1 AND 3600)
  OR
  (product_type <> 'audio_novel' AND time_spent_threshold BETWEEN 5 AND 3600)
);

ALTER TABLE click_events
  ADD COLUMN audio_novel_id bigint REFERENCES audio_novels(id),
  ADD COLUMN playback_seconds integer NOT NULL DEFAULT 0,
  ADD COLUMN media_consumed_seconds numeric(12,3) NOT NULL DEFAULT 0,
  ADD COLUMN playback_updated_at timestamptz,
  ADD COLUMN audio_started_at timestamptz,
  ADD COLUMN audio_qualified_at timestamptz,
  ADD COLUMN audio_completed_at timestamptz;

ALTER TABLE click_events ADD CONSTRAINT click_events_playback_seconds_check
  CHECK (playback_seconds BETWEEN 0 AND 86400);
ALTER TABLE click_events ADD CONSTRAINT click_events_media_consumed_seconds_check
  CHECK (media_consumed_seconds BETWEEN 0 AND 345600);

ALTER TABLE click_events DROP CONSTRAINT IF EXISTS click_events_time_spent_threshold_check;
ALTER TABLE click_events ADD CONSTRAINT click_events_time_spent_threshold_check
  CHECK (time_spent_threshold = 0 OR time_spent_threshold BETWEEN 1 AND 3600);

ALTER TABLE tiktok_events
  ADD COLUMN audio_novel_id bigint REFERENCES audio_novels(id);

ALTER TABLE tiktok_events ADD CONSTRAINT tiktok_events_content_binding_check
  CHECK (novel_id IS NULL OR audio_novel_id IS NULL);

ALTER TABLE tiktok_events DROP CONSTRAINT IF EXISTS tiktok_events_event_name_check;
ALTER TABLE tiktok_events ADD CONSTRAINT tiktok_events_event_name_check
  CHECK (event_name IN ('StartReading','StartListening','ViewContent','PageView'));

CREATE INDEX short_links_audio_novel_status
  ON short_links(audio_novel_id, enabled)
  WHERE product_type = 'audio_novel';
CREATE INDEX clicks_audio_link_time
  ON click_events(link_id, audio_novel_id, occurred_at DESC)
  WHERE surface = 'audio_novel';
CREATE INDEX clicks_audio_funnel
  ON click_events(link_id, audio_qualified_at, audio_completed_at)
  WHERE surface = 'audio_novel';

