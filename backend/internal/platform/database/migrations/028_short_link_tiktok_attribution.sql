-- Ordinary WhatsApp short links now follow the same mutually-exclusive
-- Meta/TikTok binding contract as novel distribution links. NOT VALID keeps
-- historical rows readable while enforcing the rule for every new or edited row.
ALTER TABLE short_links ADD CONSTRAINT short_links_platform_binding_check
  CHECK (product_type <> 'short_link' OR
    (ad_platform='meta' AND meta_pixel_id IS NOT NULL AND meta_connection_id IS NOT NULL AND tiktok_pixel_id IS NULL) OR
    (ad_platform='tiktok' AND tiktok_pixel_id IS NOT NULL AND meta_pixel_id IS NULL AND meta_connection_id IS NULL))
  NOT VALID;

-- Contact is the standard TikTok event used by manual and automatic WhatsApp
-- handoffs. Their visit-scoped event IDs remain distinct for deduplication.
ALTER TABLE tiktok_events DROP CONSTRAINT IF EXISTS tiktok_events_event_name_check;
ALTER TABLE tiktok_events ADD CONSTRAINT tiktok_events_event_name_check
  CHECK (event_name IN ('StartReading','StartListening','ViewContent','PageView','Contact'));

CREATE INDEX click_events_tiktok_short_link_attribution
  ON click_events(link_id,tiktok_ad_id_v2,occurred_at DESC)
  WHERE ad_platform='tiktok' AND surface='short_link';
