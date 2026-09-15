CREATE TABLE IF NOT EXISTS short_links (
 id bigserial PRIMARY KEY, code text UNIQUE NOT NULL, name text NOT NULL,
 target_url text NOT NULL, enabled boolean NOT NULL DEFAULT true,
 campaign_id text NOT NULL DEFAULT '', adset_id text NOT NULL DEFAULT '', ad_id text NOT NULL DEFAULT '',
 channel text NOT NULL DEFAULT 'facebook', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS admin_sessions (
 token_hash text PRIMARY KEY, expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS click_events (
 id text PRIMARY KEY, occurred_at timestamptz NOT NULL DEFAULT now(), link_id bigint NOT NULL REFERENCES short_links(id),
 visitor_id text, cookie_status text NOT NULL, method text NOT NULL,
 target_url text NOT NULL, status integer NOT NULL DEFAULT 302,
 device text NOT NULL, os text NOT NULL, browser text NOT NULL,
 country text NOT NULL, region text NOT NULL, city text NOT NULL,
 source text NOT NULL, campaign_id text NOT NULL, adset_id text NOT NULL, ad_id text NOT NULL,
 referrer text NOT NULL, parameters jsonb NOT NULL DEFAULT '{}', attribution_conflict boolean NOT NULL DEFAULT false,
 classification text NOT NULL, reason text NOT NULL, rule_version text NOT NULL DEFAULT '1'
);
CREATE INDEX IF NOT EXISTS clicks_link_time ON click_events(link_id,occurred_at);
CREATE INDEX IF NOT EXISTS clicks_time ON click_events(occurred_at);
CREATE INDEX IF NOT EXISTS clicks_ad_time ON click_events(ad_id,occurred_at);
CREATE INDEX IF NOT EXISTS clicks_visitor_time ON click_events(visitor_id,occurred_at);
CREATE TABLE IF NOT EXISTS ad_spend_daily (
 date date NOT NULL, ad_id text NOT NULL, amount numeric(14,4) NOT NULL CHECK(amount>=0),
 currency text NOT NULL, time_zone text NOT NULL,
 PRIMARY KEY(date,ad_id,currency,time_zone)
);
CREATE TABLE IF NOT EXISTS audit_logs (
 id bigserial PRIMARY KEY, occurred_at timestamptz NOT NULL DEFAULT now(), actor text NOT NULL,
 action text NOT NULL, detail jsonb NOT NULL
);
CREATE TABLE IF NOT EXISTS daily_totals (
 date date PRIMARY KEY, total bigint NOT NULL, filtered bigint NOT NULL, bot bigint NOT NULL,
 suspicious bigint NOT NULL, updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO short_links(code,name,target_url,channel)
SELECT 'hello','WhatsApp 默认入口','https://wa.me/13365661092','facebook'
WHERE NOT EXISTS(SELECT 1 FROM short_links);

ALTER TABLE short_links ADD COLUMN IF NOT EXISTS expires_at timestamptz;

-- Additive migration: historical links retain direct redirects and existing totals.
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS mode text NOT NULL DEFAULT 'redirect';
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS landing_brand text NOT NULL DEFAULT '';
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS landing_title text NOT NULL DEFAULT '';
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS landing_description text NOT NULL DEFAULT '';
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS landing_details text NOT NULL DEFAULT '';
ALTER TABLE click_events ADD COLUMN IF NOT EXISTS event_type text NOT NULL DEFAULT 'redirect';
ALTER TABLE click_events ADD COLUMN IF NOT EXISTS whatsapp_clicked_at timestamptz;

CREATE TABLE IF NOT EXISTS request_logs (
 id text PRIMARY KEY, occurred_at timestamptz NOT NULL DEFAULT now(),
 method text NOT NULL, path text NOT NULL, client_ip text NOT NULL,
 status integer NOT NULL, duration_ms double precision NOT NULL,
 request_headers jsonb NOT NULL, query jsonb NOT NULL, request_body jsonb NOT NULL,
 response_headers jsonb NOT NULL, response_body jsonb NOT NULL,
 request_bytes bigint NOT NULL, response_bytes bigint NOT NULL
);
CREATE INDEX IF NOT EXISTS request_logs_time ON request_logs(occurred_at DESC);
CREATE INDEX IF NOT EXISTS request_logs_status_time ON request_logs(status,occurred_at DESC);

ALTER TABLE request_logs ADD COLUMN IF NOT EXISTS is_visitor boolean NOT NULL DEFAULT false;
-- Retain identifiable historical visitor requests while hiding admin/resource logs.
UPDATE request_logs r SET is_visitor=true FROM short_links l
WHERE NOT r.is_visitor AND (r.path='/'||l.code OR r.path='/'||l.code||'/contact');
CREATE INDEX IF NOT EXISTS request_logs_visitor_time ON request_logs(occurred_at DESC) WHERE is_visitor;

ALTER TABLE short_links ADD COLUMN IF NOT EXISTS landing_delay integer NOT NULL DEFAULT 0;
ALTER TABLE click_events ADD COLUMN IF NOT EXISTS auto_redirected_at timestamptz;
