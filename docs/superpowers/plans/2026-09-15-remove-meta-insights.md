# Remove Meta Insights Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all Meta Insights permissions, reports, synchronization, and read credentials while preserving Pixel CAPI delivery and first-party short-link advertising statistics.

**Architecture:** Meta connections become lightweight account groups that provide an account ID and Graph API version to their Pixels. Pixel records remain the sole CAPI credential and event-rule owners. Historical Insights database structures remain dormant for migration compatibility, while routes, workers, UI, and active credential handling are removed.

**Tech Stack:** Go, Gin, PostgreSQL, Vue 3, Element Plus, Node test runner, Docker Compose

**Spec:** `docs/superpowers/specs/2026-09-15-remove-meta-insights-design.md`

## Global Constraints

- Preserve Meta Pixel CAPI delivery, event retries, real-ad-click qualification, and event history.
- Preserve short-link ad statistics and the dormant `meta_ad_entities` lookup used for cached ad names.
- Do not drop or rewrite historical Insights tables or columns.
- Do not deploy online and do not commit or push to GitHub.
- Add comments for every new or changed nontrivial code path.

---

### Task 1: Reduce the Meta account contract to CAPI grouping fields

**Files:**
- Modify: `backend/internal/app/meta_test.go`
- Modify: `backend/internal/modules/meta/model.go`
- Modify: `backend/internal/modules/meta/connections.go`
- Modify: `backend/internal/modules/meta/handler.go`
- Modify: `backend/internal/modules/meta/pixels.go`
- Modify: `backend/internal/modules/meta/credentials.go`
- Modify: `backend/internal/modules/meta/crypto_rewrap.go`
- Modify: `frontend/tests/meta.test.js`
- Modify: `frontend/src/utils/meta.js`
- Modify: `frontend/src/views/MetaConnectionsView.vue`
- Modify: `frontend/src/views/MetaCredentialsView.vue`

**Interfaces:**
- Consumes: `Connection.ID`, `Connection.Name`, `Connection.AccountID`, `Connection.APIVersion`, `Connection.CreatedAt`.
- Produces: account APIs that return only grouping fields; connection form payload `{name, account_id, api_version}`; Pixel credentials remain listed as `kind: "capi"`.

- [ ] **Step 1: Write failing backend tests**

Add an integration test that submits deprecated fields such as `read_token` and `insights_enabled`, then asserts the response omits them and `read_token_cipher` remains empty. Update credential tests to assert only Pixel CAPI items are returned.

- [ ] **Step 2: Run the backend tests and verify the old contract fails**

Run:

```bash
TEST_DATABASE_URL='postgres://tester:testpass@127.0.0.1:55433/linkscope_test?sslmode=disable' go test ./internal/app -run 'TestMetaConnectionsProtectCredentials|TestMetaAccountWithoutPixelAndSourceInspection' -count=1
```

Expected: failure because the current account API stores or returns Insights fields and credentials.

- [ ] **Step 3: Write failing frontend tests**

Replace the Insights connection-form tests with a literal assertion that `connectionPayload(connectionForm(...))` contains only `name`, `account_id`, and `api_version`, and that masked read credentials never enter form state.

- [ ] **Step 4: Run the frontend tests and verify the old form fails**

Run `npm test` in `frontend`.

Expected: failure because the current form still emits `insights_enabled`, report settings, and read-token properties.

- [ ] **Step 5: Implement the reduced account contract**

Reduce the public Go connection model and save query to the three editable fields. Remove read validation and legacy account credential handling. Remove account credentials from the credential inventory and rewrap targets. Remove the legacy Pixel synchronization update from `SavePixel`; the Pixel row stays authoritative.

- [ ] **Step 6: Implement the reduced account UI**

Render only connection name, account ID, API version, edit, and event-history actions. Remove read-token, validation, synchronization, report, currency, timezone, and report-profile controls. Reduce `connectionForm` and `connectionPayload` to the same three fields. Keep token fields exclusively in `MetaPixelsView.vue`.

- [ ] **Step 7: Run focused tests**

Run the focused backend command from Step 2 and `npm test` in `frontend`.

Expected: all focused tests pass.

### Task 2: Remove Insights routes, workers, reports, and frontend navigation

**Files:**
- Modify: `backend/internal/app/meta_test.go`
- Modify: `backend/internal/modules/meta/handler.go`
- Modify: `backend/internal/modules/meta/lifecycle.go`
- Modify: `backend/internal/jobs/retention.go`
- Delete: `backend/internal/modules/meta/insights.go`
- Delete: `backend/internal/modules/meta/insights_handler.go`
- Delete: `backend/internal/modules/meta/reports.go`
- Delete: `backend/internal/modules/meta/sync_worker.go`
- Delete: `backend/internal/modules/meta/insights_handler_test.go`
- Delete: `backend/internal/modules/meta/insights_test.go`
- Delete: `backend/internal/modules/meta/reports_test.go`
- Delete: `backend/internal/modules/meta/sync_worker_test.go`
- Modify: `backend/internal/modules/meta/crypto_upgrade_test.go`
- Modify: `frontend/src/router/index.js`
- Modify: `frontend/src/layouts/AdminLayout.vue`
- Delete: `frontend/src/views/MetaReportsView.vue`
- Modify: `frontend/src/api/meta.js`
- Modify: `frontend/src/views/AdAnalyticsView.vue`

