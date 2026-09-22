ALTER TABLE audio_novels
  ADD COLUMN audio_path text NOT NULL DEFAULT '',
  ADD COLUMN audio_duration text NOT NULL DEFAULT '',
  ADD COLUMN audio_size_bytes bigint NOT NULL DEFAULT 0;

-- 三个字段必须同时为空或同时有效，避免前台出现无法播放的半成品 Podcast。
ALTER TABLE audio_novels ADD CONSTRAINT audio_novels_audio_fields_check CHECK (
  (audio_path = '' AND audio_duration = '' AND audio_size_bytes = 0)
  OR
  (audio_path ~ '^/audio-novel-audio/[a-f0-9]{32}\.mp3$'
   AND audio_duration ~ '^([0-9]+:)?[0-5][0-9]:[0-5][0-9]$'
   AND audio_size_bytes > 0)
);

CREATE INDEX audio_novels_public_audio_order
  ON audio_novels(published_at DESC, id DESC)
  WHERE enabled AND deleted_at IS NULL AND audio_path <> '';
