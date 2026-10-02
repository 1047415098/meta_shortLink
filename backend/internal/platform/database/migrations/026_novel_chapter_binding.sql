-- Historical novel links intentionally keep a NULL entry chapter and continue
-- opening the introduction page. New links are validated by the application.
ALTER TABLE short_links
  ADD COLUMN entry_chapter_id bigint REFERENCES novel_chapters(id);

-- Composite ownership constraints make a cross-novel chapter binding
-- impossible even if a future write path forgets application validation.
ALTER TABLE novel_chapters
  ADD CONSTRAINT novel_chapters_id_novel_unique UNIQUE (id, novel_id);
ALTER TABLE short_links
  ADD CONSTRAINT short_links_entry_chapter_owner_fk
  FOREIGN KEY (entry_chapter_id, novel_id) REFERENCES novel_chapters(id, novel_id);
ALTER TABLE short_links
  ADD CONSTRAINT short_links_entry_chapter_product_check
  CHECK (entry_chapter_id IS NULL OR product_type = 'novel');

-- A visit owns an immutable chapter snapshot so later chapter edits do not
-- rewrite routing, attribution, or historical statistics.
ALTER TABLE click_events
  ADD COLUMN entry_chapter_id bigint REFERENCES novel_chapters(id),
  ADD COLUMN entry_chapter_number integer,
  ADD COLUMN entry_chapter_title text NOT NULL DEFAULT '';

ALTER TABLE click_events
  ADD CONSTRAINT click_events_entry_chapter_owner_fk
  FOREIGN KEY (entry_chapter_id, novel_id) REFERENCES novel_chapters(id, novel_id);

ALTER TABLE click_events DROP CONSTRAINT IF EXISTS click_events_entry_chapter_number_check;
ALTER TABLE click_events ADD CONSTRAINT click_events_entry_chapter_number_check
  CHECK (entry_chapter_number IS NULL OR entry_chapter_number > 0);

CREATE INDEX short_links_entry_chapter_lookup
  ON short_links(entry_chapter_id, id)
  WHERE product_type = 'novel' AND entry_chapter_id IS NOT NULL;
