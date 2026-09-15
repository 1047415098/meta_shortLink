# Meta Upgrade Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development for the isolated frontend task and review. No Git commits; this workspace is an existing non-Git artifact project.

**Goal:** Complete Meta account/Pixel management, event rules, source diagnostics and credential lifecycle while preserving existing traffic.
**Architecture:** Account-scoped Insights remains; Pixel targets and frozen visit routing extend the existing durable outbox. Legacy API fields remain compatible.
**Tech Stack:** Vue 3 JavaScript, Element Plus, Gin, PostgreSQL.
**Spec:** docs/superpowers/specs/2026-09-11-meta-upgrade.md

## Global Constraints
- Meta only. Single Go service. No Git commits. No third-party login cookies. Automatic redirect remains internal only.
- Preserve existing account/link/event IDs and reports. Real Graph writes require configured credentials; test with simulated Graph locally.
- One Pixel per link by default; null selects the legacy default. Currency/timezone and spend stay account-scoped.

## Task 1: Backend model and event lifecycle
Files: migrations/003_meta_upgrade.sql; meta/{model,connections,pixels,pixels_handler,events,event_worker,events_handler,credentials}.go; links/{model,repository,handler}.go; tracking/repository.go; landing/{handler,repository}.go; transport/http/router.go.
- [x] Add API integration tests creating two Pixels under one account, selecting the second, rejecting cross-account selection, generating exactly PageView + manual events from repeated submissions.
- [x] Run tests against isolated database; confirm existing API lacks pixels endpoint.
- [x] Migrate old defaults and pending events; implement CRUD and snapshots. Existing EnqueueContact remains compatibility entry; add EnqueuePageView.
- [x] Update worker to resolve frozen pixel record and enforce account/target expiry/disabled state, preserve retry payloads and event IDs.
- [x] Run integration and existing Meta tests.

## Task 2: Frontend administration (isolated delegation)
Files frontend/src/{api/meta.js,utils/meta.js,router/index.js,layouts/AdminLayout.vue,views/Meta*.vue,views/LinkListView.vue}; landing/src.
- [x] Add utility tests for new target form, event names, source URL template and immutable field handling.
- [x] Implement account/Pixel separation, event and credential pages, source diagnostics, per-link target selection using specified API.
- [x] Mount beacon POST /:code/view with existing signed ticket; retain automatic redirect and manual form behavior.
- [x] Run frontend and landing tests/build. Supply report for review.

## Task 3: Credential encryption and source diagnostics
Files meta/{crypto,credentials,sources}.go; config/config.go; tests.
- [x] Test legacy ciphertext readability after key rotation; no secret leakage; inspection rejects credentials and distinguishes missing/conflicting IDs.
- [x] Implement optional keyring META_ENCRYPTION_KEYS JSON and META_ENCRYPTION_KEY_ID, retain APP_SECRET legacy decode, transactional rewrap.
- [x] Add expiry/status/audit APIs without returning ciphertext or raw tokens.

## Task 4: Review and deployment validation
- [x] Review complete diff against source snapshot; correct material findings.
- [x] Run Go race tests and vet, both Vue tests/builds, browser walkthrough.
- [x] Back up live database before migrating; build/restart existing local service and verify counts and health. Do not send real events without explicit test credentials.
- [x] Update docs and report exact live-test limitations.
