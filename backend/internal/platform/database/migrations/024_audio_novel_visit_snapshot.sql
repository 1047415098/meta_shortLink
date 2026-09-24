-- Freeze the operator-facing title used by advertising payloads so later
-- editorial changes cannot rewrite the meaning of a historical visit.
ALTER TABLE click_events
  ADD COLUMN audio_novel_title text NOT NULL DEFAULT '';

UPDATE click_events e
SET audio_novel_title=n.title
FROM audio_novels n
WHERE e.audio_novel_id=n.id AND e.audio_novel_title='';
