# Backend structure

The process entry point is `cmd/server/main.go`. It reads `internal/config`, opens PostgreSQL, calls `internal/bootstrap.New`, starts retention, and shuts down HTTP gracefully.

- `internal/bootstrap`: composition of shared runtime, business handlers and HTTP transport.
- `internal/transport/http`: route ownership, common security headers, request size limit, admin CSRF header validation and static mounts.
- `internal/modules/auth`: session authentication and session repository.
- `internal/modules/links`: HTTP binding, link validation, link repository and transactional auditing.
- `internal/modules/tracking`: visitor classification and signed cookies, attribution, event repository and public GET/HEAD handling.
- `internal/modules/landing`: escaped Vue bootstrap rendering, per-link crawler metadata, signed contact validation and idempotent contact repository.
- `internal/modules/analytics`: date filters, report/event repository and CSV export.
- `internal/modules/requestlogs`: visitor-only capture/redaction middleware and admin inspection endpoints.
- `internal/modules/adspend`: CSV parser, atomic import/audit repository and upload handler.
- `internal/platform`: database pool/migrations, GeoIP lookup, precompressed static serving and shared security/runtime utilities.
- `internal/jobs`: retention and daily aggregation.
- `internal/app`: integration tests only. `compat_test.go` retains the old test harness aliases without shipping the old application package.

`handler.go` owns HTTP validation and responses; `service.go` owns reusable validation/parsing/business rules; `repository.go` owns database operations where separation is useful. Modules receive the same runtime so cookies, signing secret, rate limits, GeoIP and write failure reporting stay consistent.

## Vue applications and public routes

`FRONTEND_DIR` defaults to `../frontend/dist`. `LANDING_DIR` defaults to `../landing/dist`. Set both explicitly when running the compiled server outside `backend/`.

- `/` redirects to `/admin/overview`.
- `/admin` and `/admin/*` serve the admin application's index.
- `/admin-assets/*` serves `FRONTEND_DIR/admin-assets/*`.
- `/landing-assets/*` serves `LANDING_DIR/landing-assets/*`, including `images/`.
- `/assets/*` remains a compatibility mount for `FRONTEND_DIR/assets/*`.
- Existing `/api/v1/*` endpoints are unchanged; missing API routes remain 404.
- `GET /:code` records one event then renders the landing application or sends the existing 302 direct redirect. Browser modules do not refetch the visitor route.
- `POST /:code/contact` retains the one-hour signed visit ticket and separate idempotent manual/automatic timestamps before a 303 redirect.

The landing index must contain exactly one `<!--LANDING_BOOTSTRAP-->` marker. Go replaces it with a nonce-bearing `script#landing-data[type="application/json"]` containing `{link,ticket,cookie_enabled,error?}`. `link` uses the existing snake_case JSON fields. Unavailable pages use `link:null`, an empty ticket and `error:{status,message}`, with HTTP 404 or 410. If the landing build is absent, unavailable pages retain a plain-text status response; a valid landing link returns 503 until the build is restored. Go safely escapes JSON and per-link title/description/Open Graph metadata. CSP permits same-origin Vue modules, styles and images.

HTML/API responses keep `no-store`; assets retain precompression negotiation, immutable hashed JS/CSS caching and validation for stable images.

## Migrations and verification

`platform/database/migrations/*.sql` runs in filename order in a transaction protected by a PostgreSQL advisory lock. `schema_migrations` records applied files. `001_initial.sql` is the prior additive schema, so existing tables/data/settings are preserved. Add a new numbered file for subsequent changes; do not edit an applied migration.

Run `go test ./...` for tests that do not need PostgreSQL, or set `TEST_DATABASE_URL` to a disposable isolated test database to run the full suite. Integration setup drops/recreates that database's public schema: **never point it at the application database**. Tests cover signed cookies, attribution, bot filters, distinct manual/automatic timestamps, exports/imports, request-log privacy, routing, bootstrap escaping, cache policy and migration restart preservation. Run `go vet ./...` for static checks.

### Request-log URL tokens

New visitor requests preserve the original URL parameter named `token` (case insensitive, including repeated and empty values) in `request_logs.query_token_cipher`. The normal query JSON stays redacted. The authenticated detail endpoint decrypts it for direct display and records a `request_log.token_view` audit entry containing only the request ID. List responses do not return the ciphertext or plaintext; other credential fields remain redacted.

AES-GCM uses a domain-separated key derived from `APP_SECRET` and authenticates the request ID. Keep that secret with database backups; changing it makes these encrypted log values unreadable. The encrypted original shares the log's seven-day retention and a 32 KB plaintext limit. Historical redacted rows cannot be restored. Detail responses distinguish `available`, `not_saved`, and `unavailable` (decryption failed); an audit failure prevents plaintext disclosure.
