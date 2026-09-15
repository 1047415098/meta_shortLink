# Landing Meta Attribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Carry Meta URL fields through landing business request headers and expose total and Cookie-deduplicated Meta visits.

**Architecture:** The entry URL is captured by the Go tracking handler and remains the trusted attribution record. Vue mirrors a strict parameter whitelist onto page-view and consultation requests, while analytics derives Meta visit metrics from the stored entry parameters.

**Tech Stack:** Vue 3, browser Fetch, Go/Gin, PostgreSQL.

**Spec:** `docs/superpowers/specs/2026-09-15-landing-meta-attribution.md`

## Global Constraints

- Do not commit or push to GitHub.
- Add explanatory comments to every production code change.
- Do not use custom request headers as trusted attribution input.

---

### Task 1: Landing attribution headers

**Files:**
- Create: `landing/src/lib/attribution.js`
- Create: `landing/tests/attribution.test.js`
- Modify: `landing/src/lib/view.js`
- Modify: `landing/tests/view.test.js`

**Interfaces:**
- Produces: `buildMetaAttributionHeaders(search)` returning a plain header object.
- Consumes: the returned object as `attributionHeaders` in `reportLandingView`.

- [x] Write tests proving the whitelist, alias, placeholder removal, Unicode encoding, and view request headers.
- [x] Run the landing tests and confirm they fail because the feature is absent.
- [x] Implement the minimal header builder and view-header merge with comments.
- [x] Run the landing tests and confirm they pass.

### Task 2: Fetch consultation and JSON target

**Files:**
- Modify: `landing/src/lib/contact.js`
- Modify: `landing/tests/contact.test.js`
- Modify: `landing/src/views/LandingView.vue`
- Modify: `backend/internal/modules/landing/handler.go`
- Modify: `backend/internal/app/landing_test.go`

**Interfaces:**
- Produces: `submitLandingContact({ code, ticket, trigger, attributionHeaders, request, navigate })`.
- Produces: `{ "target_url": "https://wa.me/..." }` when the request accepts JSON.

- [x] Write browser and Go tests proving Fetch headers, trigger separation, JSON success, and form fallback compatibility.
- [x] Run focused tests and confirm the expected failures.
- [x] Implement the request helper, countdown callback, Vue wiring, and conditional JSON response with comments.
- [x] Run focused tests and confirm they pass.

### Task 3: Meta visit reporting

**Files:**
- Modify: `backend/internal/modules/tracking/handler.go`
- Modify: `backend/internal/modules/analytics/link_stats.go`
- Modify: `backend/internal/app/link_stats_test.go`
- Modify: `frontend/src/views/LinkStatsView.vue`

**Interfaces:**
- Produces: `meta_visits` and `meta_unique_visitors` in summary and source rows.

- [x] Extend integration fixtures with Meta, organic, duplicate-Cookie, placeholder, and `fbcli` cases.
- [x] Run focused Go tests and confirm the new assertions fail.
- [x] Implement canonical parameter capture and parameter-based SQL metrics with comments.
- [x] Add the two report cards and explicit parameter-only source label with comments.
- [x] Run focused tests and confirm they pass.

### Task 4: Readable request logs and release verification

**Files:**
- Modify: `backend/internal/modules/requestlogs/middleware.go`
- Modify: `backend/internal/app/request_logs_test.go`

**Interfaces:**
- Produces: readable decoded `X-Meta-*` values in authenticated request-log details.

- [x] Write a failing integration assertion for an encoded Meta header.
- [x] Implement bounded percent decoding for Meta headers with comments.
- [x] Run landing, frontend build, Go tests, and production builds.
- [x] Deploy without replacing production data, then verify health, headers, visit totals, Cookie deduplication, manual consultation, and automatic consultation using disposable test data.
