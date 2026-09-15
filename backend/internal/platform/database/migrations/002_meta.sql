CREATE TABLE meta_connections (
 id bigserial PRIMARY KEY, name text NOT NULL, account_id text UNIQUE NOT NULL, pixel_id text NOT NULL DEFAULT '',
 api_version text NOT NULL DEFAULT 'v26.0', capi_enabled boolean NOT NULL DEFAULT false, insights_enabled boolean NOT NULL DEFAULT false,
 capi_token_cipher text NOT NULL DEFAULT '', read_token_cipher text NOT NULL DEFAULT '',
 conversion_action_type text NOT NULL DEFAULT '', report_time text NOT NULL DEFAULT 'impression',
 backfill_days integer NOT NULL DEFAULT 28 CHECK(backfill_days BETWEEN 1 AND 90),
 currency text NOT NULL DEFAULT '', timezone text NOT NULL DEFAULT '',
 validated_at timestamptz, last_sync_at timestamptz, last_error text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE short_links ADD COLUMN meta_connection_id bigint REFERENCES meta_connections(id);
ALTER TABLE short_links ADD COLUMN attribution_mode text NOT NULL DEFAULT 'bound' CHECK(attribution_mode IN ('bound','dynamic'));
ALTER TABLE short_links ADD COLUMN legacy_campaign_param boolean NOT NULL DEFAULT false;
ALTER TABLE click_events ADD COLUMN meta_connection_id bigint REFERENCES meta_connections(id);
ALTER TABLE click_events ADD COLUMN meta_account_id text NOT NULL DEFAULT '';
ALTER TABLE click_events ADD COLUMN meta_measurement boolean NOT NULL DEFAULT false;
CREATE INDEX clicks_meta_visit ON click_events(meta_connection_id,occurred_at);
CREATE INDEX clicks_meta_manual ON click_events(meta_connection_id,whatsapp_clicked_at) WHERE whatsapp_clicked_at IS NOT NULL;
CREATE INDEX clicks_meta_auto ON click_events(meta_connection_id,auto_redirected_at) WHERE auto_redirected_at IS NOT NULL;
CREATE TABLE meta_events (
 id text PRIMARY KEY, connection_id bigint NOT NULL REFERENCES meta_connections(id), visit_id text NOT NULL DEFAULT '',
 event_name text NOT NULL, event_time timestamptz NOT NULL, pixel_id text NOT NULL,
 is_test boolean NOT NULL DEFAULT false, payload_cipher text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'pending', attempts integer NOT NULL DEFAULT 0, retry_count integer NOT NULL DEFAULT 0,
 next_attempt_at timestamptz NOT NULL DEFAULT now(), locked_until timestamptz, lock_token text NOT NULL DEFAULT '',
 events_received integer NOT NULL DEFAULT 0, fbtrace_id text NOT NULL DEFAULT '', last_error text NOT NULL DEFAULT '',
 response_messages jsonb NOT NULL DEFAULT '[]', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX meta_events_work ON meta_events(next_attempt_at) WHERE status IN ('pending','retry','processing');
CREATE INDEX meta_events_list ON meta_events(connection_id,created_at DESC);
CREATE TABLE meta_sync_jobs (
 id bigserial PRIMARY KEY, connection_id bigint NOT NULL REFERENCES meta_connections(id), profile text NOT NULL,
 start_date date NOT NULL, end_date date NOT NULL, status text NOT NULL DEFAULT 'queued', reason text NOT NULL,
 profile_config jsonb NOT NULL DEFAULT '{}', report_run_id text NOT NULL DEFAULT '', async_started_at timestamptz,
 schedule_key text UNIQUE, attempts integer NOT NULL DEFAULT 0, retry_count integer NOT NULL DEFAULT 0,
 next_attempt_at timestamptz NOT NULL DEFAULT now(), locked_until timestamptz, lock_token text NOT NULL DEFAULT '',
 rows_count integer NOT NULL DEFAULT 0, last_error text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(),
 started_at timestamptz, finished_at timestamptz
);
CREATE INDEX meta_sync_work ON meta_sync_jobs(next_attempt_at) WHERE status IN ('queued','running');
CREATE UNIQUE INDEX meta_sync_one_running ON meta_sync_jobs(connection_id) WHERE status='running';
CREATE INDEX meta_sync_list ON meta_sync_jobs(connection_id,created_at DESC);
CREATE TABLE meta_ad_entities (
 connection_id bigint NOT NULL REFERENCES meta_connections(id), ad_id text NOT NULL, ad_name text NOT NULL,
 campaign_id text NOT NULL, campaign_name text NOT NULL, adset_id text NOT NULL, adset_name text NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(connection_id,ad_id)
);
CREATE TABLE meta_insights_daily (
 connection_id bigint NOT NULL REFERENCES meta_connections(id), profile text NOT NULL, date date NOT NULL,
 ad_id text NOT NULL, ad_name text NOT NULL, campaign_id text NOT NULL, campaign_name text NOT NULL, adset_id text NOT NULL, adset_name text NOT NULL,
 currency text NOT NULL, timezone text NOT NULL, spend numeric(20,6) NOT NULL, impressions bigint NOT NULL,
 clicks bigint NOT NULL, link_clicks bigint NOT NULL, outbound_clicks bigint NOT NULL,
 meta_consultations numeric(20,6), actions jsonb NOT NULL DEFAULT '[]', synced_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(connection_id,profile,date,ad_id)
);
CREATE TABLE meta_sync_coverage (
 connection_id bigint NOT NULL REFERENCES meta_connections(id), profile text NOT NULL, date date NOT NULL,
 synced_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(connection_id,profile,date)
);
