-- TikTok configuration stays independent from Meta so existing delivery and
-- credentials can continue unchanged during a gradual rollout.
CREATE TABLE tiktok_connections (
  id bigserial PRIMARY KEY,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  access_token_cipher text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  credential_status text NOT NULL DEFAULT 'unverified'
    CHECK (credential_status IN ('unverified','valid','invalid','error')),
  validated_at timestamptz,
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tiktok_pixels (
  id bigserial PRIMARY KEY,
  connection_id bigint NOT NULL REFERENCES tiktok_connections(id),
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  pixel_code text NOT NULL UNIQUE CHECK (char_length(pixel_code) BETWEEN 5 AND 64),
  test_event_code text NOT NULL DEFAULT '' CHECK (char_length(test_event_code) <= 120),
  enabled boolean NOT NULL DEFAULT true,
  validated_at timestamptz,
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE short_links
  ADD COLUMN ad_platform text NOT NULL DEFAULT 'meta',
  ADD COLUMN tiktok_pixel_id bigint REFERENCES tiktok_pixels(id);

ALTER TABLE short_links ADD CONSTRAINT short_links_ad_platform_check
  CHECK (ad_platform IN ('meta','tiktok'));
-- NOT VALID preserves manually-created historical rows while PostgreSQL still
-- enforces the platform/Pixel contract for every new or edited novel link.
ALTER TABLE short_links ADD CONSTRAINT short_links_novel_platform_binding_check
  CHECK (product_type <> 'novel' OR
    (ad_platform='meta' AND meta_pixel_id IS NOT NULL AND meta_connection_id IS NOT NULL AND tiktok_pixel_id IS NULL) OR
    (ad_platform='tiktok' AND tiktok_pixel_id IS NOT NULL AND meta_pixel_id IS NULL AND meta_connection_id IS NULL))
  NOT VALID;

ALTER TABLE click_events
  ADD COLUMN ad_platform text NOT NULL DEFAULT 'meta',
  ADD COLUMN tiktok_pixel_id bigint REFERENCES tiktok_pixels(id),
  ADD COLUMN tiktok_pixel_code text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_ttclid text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_ttp text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_adgroup_id text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_creative_id text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_ad_id_v2 text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_placement text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_context_cipher text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_start_reading_at timestamptz,
  ADD COLUMN tiktok_view_content_at timestamptz;

-- Logical visit/link IDs deliberately have no foreign keys: the existing
-- retention job may remove expired visit details without erasing delivery logs.
CREATE TABLE tiktok_events (
  id text PRIMARY KEY,
  visit_id text NOT NULL,
  link_id bigint NOT NULL,
  novel_id bigint,
  connection_id bigint NOT NULL REFERENCES tiktok_connections(id),
  pixel_record_id bigint NOT NULL REFERENCES tiktok_pixels(id),
  pixel_code text NOT NULL,
  event_name text NOT NULL CHECK (event_name IN ('StartReading','ViewContent','PageView')),
  event_id text NOT NULL,
  event_time timestamptz NOT NULL,
  payload_cipher text NOT NULL,
  is_test boolean NOT NULL DEFAULT false,
  status text NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','sending','accepted','retry','failed')),
  attempts integer NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  locked_until timestamptz,
  lock_token text NOT NULL DEFAULT '',
  http_status integer NOT NULL DEFAULT 0,
  business_code bigint NOT NULL DEFAULT 0,
  request_id text NOT NULL DEFAULT '',
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(pixel_code,event_name,event_id)
);

CREATE INDEX tiktok_events_delivery
  ON tiktok_events(status,next_attempt_at,event_time)
  WHERE status IN ('pending','retry','sending');
CREATE INDEX tiktok_events_visit ON tiktok_events(visit_id,event_name);
CREATE INDEX click_events_tiktok_attribution
  ON click_events(link_id,tiktok_ad_id_v2,tiktok_adgroup_id,occurred_at DESC)
  WHERE ad_platform='tiktok' AND surface='novel';
