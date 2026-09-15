ALTER TABLE meta_connections ADD COLUMN read_token_expires_at timestamptz, ADD COLUMN read_token_updated_at timestamptz, ADD COLUMN read_credential_status text NOT NULL DEFAULT 'unverified';
UPDATE meta_connections SET read_credential_status=CASE WHEN validated_at IS NOT NULL THEN 'valid' ELSE 'unverified' END;
CREATE TABLE meta_pixels (
 id bigserial PRIMARY KEY, connection_id bigint NOT NULL REFERENCES meta_connections(id), name text NOT NULL,
 pixel_id text NOT NULL, enabled boolean NOT NULL DEFAULT false, pageview_enabled boolean NOT NULL DEFAULT false,
 manual_enabled boolean NOT NULL DEFAULT true, manual_event_name text NOT NULL DEFAULT 'WhatsAppConsultClick' CHECK(manual_event_name IN ('WhatsAppConsultClick','Contact')),
 capi_token_cipher text NOT NULL DEFAULT '', token_expires_at timestamptz, credential_status text NOT NULL DEFAULT 'unverified',
 validated_at timestamptz, last_error text NOT NULL DEFAULT '', updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(connection_id,pixel_id), UNIQUE(connection_id,id)
);
INSERT INTO meta_pixels(connection_id,name,pixel_id,enabled,capi_token_cipher) SELECT id,name||' · 默认 Pixel',pixel_id,capi_token_cipher<>'',capi_token_cipher FROM meta_connections WHERE pixel_id<>'';
ALTER TABLE short_links ADD COLUMN meta_pixel_id bigint;
ALTER TABLE short_links ADD CONSTRAINT short_links_meta_pixel_account_fk FOREIGN KEY(meta_connection_id,meta_pixel_id) REFERENCES meta_pixels(connection_id,id);
ALTER TABLE short_links ADD CONSTRAINT short_links_meta_pixel_requires_account CHECK(meta_pixel_id IS NULL OR meta_connection_id IS NOT NULL);
ALTER TABLE click_events ADD COLUMN meta_pixel_id bigint REFERENCES meta_pixels(id), ADD COLUMN meta_pageview_enabled boolean NOT NULL DEFAULT false, ADD COLUMN meta_manual_enabled boolean NOT NULL DEFAULT true, ADD COLUMN meta_manual_event_name text NOT NULL DEFAULT 'WhatsAppConsultClick', ADD COLUMN pageview_reported_at timestamptz;
UPDATE click_events e SET meta_pixel_id=p.id FROM meta_pixels p JOIN meta_connections c ON p.connection_id=c.id AND p.pixel_id=c.pixel_id WHERE e.meta_connection_id=c.id;
ALTER TABLE meta_events ADD COLUMN pixel_record_id bigint REFERENCES meta_pixels(id);
UPDATE meta_events e SET pixel_record_id=p.id FROM meta_pixels p WHERE p.connection_id=e.connection_id AND p.pixel_id=e.pixel_id;
CREATE INDEX meta_events_pixel ON meta_events(pixel_record_id,created_at DESC);
