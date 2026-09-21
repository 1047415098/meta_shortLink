# Audio Novel Renaming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Atomically rename the existing novel subsystem to audio novel across the filesystem, database, Gin, Vue applications, URLs, configuration, Docker and current documentation without losing content, visits or uploads.

**Architecture:** Apply an in-place PostgreSQL rename migration first, then switch every runtime consumer to the new `audio_novel`/`audio-novel` contract in one application release. Keep the existing short-link code, WhatsApp, Meta and TimeSpent behavior unchanged; deliberately do not register compatibility aliases for old novel URLs.

**Tech Stack:** Go 1.27, Gin, PostgreSQL 17, Vue 3 JavaScript, Vue Router, Vite, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-09-21-audio-novel-renaming-design.md`

## Global Constraints

- Do not add audio upload, transcoding, playback or text-to-speech in this change.
- Do not retain old `/novel/*`, `/novel-api/*`, `/novel-assets/*`, `/novel-uploads/*` or `/api/v1/novels*` routes.
- Preserve every existing content row, visit, Meta event relationship and uploaded cover.
- Keep `short_links.code`, WhatsApp targets, Meta configuration and TimeSpent thresholds unchanged.
- Use `audio-novel` for URL/filesystem names, `audio_novel` for database/surface names, `audionovel` for the Go package and `AudioNovel` for Go identifiers.
- Continue serving all three frontends from the same Gin service.
- Add clear comments to every changed code area and keep implementation direct rather than over-abstracted.
- Do not create Git commits or push to GitHub; replace commit checkpoints with `git diff --check` and focused review.

## Review Focus

- Existing populated databases must migrate once without creating a second content table; Task 1 adds row-count and identity preservation assertions.
- The new `audio_novel` surface must preserve historical visits and reject the old value after migration; Task 1 tests both outcomes.
- A generic `/:code` route must not capture `/audio-novel/...`; Task 3 adds route-order and old-route 404 assertions.
- Cover files must remain readable after the named-volume and URL rename; Tasks 4 and 6 test upload paths and copy the existing volume before restart.
- No executable source may retain a stale old route, environment variable or directory reference; Task 7 performs targeted repository scans plus end-to-end checks.

---

### Task 1: Rename the database contract without losing data

**Files:**
- Create: `backend/internal/platform/database/migrations/016_audio_novel_rename.sql`
- Create: `backend/internal/platform/database/audio_novel_migration_test.go`

**Interfaces:**
- Consumes: migrations through `015_time_spent.sql`, table `novels`, `click_events.surface` values `short_link|novel`.
- Produces: table `audio_novels`, identity sequence `audio_novels_id_seq`, indexes prefixed `audio_novels_`, and `click_events.surface` values `short_link|audio_novel`.

- [ ] **Step 1: Write the failing populated-database migration test**

Add a PostgreSQL integration test in package `database` that reads the embedded migration files, applies entries through `015_time_spent.sql`, inserts one content row and one `surface='novel'` visit, applies migration 016, then asserts:

```go
if got := tableCount(t, db, "audio_novels"); got != 1 { t.Fatalf("audio_novels rows = %d", got) }
if tableExists(t, db, "novels") { t.Fatal("old novels table still exists") }
if surface != "audio_novel" { t.Fatalf("surface = %q", surface) }
```

Also insert another row after migration and assert its generated ID is greater than the preserved row ID.

- [ ] **Step 2: Run the focused test and confirm it fails because migration 016 is absent**

Run inside the isolated test database:

```bash
TEST_DATABASE_URL='postgres://analytics:...@db:5432/analytics_codex_test?sslmode=disable' go test ./internal/platform/database -run TestAudioNovelRenameMigration -count=1 -v
```

Expected: FAIL because `audio_novels` does not exist or `surface='audio_novel'` violates the old constraint.

- [ ] **Step 3: Add the in-place migration**

The migration must use explicit PostgreSQL renames and preserve rows:

```sql
ALTER TABLE novels RENAME TO audio_novels;
ALTER SEQUENCE novels_id_seq RENAME TO audio_novels_id_seq;
ALTER INDEX novels_active_slug_unique RENAME TO audio_novels_active_slug_unique;
ALTER INDEX novels_one_effective_featured RENAME TO audio_novels_one_effective_featured;
ALTER INDEX novels_public_order RENAME TO audio_novels_public_order;

ALTER TABLE click_events DROP CONSTRAINT click_events_surface_check;
UPDATE click_events SET surface='audio_novel' WHERE surface='novel';
ALTER TABLE click_events ADD CONSTRAINT click_events_surface_check
  CHECK (surface IN ('short_link','audio_novel'));
```

Add comments explaining why the table is renamed instead of copied and why the check constraint is dropped before updating values.

- [ ] **Step 4: Run migration tests against both an empty and populated database**

Run the focused migration test and the existing application migration suite. Expected: PASS, with the same content IDs and visit count before and after migration.

- [ ] **Step 5: Review the migration checkpoint**

Run `git diff --check` and inspect only migration/test changes. Do not commit.

### Task 2: Rename the Go module and configuration contract

**Files:**
- Rename: `backend/internal/modules/novel/` → `backend/internal/modules/audionovel/`
- Modify: every renamed Go file package declaration
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/bootstrap/app.go`
- Modify: config/bootstrap tests under `backend/internal/app/`

**Interfaces:**
- Consumes: Task 1 table `audio_novels`.
- Produces: package `audionovel`, `Config.AudioNovelDir`, `Config.AudioNovelUploadDir`, and application handler field `AudioNovel *audionovel.Handler`.

- [ ] **Step 1: Update tests to demand the new config names**

Change configuration tests to set:

```go
t.Setenv("AUDIO_NOVEL_DIR", "/tmp/audio-novel")
t.Setenv("AUDIO_NOVEL_UPLOAD_DIR", "/tmp/audio-novel-uploads")
```

Assert `AudioNovelDir` and `AudioNovelUploadDir` receive those exact paths and that obsolete `NOVEL_DIR` variables have no effect.

- [ ] **Step 2: Run config/package tests and confirm compile failures reference old identifiers**

Run `go test ./internal/config ./internal/bootstrap -count=1`. Expected: FAIL until the configuration and application fields are renamed.

- [ ] **Step 3: Rename the module directory and package declarations**

Move the directory intact, then change `package novel` to `package audionovel`. Rename exported handler wiring directly:

```go
AudioNovel *audionovel.Handler
```

Do not split handlers or introduce wrappers solely for the rename.

- [ ] **Step 4: Rename configuration fields and environment variables**

Use these defaults:

```go
AudioNovelDir: env("AUDIO_NOVEL_DIR", "../audio-novel/dist"),
AudioNovelUploadDir: env("AUDIO_NOVEL_UPLOAD_DIR", "../data/audio-novel-uploads"),
```

Update rendering and upload code to read the new fields. Replace every SQL query against `novels` with `audio_novels` and add a short migration-contract comment in the repository.

- [ ] **Step 5: Run renamed module tests**

Run `go test ./internal/config ./internal/bootstrap ./internal/modules/audionovel -count=1`. Expected: PASS.

- [ ] **Step 6: Review the backend-rename checkpoint**

Run `gofmt` on changed Go files and `git diff --check`. Do not commit.

### Task 3: Switch Gin routes, signatures, logging and Meta source URLs

**Files:**
- Modify: `backend/internal/transport/http/router.go`
- Modify: `backend/internal/modules/tracking/handler.go`
- Modify: `backend/internal/modules/landing/handler.go`
- Modify: `backend/internal/modules/links/repository.go`
- Modify: `backend/internal/modules/requestlogs/middleware.go`
- Modify: `backend/internal/modules/meta/events.go`
- Rename/modify: relevant tests under `backend/internal/app/` and `backend/internal/modules/audionovel/`

**Interfaces:**
- Consumes: `AudioNovel` handler and `audio_novel` surface from Tasks 1–2.
- Produces: only the new public/admin/static route contract and signature prefix `contact:audio_novel:`.

- [ ] **Step 1: Rewrite route tests before production routes**

Add assertions for all new paths, including:

```go
for _, path := range []string{
  "/audio-novel/hello",
  "/audio-novel/hello/stories",
  "/audio-novel/hello/stories/the-glass-orchard",
} { /* expect 200 and surface audio_novel */ }
```

Assert `/novel/hello`, `/novel-api/hello/home`, `/novel-assets/missing.js`, `/api/v1/novels` and `/novel-uploads/missing.webp` return 404 rather than being handled as valid public routes.

- [ ] **Step 2: Run route tests and confirm they fail on old registrations**

Run `go test ./internal/app -run 'AudioNovel|Routes|RequestLog' -count=1`. Expected: FAIL on missing new routes or old routes still being registered.

- [ ] **Step 3: Replace route registrations in one ordered block**

Register `/audio-novel-api`, `/audio-novel-assets`, `/audio-novel-uploads` and all `/audio-novel/:code` actions before `/:code`. Rename admin CRUD routes to `/api/v1/audio-novels` and cover upload to `/api/v1/audio-novel-covers`.

- [ ] **Step 4: Change visit and signature scope**

Use `surface="audio_novel"` and ticket signing:

```go
a.Sign("contact:audio_novel:" + code + ":" + eventID)
```

Update request-log classification, short-link deletion cleanup paths and Meta source URLs to `/audio-novel/<code>`. Keep TimeSpent deduplication ID format unchanged.

- [ ] **Step 5: Run backend route and event tests**

Run `go test ./internal/app ./internal/modules/audionovel ./internal/modules/landing ./internal/modules/meta -count=1`. Expected: PASS.

- [ ] **Step 6: Review the HTTP-contract checkpoint**

Run `gofmt` and `git diff --check`. Confirm no old route alias was added. Do not commit.

### Task 4: Rename and update the independent Vue application

**Files:**
- Rename: `novel/` → `audio-novel/`
- Modify: `audio-novel/package.json`
- Modify: `audio-novel/src/router/index.js`
- Modify: `audio-novel/src/lib/api.js`
- Modify: `audio-novel/src/lib/contact.js`
- Modify: `audio-novel/src/lib/routes.js`
- Modify: `audio-novel/src/lib/timeSpent.js`
- Modify: `audio-novel/src/App.vue`
- Modify: `audio-novel/src/styles.css`
- Modify: `audio-novel/vite.config.js`
- Modify: `audio-novel/scripts/precompress.mjs`
- Modify: all `audio-novel/tests/*.test.js`

**Interfaces:**
- Consumes: Task 3 public routes and static paths.
- Produces: the `linkscope-audio-novel` Vite build under `audio-novel/dist/audio-novel-assets`.

- [ ] **Step 1: Change frontend tests to the new contract**

Assert route helpers return `/audio-novel/:code/...`, API calls use `/audio-novel-api`, contact/PageView/TimeSpent use `/audio-novel/:code/*`, and no built source references `/novel-` or `/novel/`.

- [ ] **Step 2: Run the moved frontend tests and confirm failures**

Run `cd audio-novel && npm test`. Expected: FAIL because the implementation still emits old paths.

- [ ] **Step 3: Replace runtime paths and project names**

Set package name to `linkscope-audio-novel`, Vue routes to `/audio-novel/:code`, assets to `/audio-novel-assets`, API proxy to `/audio-novel-api`, uploads proxy to `/audio-novel-uploads`, and precompression root to `dist/audio-novel-assets`.

Keep page behavior, story content, TimeSpent and Meta event logic unchanged. Update comments and visible fallback copy from “novel” to “audio novel” only where it describes the product.

- [ ] **Step 4: Run tests and production build**

Run `npm test` and `npm run build` in `audio-novel/`. Expected: all tests pass and the hashed JS/CSS files appear under `dist/audio-novel-assets/`.

- [ ] **Step 5: Scan the moved project for stale runtime paths**

Run:

```bash
rg -n '/novel|novel-assets|novel-api|novel-uploads|linkscope-novel' audio-novel --glob '!dist/**' --glob '!node_modules/**'
```

Expected: no matches.

- [ ] **Step 6: Review the Vue-app checkpoint**

Run formatter/build checks and `git diff --check`. Do not commit.

### Task 5: Rename the administration experience and API client

**Files:**
- Rename: `frontend/src/views/NovelListView.vue` → `frontend/src/views/AudioNovelListView.vue`
- Rename: `frontend/src/views/NovelFormView.vue` → `frontend/src/views/AudioNovelFormView.vue`
- Rename: `frontend/src/api/novels.js` → `frontend/src/api/audioNovels.js`
- Rename: `frontend/tests/novels.test.js` → `frontend/tests/audio_novels.test.js`
- Modify: `frontend/src/router/index.js`
- Modify: `frontend/src/layouts/AdminLayout.vue`
- Modify: renamed views and tests

**Interfaces:**
- Consumes: Task 3 admin APIs and upload path.
- Produces: admin routes named `audio-novels`, `audio-novel-create`, `audio-novel-edit` under `/admin/audio-novels`.

- [ ] **Step 1: Update admin tests first**

Assert API functions request `/audio-novels`, preview requests `/audio-novels/preview`, cover upload requests `/audio-novel-covers`, router paths use `/admin/audio-novels`, and payloads still omit author/audio fields.

- [ ] **Step 2: Run admin tests and confirm failures**

Run `cd frontend && npm test`. Expected: FAIL until imports, paths and route names are renamed.

- [ ] **Step 3: Rename the client and views without changing CRUD behavior**

Rename imports/functions to `listAudioNovels`, `createAudioNovel`, `updateAudioNovel`, `deleteAudioNovel`, `uploadAudioNovelCover` and `previewAudioNovelMarkdown`. Update visible labels to “语音小说管理”“新增语音小说”“编辑语音小说”; keep Markdown and cover behavior intact.

- [ ] **Step 4: Update router and navigation**

Use lazy imports of the renamed view files and route paths beginning `/admin/audio-novels`. Do not retain hidden alias routes.

- [ ] **Step 5: Run admin tests, formatting and build**

Run `npm test`, `npm run format:check` and `npm run build`. Expected: PASS.

- [ ] **Step 6: Review the admin checkpoint**

Run `git diff --check` and inspect labels/routes. Do not commit.

### Task 6: Rename Docker, local scripts, runtime paths and persistent uploads

**Files:**
- Modify: `Dockerfile`
- Modify: `compose.yaml`
- Modify: `deploy/Dockerfile.runtime`
- Modify: `deploy/compose.production.yaml`
- Modify: `scripts/dev-local.sh`
- Modify: `.env.example`
- Modify: deployment verification scripts that reference the old paths

**Interfaces:**
- Consumes: `audio-novel/dist` and new environment variables.
- Produces: image paths `/app/audio-novel`, `/app/data/audio-novel-uploads` and Compose volume `audio_novel_uploads`.

- [ ] **Step 1: Add/adjust build-contract tests before Docker edits**

Update static asset tests and deployment verification expectations to require `/audio-novel-assets/`, `/app/audio-novel`, and the new environment variable names. Assert obsolete values are absent.

- [ ] **Step 2: Run contract tests and confirm they fail**

Run the static/deployment test commands documented by the repository. Expected: FAIL on old Docker copy paths and environment names.

- [ ] **Step 3: Update Docker build and runtime files**

Rename the frontend stage to `audio-novel`, copy `audio-novel/package*.json`, build from `audio-novel/`, copy its output to `/app/audio-novel`, and set:

```dockerfile
ENV AUDIO_NOVEL_DIR=/app/audio-novel \
    AUDIO_NOVEL_UPLOAD_DIR=/app/data/audio-novel-uploads
```

Update Compose mounts and named volume to `audio_novel_uploads`. Update `dev-local.sh` to start the renamed directory on port 5175.

- [ ] **Step 4: Copy the current upload volume before switching containers**

Stop only the application container, leave PostgreSQL running, then execute an explicit recoverable copy:

```bash
docker volume create linkscope_audio_novel_uploads
docker run --rm \
  -v linkscope_novel_uploads:/from:ro \
  -v linkscope_audio_novel_uploads:/to \
  alpine:3.22 sh -c 'cp -a /from/. /to/'
```

Do not delete `linkscope_novel_uploads` after copying.

- [ ] **Step 5: Build the complete image**

Run `docker compose -p linkscope build app`. Expected: the Go binary and all three frontend builds complete successfully.

- [ ] **Step 6: Review the deployment checkpoint**

Run `docker compose -p linkscope config`, deployment verification tests and `git diff --check`. Do not commit.

### Task 7: Update current documentation and complete end-to-end migration

**Files:**
- Modify: `README.md`
- Modify: `LOCAL-PREVIEW.md`
- Modify: `docs/verification.md`
- Modify: active deployment documents and path-bearing historical plan commands
- Modify: any remaining executable source found by the stale-name scan

**Interfaces:**
- Consumes: all new contracts from Tasks 1–6.
- Produces: one deployable, documented audio-novel system with no active old route/config dependency.

- [ ] **Step 1: Update current documentation and executable commands**

Document `audio-novel/`, `/audio-novel/:code`, `/admin/audio-novels`, the new environment variables, uploads path, development port 5175 and volume-copy procedure. Historical narrative can mention the old name only as migration background.

- [ ] **Step 2: Scan executable code and current docs for stale names**

Run targeted scans excluding the rename migration and historical narrative:

```bash
rg -n 'modules/novel|NOVEL_DIR|NOVEL_UPLOAD_DIR|../novel/dist|COPY novel|project_dir/novel' backend frontend audio-novel Dockerfile compose.yaml deploy scripts .env.example README.md LOCAL-PREVIEW.md
rg -n '"/novel|`/novel|/api/v1/novels|novel-assets|novel-api|novel-uploads' backend frontend audio-novel Dockerfile compose.yaml deploy scripts .env.example README.md LOCAL-PREVIEW.md
```

Expected: no runtime/config matches. Any intentional migration source match must be reviewed manually.

- [ ] **Step 3: Run the full automated verification suite**

Run:

```bash
cd backend && go test ./... && go vet ./...
cd frontend && npm test && npm run format:check && npm run build
cd landing && npm test && npm run build
cd audio-novel && npm test && npm run build
git diff --check
```

Expected: every command exits 0. The existing large Vite chunk warning is non-fatal; new errors are not accepted.

- [ ] **Step 4: Start the migrated stack**

Run `docker compose -p linkscope up -d --build`. Confirm migration 016 exists in `schema_migrations`, `audio_novels` contains the preserved rows, `novels` is absent, and `click_events` has no `surface='novel'` rows.

- [ ] **Step 5: Verify HTTP behavior**

Check `/healthz`, `/audio-novel/hello`, list/detail pages, `/admin/audio-novels`, content APIs, cover serving, PageView/contact/TimeSpent actions, and verify every old route returns 404.

- [ ] **Step 6: Perform desktop and narrow-screen browser QA**

Use normal clicks to traverse home → stories → detail, confirm the visible timer remains continuous, inspect the admin list/form CRUD controls, and verify no clipping, horizontal overflow, stale “小说管理” labels or broken asset requests.

- [ ] **Step 7: Final data and rollback audit**

Compare pre/post content counts and IDs, verify the new upload volume contains the copied files, retain the old volume, review container logs for errors, and record the verification results in `docs/verification.md`. Do not commit or push.
