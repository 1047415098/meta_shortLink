# Landing Meta Attribution Design

**Goal:** Preserve Meta URL attribution across the landing page's first-party business requests and report only verified real-ad clicks without double-counting page-view or consultation requests.

## Request contract

The initial `GET /:code` remains authoritative because browser JavaScript cannot add custom headers to the navigation that loads the page. It captures the URL query in `click_events.parameters`. The Vue landing page mirrors the approved fields onto `POST /:code/view` and `POST /:code/contact` through fixed `X-Meta-*` headers. Values are percent-encoded UTF-8 so campaign and ad names are valid HTTP header values. The server request log decodes these headers for readable administration.

Approved fields are `fbclid` (with `fbcli` accepted as an input alias), `utm_source`, `utm_campaign`, `utm_content`, `adset_name`, `adset_id`, `ad_id`, `ad_name`, `placement`, and `site_source_name`. Unexpanded `{{...}}` placeholders and empty values are omitted.

Manual and timer consultation submissions use same-origin Fetch and request JSON. The server returns the configured WhatsApp target only after the signed visit ticket is validated and the original entry row is updated. The existing native form remains the fallback when the request cannot complete.

## Reporting contract

`visits` counts normal landing entries that contain both a resolved `fbclid` and a resolved `ad_id`. `unique_visitors` applies the same cohort and deduplicates by the first-party visitor Cookie. Page-view and consultation requests update the original visit and never create additional visits. Stored parameter and ad-ID values are trimmed before matching so older Meta URLs with spaces around separators remain attributable.

The per-link statistics page displays real-ad clicks, real-ad unique visitors, manual consultations, and automatic redirects. Its advertising table contains only `ad_id` rows from the same real-click cohort. Facebook preview bots, suspicious or prefetched requests, organic post clicks with only `fbclid`, and parameter sets without `ad_id` are excluded.

## Security and limits

Custom request headers are diagnostic mirrors and are not trusted for attribution. A visitor can forge them. Reporting always reads the parameters stored from the initial signed landing entry. No Meta API access token is accepted or propagated by this feature.
