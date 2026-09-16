-- New database-created links follow the same multi-ad default as the API and admin form.
ALTER TABLE short_links ALTER COLUMN attribution_mode SET DEFAULT 'dynamic';
