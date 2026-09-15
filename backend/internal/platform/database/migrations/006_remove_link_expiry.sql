-- Preserve the previous state of links that already reached their deadline,
-- then remove scheduled expiry so availability is controlled only by enabled.
UPDATE short_links SET enabled=false WHERE expires_at IS NOT NULL AND expires_at<=now();
ALTER TABLE short_links DROP COLUMN expires_at;
