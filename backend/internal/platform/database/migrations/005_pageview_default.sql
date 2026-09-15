-- Existing active Pixel targets join the requested landing PageView funnel;
-- administrators may disable PageView again from the Pixel editor if needed.
ALTER TABLE meta_pixels ALTER COLUMN pageview_enabled SET DEFAULT true;
UPDATE meta_pixels SET pageview_enabled=true WHERE enabled AND NOT pageview_enabled;
