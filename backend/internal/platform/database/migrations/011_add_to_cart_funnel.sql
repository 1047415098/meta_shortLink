-- Align every active and frozen consultation rule with Meta's standard
-- AddToCart event while leaving already-created meta_events unchanged.
ALTER TABLE meta_pixels
  DROP CONSTRAINT IF EXISTS meta_pixels_manual_event_name_check;

UPDATE meta_pixels
SET manual_event_name = 'AddToCart';

ALTER TABLE meta_pixels
  ALTER COLUMN manual_event_name SET DEFAULT 'AddToCart',
  ADD CONSTRAINT meta_pixels_manual_event_name_check
    CHECK (manual_event_name = 'AddToCart');

-- Visits created before this deployment can still receive a first manual
-- action, so their frozen rule is upgraded to the newly approved event name.
UPDATE click_events
SET meta_manual_event_name = 'AddToCart';

ALTER TABLE click_events
  ALTER COLUMN meta_manual_event_name SET DEFAULT 'AddToCart';
