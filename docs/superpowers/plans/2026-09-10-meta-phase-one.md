# Meta Phase One Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development for the independent frontend task and focused review; controller implements the coupled backend and integration.

**Goal:** Manual consultation CAPI plus daily ad Insights and admin management.
**Architecture:** Single Go service, PostgreSQL outbox/jobs, encrypted credentials, Vue SFC routes.
**Tech Stack:** Go/Gin/pgx, Vue 3 JS, Element Plus, existing PostgreSQL.
**Spec:** docs/superpowers/specs/2026-09-10-meta-phase-one.md

## Global Constraints

- No Git commits/pushes; this source directory is not a Git repository. Use source archive and database backup instead of a worktree.
- No real token provided. Features default disabled; verify external protocol with local HTTP fixtures, not real events.
- Existing counters/CSV/landing behavior preserved; one-page SFCs, no page-local components folders.
- API and UI must follow the spec's exact JSON contract.

## Task 1: Configuration, Graph boundary and schema
Files: migrations/002_meta.sql; modules/meta/{model,crypto,client,connections}.go; focused *_test.go.
- [x] Add failing integration test: unauthenticated connections endpoint returns 401, create returns masked flags, stored ciphertext does not contain token; invalid account/version rejected.
- [x] Add local HTTP protocol tests: tokens only in Authorization; Graph errors sanitized; non-Graph next URLs are never followed; invalid metrics are errors.
- [x] Implement additive tables/indexes, encryption, bounded fixed-host Graph transport, credential CRUD/validation and authenticated routing.
- [x] Run `go test ./internal/modules/meta ./internal/app` against disposable PostgreSQL.

## Task 2: Visitor attribution and durable manual events
Files: tracking/{handler,repository,service}.go; landing/{handler,repository,render}.go; links/{model,repository,service}.go; meta/{events,event_worker}.go.
- [x] Add tests proving repeat manual submission enqueues once, automatic submission enqueues none, disabled config sends none, old visit retains original config, URL tokens excluded.
- [x] Implement explicit legacy parser and dynamic/bound IDs, connection snapshot, transactional event creation, stable IDs/time, claim/retry/cleanup.
- [x] Add tests for HTTP failure/retry success, process restart lease recovery and successful payload cleanup.

## Task 3: Daily Insights synchronization and reports
Files: meta/{insights,sync_worker,reports}.go; jobs lifecycle/bootstrap/main.
- [x] Add fixtures with multiple pages and retryable/permanent failures; test no partial overwrite and repeated sync replacement.
- [x] Implement account metadata probe, pagination/asynchronous report jobs, scheduled windows, immutable profile keys and date coverage.
- [x] Add integration fixtures for two accounts, two ads sharing a visitor, auto/manual/date boundaries; assert correct sums, distinct UV, missing mapping/null costs and no CSV double count.
- [x] Implement hierarchy reports, summaries/trends, filters and pagination.

## Task 4: Admin and landing integration (independent frontend worker)
Files: frontend/src/views/MetaConnectionsView.vue, MetaEventsView.vue, MetaReportsView.vue; api/meta.js; routes/nav; LinkListView.vue; landing measurement notice.
- [x] Implement exact spec API, loading/empty/error states, write-only token inputs, validation and test-event controls.
- [x] Reports default one selected connection; campaign/adset/ad drilldown, date filters, cards, daily chart/table, sync and history, separate internal/Meta metrics.
- [x] Existing CSV ad page retained separately; no adding sources together.
- [x] Run frontend tests/build, landing tests/build, format changed files; inspect via browser after backend ready.

## Task 5: Review, deployment and documentation
- [x] Review integration against spec and credentials/data consistency; fix findings with covering tests.
- [x] Run Go unit/integration/race tests and Vue tests/build; preserve result evidence.
- [x] Source/database backup, build service image and restart only linkscope app; validate health, auth routes and real admin UI without supplying credentials or triggering real Meta sends.
- [x] Document account/token setup, metrics, lifecycle/retries, backup/encryption-key handling and pending real Meta verification.

## Progress

Plan reviewed: prior user approval covers implementation. Multiple connections confirmed. Skills' commit/worktree steps do not apply to this non-Git source directory; no repository will be created.


Implementation complete and verified on 2026-09-10. No Git commits or pushes.

- All Go packages pass `go test -race ./... -count=1` against two disposable PostgreSQL databases; `go vet ./...` and local/Linux arm64 builds pass.
- Admin 20 tests and landing 7 tests pass; both production builds pass, admin formatting passes. Existing shared vendor chunk size advisory remains.
- Review found and fixed credential-recovery UI deadlock and concurrent stale credential restore. Added covering tests; both fixes re-reviewed. UTF-8 error truncation also covered.
- Browser verified configuration save, campaign/adset/ad drilldown, expected totals and empty event state in an isolated mock-data database. Production navigation and existing counters verified.
- Normal multi-stage container build stalled while resolving remote Node/Go images. Deployed Linux arm64 binary and both locally verified bundles onto the existing runtime image. Rollback runtime retained as `linkscope-app:before-meta-20260910`. Original Dockerfile unchanged.
- Restored missing GeoIP MMDB from the existing work-directory gzip archive after restart exposed a missing mount file. Service and geolocation health confirmed afterward.
- Production migration count changed 1→2; existing 8 links and 238 raw visit records preserved. Meta configuration/event/job tables remain empty. Local and existing ngrok health return 200.
- TEST95428 is validated against local Graph fixtures only. No real Pixel/ad account IDs or credentials supplied; actual Meta receipt, Test Events UI confirmation and live Insights reconciliation remain pending user configuration.
