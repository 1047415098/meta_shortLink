-- Keep the audio content owner on the Meta outbox row itself. Visit details
-- have a shorter retention window, while delivery diagnostics must still be
-- able to identify the audio novel after those details are removed.
ALTER TABLE meta_events
  ADD COLUMN audio_novel_id bigint REFERENCES audio_novels(id);

-- Existing audio events already point at their originating visit, so freeze
-- that visit's content binding before later retention jobs can remove it.
UPDATE meta_events e
SET audio_novel_id=c.audio_novel_id
FROM click_events c
WHERE c.id=e.visit_id
  AND e.audio_novel_id IS NULL
  AND c.audio_novel_id IS NOT NULL;

-- Audio statistics and diagnostics repeatedly resolve the latest delivery
-- state for one visit/event pair. Test events never participate in that path.
CREATE INDEX meta_events_visit_event_updated
  ON meta_events(visit_id,event_name,updated_at DESC)
  WHERE NOT is_test;
