-- Rename the existing table in place so content IDs, timestamps and identity state remain unchanged.
ALTER TABLE novels RENAME TO audio_novels;
ALTER SEQUENCE novels_id_seq RENAME TO audio_novels_id_seq;
ALTER INDEX novels_active_slug_unique RENAME TO audio_novels_active_slug_unique;
ALTER INDEX novels_one_effective_featured RENAME TO audio_novels_one_effective_featured;
ALTER INDEX novels_public_order RENAME TO audio_novels_public_order;

-- Remove the old allowed-value constraint before translating historical visits to the new surface.
ALTER TABLE click_events DROP CONSTRAINT click_events_surface_check;
UPDATE click_events SET surface = 'audio_novel' WHERE surface = 'novel';
ALTER TABLE click_events ADD CONSTRAINT click_events_surface_check
  CHECK (surface IN ('short_link', 'audio_novel'));
