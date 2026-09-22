# Free Novel H5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a standalone Vue 3 free-novel H5, independent novel/chapter backend APIs, and matching admin content management without changing the existing landing or audio-novel products.

**Architecture:** A new `novel-h5/` Vite application consumes public `/novel-api/:code/*` endpoints served by a new Go `novel` module. PostgreSQL stores novels and chapters in separate tables, the existing admin app manages both, and the existing tracking pipeline gains a third `novel` surface while keeping current behavior intact.

**Tech Stack:** Vue 3.5, Vue Router 4, Vite 6, JavaScript, Node test runner, Go 1.27, Gin, pgx/PostgreSQL, Element Plus, Docker.

**Spec:** `docs/superpowers/specs/2026-09-21-free-novel-h5-design.md`

## Global Constraints

- Create `novel-h5/` beside `frontend/`, `landing/`, and `audio-novel/`; do not replace or rename existing projects.
- All enabled chapters are free; do not add login, payment, subscription, coins, or locked chapters.
- Keep `novels` and `novel_chapters` independent from `audio_novels`.
- Preserve all existing short-link, landing, audio-novel, Meta, and analytics behavior.
- Use `novel` as the new tracking surface while retaining `short_link` and `audio_novel`.
- Use Vue 3 with JavaScript, keep code direct, and add concise Chinese comments only around non-obvious business, transaction, security, routing, and tracking logic.
- Preserve all existing changes. Do not create or push Git commits.
- Implement behavior test-first and verify each red test fails for the intended missing behavior before writing production code.

## Review Focus

- A disabled or missing short code must return 410 or 404 from every public novel API and page route without inserting a new visit.
- A disabled, deleted, or wrong-novel chapter must never be exposed through a guessed public URL.
- Duplicate active slugs, duplicate chapter numbers, and concurrent featured updates must return deterministic results instead of corrupting ordering.
- Corrupt local reading progress or a removed chapter must fall back to chapter 1 without breaking the reading page.
- Reintroducing `/novel/*` must not allow the generic `/:code` route to capture it or regress `/audio-novel/*`.

---

### Task 1: Database schema and runtime configuration

