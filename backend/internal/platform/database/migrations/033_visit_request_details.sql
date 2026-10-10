-- Admin visit diagnostics keep the allowlisted raw request context requested by operations.
-- Authentication, Cookie and arbitrary proxy headers are deliberately excluded from this snapshot.
ALTER TABLE click_events
    ADD COLUMN IF NOT EXISTS client_ip text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS user_agent text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS request_url text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS referrer_url text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS accept_language text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS client_hints jsonb NOT NULL DEFAULT '{}';
