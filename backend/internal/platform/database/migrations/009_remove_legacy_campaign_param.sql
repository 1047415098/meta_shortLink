-- Canonical Meta links now provide campaign_id, adset_id and ad_id explicitly.
-- Removing the flag prevents utm_content={{ad.id}} from being misread as a campaign ID.
ALTER TABLE short_links DROP COLUMN IF EXISTS legacy_campaign_param;