**Files:**
- Create: `backend/internal/platform/database/migrations/018_free_novels.sql`
- Create: `backend/internal/platform/database/free_novel_migration_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `.env.example`

**Interfaces:**
- Produces: PostgreSQL tables `novels`, `novel_chapters`; valid `click_events.surface='novel'`; `Config.NovelDir` and `Config.NovelUploadDir`.
- Consumes: existing migration runner and `Config` environment loader.

- [ ] **Step 1: Write the failing migration test**

Add an integration test that applies migrations through `018`, inserts two novels and chapters, and asserts these literal outcomes:

```go
if got := migrationTableCount(t, ctx, db, "novels"); got != 1 {
    t.Fatalf("novels rows = %d, want 1", got)
}
if _, err := db.Exec(ctx, `INSERT INTO novel_chapters(novel_id,chapter_number,title,body_markdown) VALUES($1,1,'Again','Body')`, novelID); err == nil {
    t.Fatal("duplicate active chapter number was accepted")
}
var surfaceRule string
if err := db.QueryRow(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname='click_events_surface_check'`).Scan(&surfaceRule); err != nil {
    t.Fatalf("read surface constraint: %v", err)
}
if !strings.Contains(surfaceRule, "'novel'") {
    t.Fatalf("surface constraint = %q, want novel", surfaceRule)
}
```

- [ ] **Step 2: Run the migration test and verify RED**

Run: `cd backend && go test ./internal/platform/database -run FreeNovelMigration -count=1`

Expected: FAIL because migration 018 and the tables do not exist.

- [ ] **Step 3: Add schema and constraints**

Create `018_free_novels.sql` with:

```sql
CREATE TABLE novels (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  title text NOT NULL,
  slug text NOT NULL,
  author text NOT NULL DEFAULT '',
  category text NOT NULL DEFAULT '',
  excerpt text NOT NULL,
  cover_path text NOT NULL DEFAULT '',
  published_at date NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  featured boolean NOT NULL DEFAULT false,
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX novels_active_slug_unique ON novels(slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX novels_one_effective_featured ON novels(featured) WHERE featured AND enabled AND deleted_at IS NULL;
CREATE INDEX novels_public_order ON novels(sort_order DESC,published_at DESC,id DESC) WHERE enabled AND deleted_at IS NULL;

CREATE TABLE novel_chapters (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  novel_id bigint NOT NULL REFERENCES novels(id),
  chapter_number integer NOT NULL CHECK (chapter_number > 0),
  title text NOT NULL DEFAULT '',
  body_markdown text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX novel_chapters_active_number_unique ON novel_chapters(novel_id,chapter_number) WHERE deleted_at IS NULL;
CREATE INDEX novel_chapters_public_order ON novel_chapters(novel_id,chapter_number) WHERE enabled AND deleted_at IS NULL;

ALTER TABLE click_events DROP CONSTRAINT click_events_surface_check;
ALTER TABLE click_events ADD CONSTRAINT click_events_surface_check CHECK (surface IN ('short_link','audio_novel','novel'));
```

- [ ] **Step 4: Run the migration test and verify GREEN**

Run: `cd backend && go test ./internal/platform/database -run FreeNovelMigration -count=1`

Expected: PASS.

- [ ] **Step 5: Write failing config tests**

Assert that absent variables produce `../novel-h5/dist` and `../data/novel-uploads`, and explicit `NOVEL_DIR` / `NOVEL_UPLOAD_DIR` override both values.

- [ ] **Step 6: Implement configuration**

Add `NovelDir` and `NovelUploadDir` to `Config`, load both environment variables, document them in `.env.example`, and retain all existing defaults.

- [ ] **Step 7: Run configuration and migration suites**

Run: `cd backend && go test ./internal/config ./internal/platform/database`

Expected: PASS.

### Task 2: Novel and chapter domain, validation, repository, Markdown, and uploads

**Files:**
- Create: `backend/internal/modules/novel/model.go`
- Create: `backend/internal/modules/novel/model_test.go`
- Create: `backend/internal/modules/novel/repository.go`
- Create: `backend/internal/modules/novel/repository_test.go`
- Create: `backend/internal/modules/novel/markdown.go`
- Create: `backend/internal/modules/novel/markdown_test.go`
- Create: `backend/internal/modules/novel/upload.go`
- Create: `backend/internal/modules/novel/upload_test.go`

**Interfaces:**
- Produces: `Novel`, `NovelInput`, `Chapter`, `ChapterInput`, `NovelList`, `Repository`, `ValidateNovelInput`, `ValidateChapterInput`, `RenderMarkdown`, and secure cover upload helper.
- Consumes: `*pgxpool.Pool`, audit table conventions, runtime upload limits.

- [ ] **Step 1: Write failing validation tests**

Table-drive literal invalid inputs: empty title, malformed slug, 161-character title, 501-character excerpt, invalid cover prefix, invalid date, chapter number zero, empty body, and duplicate unknown JSON fields at handler level later. Include valid Unicode slug and empty optional author/category cases.

- [ ] **Step 2: Run validation tests and verify RED**

Run: `cd backend && go test ./internal/modules/novel -run 'ValidateNovel|ValidateChapter' -count=1`

Expected: build failure because the package and validation functions do not exist.

- [ ] **Step 3: Implement focused models and validation**

Use these public shapes:

```go
type Novel struct {
    ID int64 `json:"id"`
    Title string `json:"title"`
    Slug string `json:"slug"`
    Author string `json:"author"`
    Category string `json:"category"`
    Excerpt string `json:"excerpt"`
    CoverPath string `json:"cover_path"`
    PublishedAt string `json:"published_at"`
    Enabled bool `json:"enabled"`
    Featured bool `json:"featured"`
    SortOrder int `json:"sort_order"`
    ChapterCount int `json:"chapter_count"`
}
type Chapter struct {
    ID int64 `json:"id"`
    NovelID int64 `json:"novel_id,omitempty"`
    ChapterNumber int `json:"chapter_number"`
    Title string `json:"title"`
    BodyMarkdown string `json:"body_markdown,omitempty"`
    BodyHTML string `json:"body_html,omitempty"`
    Enabled bool `json:"enabled"`
}
```

Accept `/novel-uploads/<32 hex>.(jpg|png|webp)` only. Trim text before database writes and cap chapter Markdown at 200,000 runes.

- [ ] **Step 4: Run validation tests and verify GREEN**

Run: `cd backend && go test ./internal/modules/novel -run 'ValidateNovel|ValidateChapter' -count=1`

Expected: PASS.

- [ ] **Step 5: Write failing repository integration tests**

Cover create/update/list/filter/soft-delete for novels; create/update/list/soft-delete for chapters; stable public ordering; featured uniqueness; disabled/deleted content exclusion; a chapter ID belonging to another novel returning `pgx.ErrNoRows`; and duplicate slug/chapter number returning a PostgreSQL unique violation.

- [ ] **Step 6: Implement repository transactions**

Keep one `Repository{DB *pgxpool.Pool}`. Use explicit transactions for featured changes and novel soft deletion. Each write inserts an audit record with `novel.create`, `novel.update`, `novel.delete`, `novel.chapter.create`, `novel.chapter.update`, or `novel.chapter.delete`. Do not generalize the existing audio-novel repository.

- [ ] **Step 7: Run repository tests and verify GREEN**

Run: `cd backend && go test ./internal/modules/novel -run Repository -count=1`

Expected: PASS when a test database is configured; otherwise the existing test convention must report a clear skip.

- [ ] **Step 8: Write failing Markdown and upload tests**

Assert paragraphs/headings/lists render, while raw HTML, `javascript:` links, iframe, and Markdown images do not survive. Assert JPEG/PNG/WebP succeed and SVG, forged MIME, oversized input, and path fragments fail.

- [ ] **Step 9: Implement Markdown and cover upload**

Follow the proven `audionovel` whitelist and atomic upload pattern, but emit `/novel-uploads/` paths and write only below `Config.NovelUploadDir`. Add Chinese comments at the whitelist and atomic rename boundaries.

- [ ] **Step 10: Run the entire novel module suite**

Run: `cd backend && go test ./internal/modules/novel -count=1`

Expected: PASS.

### Task 3: Admin and public novel HTTP APIs

**Files:**
- Create: `backend/internal/modules/novel/handler.go`
- Create: `backend/internal/modules/novel/admin.go`
- Create: `backend/internal/modules/novel/public.go`
- Create: `backend/internal/app/free_novel_content_test.go`
- Modify: `backend/internal/transport/http/router.go`
- Modify: `backend/internal/bootstrap/app.go`

**Interfaces:**
- Produces: authenticated `/api/v1/novels*` and public `/novel-api/:code/*` contracts from the spec.
- Consumes: Task 2 repository/model functions, existing auth middleware, `links.Repository.ByCode`.

- [ ] **Step 1: Write failing end-to-end API tests**

Using the existing app test harness, assert:

```go
admin := login(t, app)
created := call(app, "POST", "/api/v1/novels", novelJSON, admin)
if created.Code != 200 { t.Fatalf("create novel: %d %s", created.Code, created.Body.String()) }
chapter := call(app, "POST", "/api/v1/novels/1/chapters", chapterJSON, admin)
if chapter.Code != 200 { t.Fatalf("create chapter: %d %s", chapter.Code, chapter.Body.String()) }
if got := call(app, "GET", "/novel-api/hello/stories/story-one", "", nil); got.Code != 200 {
    t.Fatalf("public story: %d %s", got.Code, got.Body.String())
}
```

Also assert missing/disabled code, unpublished novel, disabled chapter, wrong novel/chapter pairing, search `q`, pagination, malformed IDs, unknown JSON fields, duplicate slug/number conflict, and no extra `click_events` from API reads.

- [ ] **Step 2: Run API tests and verify RED**

Run: `cd backend && go test ./internal/app -run FreeNovelContent -count=1`

Expected: FAIL with 404 routes.

- [ ] **Step 3: Implement admin handlers**

Use strict `json.Decoder.DisallowUnknownFields`, five-second DB contexts, 400 for validation, 404 for missing rows, 409 for unique/featured conflicts, 503 through `runtime.ServerError`, and 204 for successful deletes. Keep novel and chapter IDs validated separately and verify child ownership in repository queries.

- [ ] **Step 4: Implement public handlers**

Return:

```json
{"featured":{},"ranking":[],"items":[]}
{"items":[],"page":1,"page_size":20,"total":0,"pages":0}
{"story":{},"chapters":[],"related":[]}
{"chapter":{},"previous":null,"next":{"chapter_number":2,"title":"..."}}
```

Validate the short code before every query. The detail endpoint returns chapter summaries only; the chapter endpoint renders `body_html` and omits `body_markdown` publicly.

- [ ] **Step 5: Register module and API routes**

Instantiate `novel.Handler` in bootstrap, add it to transport handlers, register admin APIs after auth, register `/novel-api` before generic routes, and serve `/novel-uploads` from `NovelUploadDir`.

- [ ] **Step 6: Run API and regression tests**

Run: `cd backend && go test ./internal/app ./internal/modules/novel -count=1`

Expected: PASS, including existing audio-novel tests.

### Task 4: Novel page rendering, routing, tracking surface, and analytics compatibility

**Files:**
- Create: `backend/internal/modules/novel/render.go`
- Create: `backend/internal/modules/novel/render_test.go`
- Modify: `backend/internal/modules/tracking/handler.go`
- Modify: `backend/internal/modules/requestlogs/middleware.go`
- Modify: `backend/internal/modules/analytics/service.go`
- Modify: `backend/internal/modules/analytics/service_test.go`
- Modify: `backend/internal/modules/analytics/model.go`
- Modify: `backend/internal/modules/analytics/repository.go`
- Modify: `backend/internal/modules/analytics/link_stats.go`
- Modify: `backend/internal/transport/http/router.go`
- Modify: `backend/internal/app/routes_test.go`
- Modify: `backend/internal/app/app_test.go`

**Interfaces:**
- Produces: `GET|HEAD /novel/:code`, `/search`, `/stories`, `/stories/:slug`; bootstrap marker `<!--NOVEL_H5_BOOTSTRAP-->`; analytics surface `novel`.
- Consumes: existing tracking classification, visit persistence, Meta PageView, request logging, and Task 1 configuration.

- [ ] **Step 1: Write failing render and route tests**

Assert safe JSON escaping of `</script>`, public bootstrap contains `surface:"novel"` and code but no secret, GET returns the H5 shell, HEAD has no body, missing code is 404, disabled code is 410, and `/audio-novel/hello` remains unchanged.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `cd backend && go test ./internal/modules/novel ./internal/app -run 'NovelRender|FreeNovelRoutes' -count=1`

Expected: FAIL because the renderer and page routes do not exist.

- [ ] **Step 3: Implement the standalone renderer and tracking entry**

Read `Config.NovelDir/index.html`, replace exactly one `<!--NOVEL_H5_BOOTSTRAP-->`, and set page title/description safely. Extend the tracking handler through a new `Novel` entry that calls the shared tracking path with surface `novel` and delegates successful/error rendering to the new handler. Add a Chinese comment explaining why all `/novel/*` page routes precede `/:code`.

- [ ] **Step 4: Register page and asset routes**

Register GET/HEAD routes for home, search, list, and story. Add `/novel-assets/*` to the static map. Include novel page paths in request-log capture but keep static/API calls excluded.

- [ ] **Step 5: Write failing analytics tests**

Assert `surface=novel` is accepted, bad values are rejected, overview and link stats return distinct `novel_views`, and filtering by novel excludes short-link/audio-novel rows.

- [ ] **Step 6: Extend analytics without renaming existing fields**

Add `NovelViews` / `novel_views` alongside current fields, accept `novel` in filters, and extend SQL `FILTER` expressions. Preserve every old JSON property.

- [ ] **Step 7: Run backend regression suite**

Run: `cd backend && go test ./... && go vet ./...`

Expected: PASS with no new warnings.

### Task 5: Admin API client and novel/chapter management UI

**Files:**
- Create: `frontend/src/api/novels.js`
- Create: `frontend/tests/novels.test.js`
- Create: `frontend/src/views/NovelListView.vue`
- Create: `frontend/src/views/NovelFormView.vue`
- Modify: `frontend/src/router/index.js`
- Modify: `frontend/src/layouts/AdminLayout.vue`
- Modify: `frontend/src/views/DashboardView.vue`
- Modify: `frontend/src/components/AnalyticsFilter.vue`
- Modify: `frontend/src/views/VisitListView.vue`

**Interfaces:**
- Produces: admin routes `novels`, `novel-create`, `novel-edit`; API helpers for all Task 3 admin endpoints.
- Consumes: existing `request()` helper, Element Plus patterns, backend response shapes.

- [ ] **Step 1: Write failing API client tests**

Assert payload whitelisting and these literal paths: `/novels?q=glass`, `/novels/7`, `/novels/7/chapters`, `/novels/7/chapters/3`, `/novels/preview`, and `/novel-covers`. Assert author, sort order, chapter number/title/body/status are included while unknown UI fields are dropped.

- [ ] **Step 2: Run the client test and verify RED**

Run: `cd frontend && npm test -- --test-name-pattern='novel client|novel payload'`

Expected: FAIL because `src/api/novels.js` does not exist.

- [ ] **Step 3: Implement the API client**

Export direct functions for list/get/create/update/delete/status/featured/upload/preview and chapter list/create/update/delete. Keep payload functions small and explicit; add Chinese comments explaining field whitelisting.

- [ ] **Step 4: Run client tests and verify GREEN**

Run: `cd frontend && npm test -- --test-name-pattern='novel client|novel payload'`

Expected: PASS.

- [ ] **Step 5: Add routes and navigation**

Add `/admin/novels`, `/admin/novels/new`, and `/admin/novels/:id/edit`; add “小说管理” beside “语音小说管理” using the existing `Reading` icon and active-menu metadata.

- [ ] **Step 6: Build the list page**

Match `AudioNovelListView.vue` conventions: query/status/page state, cover/title/author/chapter count/category/order/date/status/featured columns, create/edit/status/featured/delete actions, empty-page fallback, and Chinese messages.

- [ ] **Step 7: Build the form and chapter editor**

Keep basic novel fields in one clear form. After the novel exists, show an inline chapter table and dialog containing chapter number, title, status, Markdown editor, and server preview. Handle 409 with explicit duplicate slug/chapter messages. Do not create page-local wrapper components.

- [ ] **Step 8: Extend analytics labels**

Add “免费小说站” as a filter/display value and `novel_views` to the dashboard without altering current short-link/audio-novel metrics.

- [ ] **Step 9: Run admin tests and build**

Run: `cd frontend && npm test && npm run build`

Expected: PASS and successful production build.

### Task 6: Scaffold the standalone Vue 3 novel H5 and implement data behavior

**Files:**
- Create: `novel-h5/package.json`
- Create: `novel-h5/package-lock.json`
- Create: `novel-h5/vite.config.js`
- Create: `novel-h5/index.html`
- Create: `novel-h5/scripts/precompress.mjs`
- Create: `novel-h5/src/main.js`
- Create: `novel-h5/src/App.vue`
- Create: `novel-h5/src/bootstrap.js`
- Create: `novel-h5/src/router/index.js`
- Create: `novel-h5/src/lib/api.js`
- Create: `novel-h5/src/lib/routes.js`
- Create: `novel-h5/src/lib/progress.js`
- Create: `novel-h5/tests/api.test.js`
- Create: `novel-h5/tests/router.test.js`
- Create: `novel-h5/tests/progress.test.js`

**Interfaces:**
- Produces: runnable package `linkscope-free-novel-h5`, router factory, public API client, safe progress helpers.
- Consumes: Task 3 public JSON and Task 4 bootstrap data.

- [ ] **Step 1: Create package metadata and install lockfile only**

Use Vue 3.5, Vue Router 4.5+, Vite 6, `@vitejs/plugin-vue`, and `@fortawesome/fontawesome-free` for icons. Scripts: `dev`, `build`, `preview`, `test`. Configure port 5176 and proxies for `/novel-api`, `/novel-uploads`, and relevant `/novel/:code` tracking endpoints. Build assets into `novel-assets`.

- [ ] **Step 2: Write failing route/API/progress tests**

Assert literal code-preserving paths, encoded search query and pagination, complete response validation, progress save/read/remove, malformed JSON fallback, wrong slug fallback, and chapter removal fallback to chapter 1.

- [ ] **Step 3: Run tests and verify RED**

Run: `cd novel-h5 && npm test`

Expected: FAIL because implementation modules do not exist.

- [ ] **Step 4: Implement minimal router, API, and progress helpers**

Routes:

```js
{ path: "/novel/:code", name: "home" }
{ path: "/novel/:code/search", name: "search" }
{ path: "/novel/:code/stories", name: "stories" }
{ path: "/novel/:code/stories/:slug", name: "story" }
```

Use one `readJSON()` error mapper for 404, 410, and retryable failures. Store progress under `novel-progress:<slug>` as `{ chapterNumber, scrollY, updatedAt }`, validate all numeric fields, and cap negative scroll at zero.

- [ ] **Step 5: Run behavior tests and verify GREEN**

Run: `cd novel-h5 && npm test`

Expected: PASS.

### Task 7: Implement screenshot-matched H5 views and interactions

**Files:**
- Create: `novel-h5/src/components/AppHeader.vue`
- Create: `novel-h5/src/components/BottomNav.vue`
- Create: `novel-h5/src/components/StoryCard.vue`
- Create: `novel-h5/src/components/ChapterDrawer.vue`
- Create: `novel-h5/src/views/HomeView.vue`
- Create: `novel-h5/src/views/SearchView.vue`
- Create: `novel-h5/src/views/StoryListView.vue`
- Create: `novel-h5/src/views/StoryView.vue`
- Create: `novel-h5/src/views/UnavailableView.vue`
- Create: `novel-h5/src/styles.css`
- Create: `novel-h5/public/novel-assets/images/cover-placeholder.webp`
- Create: `novel-h5/tests/content.test.js`
- Create: `novel-h5/tests/reader.test.js`

**Interfaces:**
- Produces: complete visible home/search/list/reader/drawer flow and responsive layout.
- Consumes: Task 6 API/router/progress helpers and backend-provided cover images.

- [ ] **Step 1: Write failing content-state tests**

Test pure state helpers exported from view-adjacent modules: home sections map empty arrays safely; query trimming does not request empty input; chapter navigation returns correct previous/next literals; last chapter produces no next target; and progress picks an existing chapter only.

- [ ] **Step 2: Run tests and verify RED**

Run: `cd novel-h5 && npm test -- --test-name-pattern='home sections|chapter navigation|reading progress'`

Expected: FAIL because helpers and views are absent.

- [ ] **Step 3: Implement shared shell and cards**

Use real Font Awesome icons, semantic buttons, explicit labels, and backend covers. Recreate the screenshot proportions: compact header, centered portrait carousel, ranking rows, two-column cards, and fixed bottom navigation. Keep one `StoryCard` with `compact` and standard variants; do not create wrappers for one-off sections.

- [ ] **Step 4: Implement home, search, and list**

Home loads once and renders loading/error/empty/content states. Search syncs `q` to the URL and debounces non-empty input without issuing requests for whitespace. List appends pages through “Load more” and prevents duplicate requests while loading.

- [ ] **Step 5: Implement reader and chapter drawer**

Load story metadata first, then the saved or first chapter. `Start Reading` moves to the reading section; `Continue Reading` loads the next chapter and scrolls to its heading. The drawer traps focus, closes with Escape/backdrop/X, marks the current chapter, and shows no locks. Save throttled scroll progress and restore only after chapter content renders.

- [ ] **Step 6: Add responsive and accessible styling**

Use the screenshots as the mobile target, a centered app shell on large displays, `max-width:760px` for body text, `44px` controls, visible `:focus-visible`, stable image aspect ratios, bottom safe-area padding, reduced-motion rules, and no horizontal overflow at 360px.

- [ ] **Step 7: Run H5 tests and build**

Run: `cd novel-h5 && npm test && npm run build`

Expected: PASS and `dist/novel-assets/*` output.

### Task 8: Build/deploy integration, documentation, and full verification

**Files:**
- Modify: `Dockerfile`
- Modify: `compose.yaml`
- Modify: `deploy/compose.production.yaml`
- Modify: `scripts/dev-local.sh`
- Modify: `README.md`
- Modify: `docs/verification.md`
- Create: `design-qa.md`

**Interfaces:**
- Produces: one container serving all four frontends and persistent novel covers; documented local/production commands; final visual QA result.
- Consumes: all prior tasks.

- [ ] **Step 1: Write or extend failing integration assertions**

Extend route/config tests so a production-like app requires `NovelDir/index.html`, serves `/novel-assets/`, and never maps `/novel-assets` to audio-novel files. Run them before Docker changes and confirm the expected failure.

- [ ] **Step 2: Add the fourth frontend build and runtime paths**

Add a `novel-h5` Node build stage, copy its dist to `/app/novel-h5`, create/chown `/app/data/novel-uploads`, set `NOVEL_DIR` and `NOVEL_UPLOAD_DIR`, and mount a separate persistent volume in both compose files.

- [ ] **Step 3: Update local development and README**

Build/start `novel-h5` on port 5176, document `/novel/:code`, new admin routes, environment variables, database model, test commands, and the fact that all chapters are free.

- [ ] **Step 4: Run all automated verification**

Run:

```sh
(cd frontend && npm test && npm run build)
(cd landing && npm test && npm run build)
(cd audio-novel && npm test && npm run build)
(cd novel-h5 && npm test && npm run build)
(cd backend && go test ./... && go vet ./...)
docker build -t linkscope-free-novel:verify .
```

Expected: every command exits 0 with no failing tests.

- [ ] **Step 5: Start the local system and verify the primary flow**

Open a valid local short code, then verify: home carousel → search → result → reader → start reading → contents drawer → chapter switch → continue reading → refresh restores progress. Also verify admin create novel → upload cover → add two chapters → publish → content appears publicly.

- [ ] **Step 6: Run blocking visual QA**

At comparable mobile viewports, capture local home, search, reader top, long body, and contents drawer. Compare each against the four supplied references, record P0–P3 differences in root `design-qa.md`, fix all P0/P1/P2 findings, and repeat until the file contains `final result: passed`. If browser capture is unavailable, record `final result: blocked` and do not claim visual completion.

- [ ] **Step 7: Inspect final working tree**

Run: `git status --short` and `git diff --check`.

Expected: only intentional project files are changed; no whitespace errors, generated secrets, database dumps, or Git commits.
