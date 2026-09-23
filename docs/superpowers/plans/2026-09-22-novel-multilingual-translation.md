# Novel Multilingual Translation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let operators generate selected stored translations for English novels, and serve the correct published language by IP or remembered manual selection.

**Architecture:** PostgreSQL stores immutable translation versions keyed by the English source revision. A small Go worker calls APIHZ for explicitly selected locales and atomically publishes complete versions; admin Vue exposes selection and progress, while the public repository and Vue H5 resolve only published translations with English fallback.

**Tech Stack:** Go 1.27, Gin, pgx/PostgreSQL, Vue 3, Element Plus, Vite, Node test runner, APIHZ form API.

**Spec:** `docs/superpowers/specs/2026-09-22-novel-multilingual-translation-design.md`

## Global Constraints

- English is the only source language and remains the unconditional fallback.
- Generate only locales selected for the current request; never translate on visitor requests.
- Supported targets are exactly `id`, `ja`, `ko`, `ms`, `pt`, `fil`, `th`, and `vi`.
- Publish one locale only when metadata and every active chapter are translated for the same source revision.
- Keep an old published version online until its replacement is complete.
- Keep APIHZ credentials server-only and out of responses and logs.
- Preserve short-link attribution, Meta events, reading-time behavior, and existing user changes.
- Add concise comments around revision, credential, recovery, and atomic-publish decisions; keep implementation direct.
- Do not create or push Git commits.

## Review Focus

- A UTF-8 block containing multibyte characters must never exceed 5000 bytes or split a rune.
- A source edit racing a running translation must never publish mismatched content.
- A failed or partially complete translation must not become visible through any public endpoint.
- An unsupported or unavailable remembered locale must fall back to English without a request loop.
- Retrying or double-clicking generation must not create two active jobs for the same novel and locale.

---

### Task 1: Translation schema and source revisions

**Files:**
- Create: `backend/internal/platform/database/migrations/021_novel_translations.sql`
- Create: `backend/internal/platform/database/novel_translation_migration_test.go`
- Modify: `backend/internal/modules/novel/repository.go`
- Modify: `backend/internal/modules/novel/model.go`
- Modify: `backend/internal/app/free_novel_content_test.go`

**Interfaces:**
- Produces: `novels.source_revision`, translation version/chapter tables, source revision increments.
- Consumes: current novel and chapter write transactions.

- [ ] Write migration and revision integration tests that fail because migration 021 and `source_revision` do not exist.
- [ ] Run `cd backend && go test ./internal/platform/database ./internal/app -run 'NovelTranslationMigration|NovelSourceRevision' -count=1` and verify the expected failure.
- [ ] Add the migration, model fields, and revision increments only for translatable source changes.
- [ ] Re-run the focused tests and the existing free novel suite to green.

### Task 2: APIHZ client and deterministic chunking

