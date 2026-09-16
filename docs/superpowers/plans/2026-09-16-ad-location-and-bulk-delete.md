# Ad Location Breakdown and Batch Link Deletion Implementation Plan

> **For agentic workers:** Execute inline with strict test-first changes; do not create commits.

**Goal:** Show per-ad visit location counts for the strict real-ad-click cohort and let administrators delete one or many short links safely.

**Architecture:** Extend the existing link statistics SQL so location groups are produced from the same `filtered` CTE as every other real-click metric. Add one authenticated batch-delete endpoint backed by a transaction that validates every requested link, removes visit/CAPI/request-log records, deletes the links, and writes one audit record. The Vue pages consume these contracts without creating page-only components.

**Tech Stack:** Go 1.27, Gin, pgx/PostgreSQL, Vue 3 JavaScript, Element Plus.

**Spec:** User-approved design in the current task.

## Global Constraints

- Never commit or push to GitHub.
- Every production change includes an explanatory code comment.
- Location counts use visits, not Cookie-unique visitors.
- Only strict real advertising clicks contribute to locations.
- Batch deletion is permanent and requires a UI confirmation.
- Any missing requested link aborts the entire delete transaction.

---

### Task 1: Lock the location contract with a failing integration test

**Files:**
- Modify: `backend/internal/app/link_stats_test.go`
- Modify: `frontend/tests/utils.test.js`

- [ ] Add fixtures covering two locations for one ad, another ad, a bot visit, and blank location values.
- [ ] Assert literal location visit counts returned under each ad row.
- [ ] Add a formatter test for full and unknown location fields.
- [ ] Run targeted tests and confirm they fail because the feature is absent.

### Task 2: Implement location aggregation and display

**Files:**
- Modify: `backend/internal/modules/analytics/link_stats.go`
- Modify: `frontend/src/utils/index.js`
- Modify: `frontend/src/views/LinkStatsView.vue`

- [ ] Add `locations` to each statistics row.
- [ ] Aggregate country, region, and city inside the strict filtered CTE and order locations by visits descending.
- [ ] Show the top three locations inline and the complete list in an Element Plus popover.
- [ ] Run targeted tests and confirm they pass.

### Task 3: Lock batch deletion with a failing integration test

**Files:**
- Create: `backend/internal/app/link_delete_test.go`

- [ ] Assert unauthenticated requests fail.
- [ ] Assert empty, duplicate, and invalid IDs fail without deleting anything.
- [ ] Assert a missing link aborts the whole batch.
- [ ] Assert successful deletion removes short links, click events, linked CAPI events and visitor request logs while retaining one audit record.
- [ ] Run the targeted test and confirm it fails because the route is absent.

### Task 4: Implement the transactional deletion API and UI

**Files:**
- Modify: `backend/internal/modules/links/handler.go`
- Modify: `backend/internal/modules/links/repository.go`
- Modify: `backend/internal/transport/http/router.go`
- Modify: `frontend/src/api/links.js`
- Modify: `frontend/src/views/LinkListView.vue`

- [ ] Add `POST /api/v1/links/batch-delete` with strict JSON validation and a bounded ID list.
- [ ] Lock and validate every requested link before deleting associated data and links in one transaction.
- [ ] Add row checkboxes, per-row delete, selected-count bulk delete, and a destructive confirmation dialog.
- [ ] Refresh the list and clear selection after success.
- [ ] Run targeted tests and confirm they pass.

### Task 5: Full verification and online deployment

**Files:**
- No additional source files expected.

- [ ] Run frontend tests, format check and production build.
- [ ] Run landing tests and production build.
- [ ] Run backend tests and vet with the dedicated test database.
- [ ] Build and deploy the application image while preserving the production database.
- [ ] Verify health, authenticated page assets and non-destructive API behavior; do not delete production links during verification.

