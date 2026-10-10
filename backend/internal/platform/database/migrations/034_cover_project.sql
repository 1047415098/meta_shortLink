-- Cover campaigns are a standalone product and never bind novels or audio content.
ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_product_type_check;
ALTER TABLE short_links ADD CONSTRAINT short_links_product_type_check
  CHECK (product_type IN ('legacy', 'short_link', 'audio_novel', 'novel', 'cover'));

ALTER TABLE click_events DROP CONSTRAINT IF EXISTS click_events_surface_check;
ALTER TABLE click_events ADD CONSTRAINT click_events_surface_check
  CHECK (surface IN ('short_link', 'audio_novel', 'novel', 'cover'));

ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_cover_platform_binding_check;
ALTER TABLE short_links ADD CONSTRAINT short_links_cover_platform_binding_check
  CHECK (product_type <> 'cover' OR
    (novel_id IS NULL AND audio_novel_id IS NULL AND
      ((ad_platform='meta' AND meta_pixel_id IS NOT NULL AND meta_connection_id IS NOT NULL AND tiktok_pixel_id IS NULL) OR
       (ad_platform='tiktok' AND tiktok_pixel_id IS NOT NULL AND meta_pixel_id IS NULL AND meta_connection_id IS NULL))))
  NOT VALID;

-- Free-novel links return to the single established countdown experience.
UPDATE short_links SET startup_theme='countdown' WHERE product_type='novel';

CREATE INDEX IF NOT EXISTS short_links_cover_project ON short_links(product_type,id DESC)
  WHERE product_type='cover';
CREATE INDEX IF NOT EXISTS clicks_cover_link_time ON click_events(link_id,occurred_at DESC)
  WHERE surface='cover';
