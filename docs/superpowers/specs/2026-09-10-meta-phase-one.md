# Meta phase one — approved scope

User approved implementation after the API analysis and confirmed multiple connections: one short link selects one connection (ad account + Pixel); many links may share it. Implement consultation reporting and daily per-ad Insights, with campaign/adset/ad drilldown. No orders, payments, extensions or Git commits.

## Architecture

Keep Vue 3 JavaScript / Element Plus frontend pages and separate landing app, served by the same Gin service. Add backend/internal/modules/meta for configuration, encrypted credentials, fixed-host Graph client, durable consultation outbox and Insights jobs/reports. PostgreSQL is the queue. Existing statistics and CSV spend remain separate; never sum imported spend with Meta spend.

## Contracts and invariants

- Manual consultation only; per-visit dedupe remains. Automatic redirects remain internal.
- Store visit connection/account snapshots. Link attribution mode bound (existing behavior) or dynamic; explicit legacy campaign-in-utm_content opt-in. Preserve raw selected name/platform/placement fields and no public Meta tokens.
- Connection account_id and pixel_id immutable after creation; one configuration per account in this phase. API version configurable, default v26.0. Currency and timezone fetched by validating read access. Tokens write-only, AES-GCM encrypted using a purpose-separated APP_SECRET-derived key; never returned/logged.
- Connection capi_enabled / insights_enabled default false; real external operations only after administrator supplies credentials and enables them. Event name fixed WhatsAppConsultClick. Optional conversion_action_type maps verified Insights output; empty mapping means unknown, not zero.
- Consultation timestamp and encrypted outbox payload committed in the same transaction. Stable visit-derived event ID, at most one manual event. Raw matching information is limited to the submitted request and valid own-site fbc/fbp or actual stored fbclid. No fabricated PII. Do not read Facebook/WhatsApp login cookies.
- Jobs use durable claims, leases, fencing tokens, retry/backoff and error records. Event attempts do not block redirect. Expired events cannot be retried. Successful payloads cleared; expired matching context removed. Test events require test_event_code and are distinct from live events.
- HTTP client calls only graph.facebook.com in production, uses Authorization header, disallows redirects, bounds response size/time and follows pagination cursors without trusting arbitrary next URLs. Test fake HTTP server substitutes the network boundary only.
- Insights uses level=ad, time_increment=1 and explicit dates/report_time/unified attribution setting. Fetch account currency/timezone. All pages complete before atomic date-window replacement plus coverage records; retain previous snapshots on failure. Automatic hourly recent sync, daily rolling backfill and manual history jobs.
- Reporting scoped to one connection, currency and report profile; daily facts don't overwrite another profile. No adding reach or daily unique counts to totals. Compute rates from summed numerators/denominators. Internal clicks remain distinct from Meta-accepted and Meta-attributed conversions. No internal ROAS.
- Public landing adds a compact measurement notice only when linked CAPI is enabled. No browser Pixel script or automatic events sent in phase one.

## Admin API contract (all existing auth/CSRF guards apply)

Base /api/v1/meta.
- GET /connections -> array. POST /connections, PATCH /connections/:id -> connection.
- Connection fields: id, name, account_id, pixel_id, api_version, capi_enabled, insights_enabled, conversion_action_type, report_time (impression|conversion), backfill_days (1..90), currency, timezone, has_capi_token, has_read_token, validated_at, last_sync_at, last_error, created_at. Write-only capi_token/read_token; omitted or blank preserves stored token on edit. Separate clear_capi_token/clear_read_token flags, cannot clear when its feature enabled.
- POST /connections/:id/validate -> updated connection after account/Insights read probe; failure HTTP 422 with error, stored last_error.
- POST /connections/:id/test-event body {test_event_code} -> queued event {id,status}; requires CAPI credential and Pixel even if live capi disabled.
- GET /events?connection_id=&status=&page= -> {items,total,page,page_size}; items id,connection_id,connection_name,visit_id,event_name,event_time,is_test,status,attempts,last_error,events_received,fbtrace_id,created_at,updated_at. No payload or credentials.
- POST /events/:id/retry -> {ok:true}; only failed/retry and unexpired, never successful or in-flight.
- POST /connections/:id/sync body {start:'YYYY-MM-DD',end:'YYYY-MM-DD'} -> job {id,status}; range <=90 days, dates in account timezone, verified read access required, enabled toggle controls auto-sync only.
- GET /sync-jobs?connection_id=&page= -> {items,total,page,page_size}; jobs id,connection_id,connection_name,start,end,status,reason,attempts,rows_count,last_error,created_at,finished_at.
- POST /sync-jobs/:id/retry -> {ok:true} for failed.
- GET /reports?connection_id=&start=&end=&level=campaign|adset|ad&campaign_id=&adset_id=&page= -> {items,total,page,page_size,summary,trends,coverage,connection}. Rows id,name,campaign_id,campaign_name,adset_id,adset_name,ad_id,ad_name,spend (number|null),currency,impressions,clicks,link_clicks,outbound_clicks,meta_consultations(number|null),visits,unique_visitors,manual_consultations,auto_redirects,link_ctr(number|null),link_cpc(number|null),internal_cpa(number|null),meta_cpa(number|null),matched(bool). Summary same numerical fields, total-level UV recomputed. Trends date + same numerical fields, independent daily UV. coverage {complete,covered_days,total_days,last_synced_at}; connection masked public config. Missing mapping/conversion returns null; unsynced window costs null.
- Existing link API adds meta_connection_id(number|null), attribution_mode(bound|dynamic), legacy_campaign_param(bool).

## Acceptance

Real PostgreSQL integration tests on a dedicated disposable database, never drop production schema. Verify auth, token secrecy/encryption, duplicate manual vs automatic events, disabled/test behavior, stable payload retry and lease recovery, partial-page failure preserving prior reports, account separation, profile separation, idempotent date replacement, correct hierarchy/UV/cost/null handling. Build/test both Vue apps; exercise admin in browser. Back up source/database before migrating running service. No real Meta token supplied: report live authorization/test-event/Ads Manager reconciliation as pending setup rather than claim verified delivery.
