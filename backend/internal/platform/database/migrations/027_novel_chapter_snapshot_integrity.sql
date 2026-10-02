-- PostgreSQL's default MATCH SIMPLE foreign key permits one NULL column, so
-- keep the optional legacy state and the complete frozen identity as the only states.
-- This is separate from 026 because existing environments may have already
-- recorded that migration before this additional defensive constraint existed.
ALTER TABLE click_events
  DROP CONSTRAINT IF EXISTS click_events_entry_chapter_snapshot_check;

ALTER TABLE click_events
  ADD CONSTRAINT click_events_entry_chapter_snapshot_check
  CHECK (
    (entry_chapter_id IS NULL AND entry_chapter_number IS NULL AND entry_chapter_title = '')
    OR
    (entry_chapter_id IS NOT NULL AND novel_id IS NOT NULL AND entry_chapter_number IS NOT NULL)
  );
