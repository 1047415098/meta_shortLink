-- Future visits retain structured device details parsed from User-Agent without storing the raw header.
-- Historical rows stay valid and are shown as "未采集" in the operations console.
ALTER TABLE click_events
    ADD COLUMN IF NOT EXISTS device_model text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS os_version text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS browser_version text NOT NULL DEFAULT '';