**Interfaces:**
- Consumes: existing authenticated `/api/v1/meta` group and CAPI `Service.ProcessEvent`.
- Produces: removed Insights endpoints returning 404; lifecycle with one CAPI worker; no Meta report route or menu entry.

- [ ] **Step 1: Write a failing removed-routes test**

Add an authenticated integration test asserting these old endpoints return 404:

```text
POST /api/v1/meta/connections/1/validate
POST /api/v1/meta/connections/1/sync
GET  /api/v1/meta/sync-jobs
POST /api/v1/meta/sync-jobs/1/retry
GET  /api/v1/meta/reports
```

- [ ] **Step 2: Run the route test and verify it fails**

Run the new test against the isolated PostgreSQL database.

Expected: failure because the current routes are registered.

- [ ] **Step 3: Remove backend Insights runtime code**

Remove `registerInsights`, account validation, `ProcessSyncJob`, `ScheduleSyncJobs`, report queries, and sync-job retention. Delete Insights implementation and tests. Change `Service.Start` from three workers to one `ProcessEvent` worker while preserving idempotent start and clean shutdown.

- [ ] **Step 4: Remove frontend Insights code**

Delete the report page, route, menu item, and report API functions. Remove Meta report wording from CSV import statistics.

- [ ] **Step 5: Run focused backend and frontend checks**

Run the removed-routes test, CAPI worker lifecycle test, `npm test`, and `npm run build`.

Expected: old endpoints return 404, CAPI still delivers, and the report page is absent from the built routes.

### Task 3: Preserve short-link advertising statistics and CAPI behavior

**Files:**
- Modify: `backend/internal/app/link_stats_test.go` only if fixtures depend on removed connection fields.
- Modify: `backend/internal/app/meta_test.go` only if helper setup depends on removed account credentials.
- Modify: `backend/internal/app/meta_upgrade_test.go` only if helper setup depends on removed account credentials.
- Modify: `backend/internal/modules/meta/crypto_upgrade_test.go` to retain Pixel and event encryption coverage.
- Modify: `docs/meta-setup.md`
- Modify: `backend/README.md`

**Interfaces:**
- Consumes: short-link `meta_connection_id`/`meta_pixel_id`, Pixel CAPI token, captured Meta URL parameters, dormant `meta_ad_entities`.
- Produces: unchanged link-level visits, unique visitors, manual consultations, automatic redirects, and CAPI delivery.

- [ ] **Step 1: Update test fixtures to create account then Pixel**

Replace legacy account-level Pixel and token setup with explicit `/meta/connections` followed by `/meta/pixels`. Return both identifiers where tests need link routing.

- [ ] **Step 2: Run CAPI and advertising-statistics tests**

Run:

```bash
TEST_DATABASE_URL='postgres://tester:testpass@127.0.0.1:55433/linkscope_test?sslmode=disable' go test ./internal/app -run 'Meta|LinkStats' -count=1
```

Expected: real-ad-click filtering, Pixel delivery, manual/auto split, and link statistics all pass.

- [ ] **Step 3: Update operator documentation**

Document the supported flow: create account group, add Pixel ID and CAPI Token, bind short link to Pixel, send a Meta test event, and inspect event history. Remove instructions for `ads_read`, read tokens, Insights sync, or Meta reports.

### Task 4: Full verification and local runtime update

**Files:**
- Modify: `.release/server-linux-arm64` generated artifact.
- Modify: `frontend/dist/**` generated artifacts.

**Interfaces:**
- Consumes: verified Go source and Vue production build.
- Produces: local `linkscope-app:latest` image running on `127.0.0.1:8080` with existing PostgreSQL data preserved.

- [ ] **Step 1: Run complete verification**

Run frontend tests, Prettier check, production build, backend `go test ./...` with the isolated database, and `go vet ./...`.

- [ ] **Step 2: Build the local runtime**

Cross-compile `server-linux-arm64`, build `linkscope-app:latest` from `deploy/Dockerfile.runtime`, and recreate only the Compose `app` service with `--no-build`.

- [ ] **Step 3: Verify the running service**

Check `/healthz`, authenticated Meta accounts, Pixels, and events APIs. Confirm old Insights endpoints return 404 and the served admin assets contain the simplified account/Pixel copy.

- [ ] **Step 4: Preserve local-only scope**

Confirm no remote deployment and no Git operation occurred.
