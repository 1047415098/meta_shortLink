-- Keep every existing and future account on the application-owned Graph API
-- version so a database row cannot silently select a different CAPI contract.
UPDATE meta_connections SET api_version='v26.0' WHERE api_version<>'v26.0';
ALTER TABLE meta_connections ALTER COLUMN api_version SET DEFAULT 'v26.0';
