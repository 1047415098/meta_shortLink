-- Split automatic redirects from deliberate consultations while preserving each
-- existing Pixel's previous delivery behavior.
ALTER TABLE meta_pixels
  ADD COLUMN auto_enabled boolean NOT NULL DEFAULT true;

UPDATE meta_pixels
SET auto_enabled = manual_enabled,
    manual_event_name = 'Contact',
    token_expires_at = NULL,
    credential_status = CASE
      WHEN credential_status = 'expired' THEN 'unverified'
      ELSE credential_status
    END;

-- Freeze the new automatic rule on every visit just like the existing manual
-- and PageView rules, so later Pixel edits do not alter historical visits.
ALTER TABLE click_events
  ADD COLUMN meta_auto_enabled boolean NOT NULL DEFAULT false;

UPDATE click_events
SET meta_auto_enabled = meta_manual_enabled,
    meta_manual_event_name = 'Contact';