**Files:**
- Create: `backend/internal/modules/novel/translation_client.go`
- Create: `backend/internal/modules/novel/translation_client_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `.env.example`

**Interfaces:**
- Produces: locale definitions, `SplitTranslationText`, `Translator.Translate(ctx, locale, text)`.
- Consumes: APIHZ URL, developer ID and key from `Config`.

- [ ] Write failing unit tests for exact locale mappings, UTF-8 byte-safe paragraph chunks, form fields, non-200/body errors, empty output, and three attempts.
- [ ] Run `cd backend && go test ./internal/modules/novel ./internal/config -run 'TranslationClient|TranslationChunks|APIHZ' -count=1` and verify RED.
- [ ] Implement the direct HTTP client, injected `http.Client`, bounded response, retries, and server-only configuration.
- [ ] Re-run focused tests and `go test ./internal/config ./internal/modules/novel` to green.

### Task 3: Translation repository, worker, and admin API

**Files:**
- Create: `backend/internal/modules/novel/translation.go`
- Create: `backend/internal/modules/novel/translation_test.go`
- Create: `backend/internal/app/novel_translation_test.go`
- Modify: `backend/internal/modules/novel/admin.go`
- Modify: `backend/internal/modules/novel/handler.go`
- Modify: `backend/internal/transport/http/router.go`
- Modify: `backend/internal/bootstrap/app.go`

**Interfaces:**
- Produces: queue/list/status services and `/api/v1/novels/:id/translations` routes.
- Consumes: Task 1 tables and Task 2 translator.

- [ ] Write failing tests for locale validation, deduplication, missing credentials, one active version, progress, partial failure, source race, old-version retention, atomic replacement, recovery, and disable/enable.
- [ ] Run focused package/app tests and verify RED.
- [ ] Implement a two-worker in-process queue with database-backed recovery and bounded retry behavior.
- [ ] Implement authenticated list/generate/status endpoints with strict JSON validation and audit records.
- [ ] Run focused tests and all novel backend tests to green.

### Task 4: Admin translation controls

**Files:**
- Modify: `frontend/src/api/novels.js`
- Modify: `frontend/src/views/NovelFormView.vue`
- Modify: `frontend/tests/novels.test.js`

**Interfaces:**
- Produces: locale checkbox payload, translation status table, generate and enable/disable actions.
- Consumes: Task 3 admin endpoints.

- [ ] Add failing Node tests for the exact eight locale options, deduplicated payload, routes, independent Generate action, and status labels.
- [ ] Run `cd frontend && npm test -- --test-name-pattern='translation|翻译'` and verify RED.
- [ ] Add a simple multilingual panel to the existing novel form, poll while active tasks exist, and preserve old translation state when checkboxes change.
- [ ] Run the focused tests, complete frontend tests, and `npm run build` to green.

### Task 5: Localized public repository and bootstrap resolution

**Files:**
- Create: `backend/internal/modules/novel/locale.go`
- Create: `backend/internal/modules/novel/locale_test.go`
- Modify: `backend/internal/modules/novel/repository.go`
- Modify: `backend/internal/modules/novel/public.go`
- Modify: `backend/internal/modules/novel/render.go`
- Modify: `backend/internal/modules/tracking/handler.go`
- Modify: `backend/internal/app/free_novel_content_test.go`
- Modify: `backend/internal/app/novel_distribution_test.go`

**Interfaces:**
- Produces: locale resolution, localized public data, bootstrap `locale` and `available_locales`.
- Consumes: published versions and the entry GeoIP country already resolved by tracking.

- [ ] Write failing tests for query/cookie/IP/English priority, country mapping, per-novel English fallback, translated search, unpublished isolation, and bootstrap fields.
- [ ] Run the focused backend tests and verify RED.
- [ ] Implement locale validation and localized SQL joins without changing visit creation or ticket semantics.
- [ ] Run all novel distribution/content tests and the complete backend suite to green.

### Task 6: H5 dictionaries and language switching

**Files:**
- Create: `novel-h5/src/lib/i18n.js`
- Create: `novel-h5/tests/i18n.test.js`
- Modify: `novel-h5/src/bootstrap.js`
- Modify: `novel-h5/src/lib/api.js`
- Modify: `novel-h5/src/main.js`
- Modify: `novel-h5/src/App.vue`
- Modify: `novel-h5/src/components/SiteHeader.vue`
- Modify: `novel-h5/src/views/HomeView.vue`
- Modify: `novel-h5/src/views/SearchView.vue`
- Modify: `novel-h5/src/views/StoryListView.vue`
- Modify: `novel-h5/src/views/StoryView.vue`
- Modify: `novel-h5/src/styles.css`
- Modify: `novel-h5/tests/api.test.js`
- Modify: `novel-h5/tests/ui_content.test.js`

**Interfaces:**
- Produces: nine fixed UI dictionaries, remembered switch, `lang` on public API calls.
- Consumes: Task 5 bootstrap and localized API responses.

- [ ] Write failing tests for dictionary completeness, valid locale normalization, persisted manual selection, query propagation, available-locale filtering, and English fallback.
- [ ] Run focused novel H5 tests and verify RED.
- [ ] Add the language menu and reactive locale state; refetch current route without reloading the entry document.
- [ ] Replace fixed UI strings through the small dictionary helper, update document language/title, and keep tracking bootstrap unchanged.
- [ ] Run all H5 tests and production build to green.

### Task 7: Documentation, regression, and final review

**Files:**
- Modify: `README.md`
- Modify: `docs/verification.md`
- Modify: deployment environment examples when required by config tests.

**Interfaces:**
- Produces: operator setup and verification instructions.
- Consumes: completed Tasks 1-6.

- [ ] Document APIHZ variables, generation workflow, fallback rules, and credential handling.
- [ ] Run `go test ./...` and `go vet ./...` in `backend/`.
- [ ] Run tests and builds for `frontend/`, `novel-h5/`, `audio-novel/`, and `landing/`.
- [ ] Perform a fresh whole-change review against the spec, fix every important finding with a failing test first, and report any remaining environmental limitation.

