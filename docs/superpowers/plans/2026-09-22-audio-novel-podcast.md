# Audio Novel Podcast Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Add self-hosted single-file Podcast support to the audio-novel product, import the 11 authorized 2026 MP3 files, and match the production site's Audio Fiction list, story-to-Podcast navigation, Podcast detail, playback, and download behavior.

**Architecture:** Keep the one-to-one Podcast metadata on audio_novels, store MP3 files in a dedicated persistent directory, and expose separate audio list/detail APIs and SPA routes. Reuse the authenticated admin API, static-file handler, short-code validation, and Vue applications while keeping free-novel data and routes isolated.

**Tech Stack:** Go 1.22+, Gin, pgx/PostgreSQL 17, Vue 3 JavaScript, Vue Router, Element Plus, Node test runner, Ruby 2.6 + Nokogiri/Minitest, Docker Compose.

**Spec:** docs/superpowers/specs/2026-09-22-audio-novel-podcast-design.md

## Global Constraints

- One audio_novels row has zero or one complete MP3; do not add chapters or an audio_episodes table.
- MP3 files live under AUDIO_NOVEL_AUDIO_DIR and never share cover or free-novel directories.
- Maximum upload size is exactly 100 << 20 bytes.
- Public paths match ^/audio-novel-audio/[a-f0-9]{32}\.mp3$.
- Podcast APIs read only audio_novels; free-novel APIs must not return audio fields.
- Public players use preload="metadata" and never autoplay.
- Add concise Chinese comments around non-obvious business, transaction, security, and compatibility logic.
- Keep functions direct; do not add generic storage abstractions or unrelated refactors.
- Do not create, amend, or push Git commits. Each task ends with a diff checkpoint.

## Review Focus

- Accept ID3 and valid MPEG-frame headers; reject random bytes even if named .mp3 (Task 2).
- A failed replacement update must leave the old database path and playable file intact (Task 3).
- Range: bytes=0-15 must return 206 and exactly 16 bytes (Task 1).
- Audio metadata must never appear through /novel-api/* even with an equal slug (Task 4).
- Oversized or truncated remote downloads must not attach paths or stop later imports (Task 7).

---

### Task 1: Database, configuration, storage, and Range delivery

**Files:**
- Create: backend/internal/platform/database/migrations/020_audio_novel_podcasts.sql
- Modify: backend/internal/config/config.go
- Modify: backend/internal/config/config_test.go
- Modify: backend/internal/transport/http/router.go
- Modify: backend/internal/platform/staticfiles/assets_test.go
- Modify: .env.example
- Modify: compose.yaml
- Modify: deploy/compose.production.yaml
- Modify: deploy/Dockerfile.runtime
- Modify: deploy/Dockerfile.runtime.dockerignore

**Interfaces:**
- Produces Config.AudioNovelAudioDir from AUDIO_NOVEL_AUDIO_DIR.
- Produces audio_path, audio_duration, and audio_size_bytes columns.
- Produces GET|HEAD /audio-novel-audio/*filepath.

- [ ] **Step 1: Write failing tests**

Add migration assertions for all three columns and defaults. Extend config tests with AUDIO_NOVEL_AUDIO_DIR=/tmp/audio-novel-audio. Add a static-file test:

~~~go
request := httptest.NewRequest(http.MethodGet, "/audio-novel-audio/sample.mp3", nil)
request.Header.Set("Range", "bytes=0-15")
response := httptest.NewRecorder()
router.ServeHTTP(response, request)
if response.Code != http.StatusPartialContent || response.Body.Len() != 16 {
    t.Fatalf("range response = %d/%d", response.Code, response.Body.Len())
}
~~~

- [ ] **Step 2: Verify RED**

Run:

~~~bash
cd backend
go test ./internal/config ./internal/platform/database ./internal/platform/staticfiles
~~~

Expected: missing config, columns, or audio route causes failure.

- [ ] **Step 3: Implement migration and configuration**

Use:

~~~sql
ALTER TABLE audio_novels
  ADD COLUMN audio_path text NOT NULL DEFAULT '',
  ADD COLUMN audio_duration text NOT NULL DEFAULT '',
  ADD COLUMN audio_size_bytes bigint NOT NULL DEFAULT 0;

ALTER TABLE audio_novels ADD CONSTRAINT audio_novels_audio_fields_check CHECK (
  (audio_path = '' AND audio_duration = '' AND audio_size_bytes = 0)
  OR
  (audio_path ~ '^/audio-novel-audio/[a-f0-9]{32}\.mp3$'
   AND audio_duration ~ '^([0-9]+:)?[0-5][0-9]:[0-5][0-9]$'
   AND audio_size_bytes > 0)
);

CREATE INDEX audio_novels_public_audio_order
  ON audio_novels(published_at DESC, id DESC)
  WHERE enabled AND deleted_at IS NULL AND audio_path <> '';
~~~

Default AudioNovelAudioDir to ../data/audio-novel-audio and register the static handler. Add AUDIO_NOVEL_AUDIO_DIR=/app/data/audio-novel-audio and a dedicated audio_novel_audio volume to local and production Compose.

- [ ] **Step 4: Verify GREEN**

Run the Step 2 command. Expected: all packages pass.

- [ ] **Step 5: Checkpoint**

Run git diff --check and git status --short. Do not commit.

### Task 2: Audio metadata validation and secure MP3 upload

**Files:**
- Modify: backend/internal/modules/audionovel/model.go
- Modify: backend/internal/modules/audionovel/markdown_test.go
- Create: backend/internal/modules/audionovel/audio_upload.go
- Create: backend/internal/modules/audionovel/audio_upload_test.go
- Modify: backend/internal/modules/audionovel/handler.go
- Modify: backend/internal/transport/http/router.go

**Interfaces:**
- Adds AudioPath string, AudioDuration string, AudioSizeBytes int64 to AudioNovel and AudioNovelInput.
- Produces POST /api/v1/audio-novel-audio returning path and size_bytes.
- Produces isMP3Header and maxAudioBytes = 100 << 20.

- [ ] **Step 1: Write failing validation tests**

Use literal cases for the empty group, a valid path/duration/size, missing duration, wrong prefix, invalid seconds, and zero size. The valid example is:

~~~go
AudioPath: "/audio-novel-audio/0123456789abcdef0123456789abcdef.mp3",
AudioDuration: "32:05",
AudioSizeBytes: 23_100_419,
~~~

- [ ] **Step 2: Write failing upload tests**

Exercise real multipart requests for ID3 acceptance, MPEG-frame acceptance, random byte rejection, empty file rejection, 100 MiB + 1 rejection, random final names, and temporary-file cleanup.

- [ ] **Step 3: Verify RED**

Run:

~~~bash
cd backend
go test ./internal/modules/audionovel -run 'Audio|MP3|Validate'
~~~

Expected: missing fields or upload behavior fails.

- [ ] **Step 4: Implement minimal validation and streaming upload**

Use:

~~~go
var audioNovelAudioPattern = regexp.MustCompile("^/audio-novel-audio/[a-f0-9]{32}\\.mp3$")
var audioDurationPattern = regexp.MustCompile("^(?:[0-9]+:)?[0-5][0-9]:[0-5][0-9]$")
const maxAudioBytes = 100 << 20
~~~

Accept ID3 or MPEG sync where data[0] == 0xff and data[1]&0xe0 == 0xe0. Stream through io.LimitReader into os.CreateTemp, probe only the header, then Sync, close, generate a 16-byte random name, and atomically rename. Add Chinese comments at the header check and rename.

- [ ] **Step 5: Register and verify GREEN**

Register api.POST("/audio-novel-audio", h.AudioNovel.UploadAudio). Run the focused test and the complete audionovel package.

- [ ] **Step 6: Checkpoint**

Run git diff --check. Do not commit.

### Task 3: Persistence, replacement, removal, and cleanup

**Files:**
- Modify: backend/internal/modules/audionovel/repository.go
- Modify: backend/internal/modules/audionovel/admin.go
- Modify: backend/internal/modules/audionovel/handler.go
- Create: backend/internal/modules/audionovel/audio_lifecycle_test.go
- Modify: backend/internal/app/audio_novel_content_test.go
- Modify: backend/internal/transport/http/router.go

**Interfaces:**
- Repository.Create and Update persist audio fields.
- Repository.ClearAudio(ctx, id, actor) returns the previous public path.
- DELETE /api/v1/audio-novels/:id/audio clears metadata and removes the current local file.

- [ ] **Step 1: Write failing repository and handler tests**

Cover create/read, update, clear + audit log, missing row 404, article soft-delete cleanup, failed update preserving the old path/file, and rejection of filesystem deletion for a path outside /audio-novel-audio/.

- [ ] **Step 2: Verify RED**

Run:

~~~bash
cd backend
go test ./internal/modules/audionovel ./internal/app -run 'AudioNovel|AudioLifecycle'
~~~

Expected: missing scan fields, method, or route assertions fail.

- [ ] **Step 3: Extend SQL and scans**

Append audio_path,audio_duration,audio_size_bytes consistently to audioNovelColumns, audioNovelSummaryColumns, every Scan, INSERT, and UPDATE. Keep SQL argument order identical to AudioNovelInput.

Implement ClearAudio in one transaction: SELECT FOR UPDATE, clear all three fields, add audio_novel.audio_remove audit, commit, return the old path.

- [ ] **Step 4: Implement safe cleanup**

For update: read the previous row, perform the database update, then remove the old validated file only when the path changed. For deletion: capture the path, soft-delete first, then remove. Use only the basename after matching audioNovelAudioPattern. Log removal failures without restoring publicly deleted content.

- [ ] **Step 5: Register and verify GREEN**

Register api.DELETE("/audio-novels/:id/audio", h.AudioNovel.RemoveAudio). Run the focused command and go test ./....

- [ ] **Step 6: Checkpoint**

Run git diff --check. Do not commit.

### Task 4: Public Audio Fiction APIs and page routes

**Files:**
- Modify: backend/internal/modules/audionovel/repository.go
- Modify: backend/internal/modules/audionovel/public.go
- Modify: backend/internal/transport/http/router.go
- Modify: backend/internal/modules/audionovel/render.go
- Modify: backend/internal/app/audio_novel_content_test.go
- Modify: backend/internal/app/free_novel_content_test.go
- Modify: backend/internal/app/routes_test.go

**Interfaces:**
- Produces Repository.PublicAudioList and PublicAudioBySlug.
- Produces Handler.PublicAudioList and PublicAudioStory.
- Produces /audio-novel-api/:code/audio, /audio-novel-api/:code/audio/:slug, /audio-novel/:code/audio, and /audio-novel/:code/audio/:slug.

- [ ] **Step 1: Write failing public contract tests**

Create rows with audio, without audio, disabled audio, and an equal free-novel slug. Assert audio list filtering, metadata-only detail, 404 for unavailable content, short-code validation, no audio fields from /novel-api/*, and route precedence.

- [ ] **Step 2: Verify RED**

Run:

~~~bash
cd backend
go test ./internal/app -run 'Audio|FreeNovel|Routes'
~~~

Expected: new API or SPA route assertions fail.

- [ ] **Step 3: Implement queries and handlers**

Use existing public pagination/order with audio_path <> ''. PublicAudioBySlug must not render or return body content. Reuse the current short-code validation helper rather than copying validation SQL.

- [ ] **Step 4: Register routes**

Register API routes beside current audio-novel APIs. Register GET and HEAD SPA routes before generic /:code. Ensure bootstrap rendering accepts both routes.

- [ ] **Step 5: Verify GREEN**

Run the focused command, go test ./..., and go vet ./....

- [ ] **Step 6: Checkpoint**

Run git diff --check. Do not commit.

### Task 5: Admin upload, replacement, removal, and status UI

**Files:**
- Modify: frontend/src/api/audioNovels.js
- Modify: frontend/src/views/AudioNovelFormView.vue
- Modify: frontend/src/views/AudioNovelListView.vue
- Modify: frontend/tests/audio_novels.test.js

**Interfaces:**
- Produces uploadAudioNovelAudio(file) and removeAudioNovelAudio(id).
- Extends the whitelisted payload with audio_path, audio_duration, audio_size_bytes.
- Produces edit-form controls and a list status column.

- [ ] **Step 1: Write failing client/component tests**

Assert exact endpoints /audio-novel-audio and /audio-novels/42/audio. Assert the payload contains only the existing fields plus the three audio fields. Assert accept="audio/mpeg,.mp3", preload="metadata", no autoplay, replacement/removal labels, and list status text.

- [ ] **Step 2: Verify RED**

Run:

~~~bash
cd frontend
npm test
~~~

Expected: missing API functions, fields, or controls.

- [ ] **Step 3: Implement API and form state**

Upload with FormData and remove with DELETE. Add three audio defaults and preserve them on edit load. Before upload, reject files over 100 MiB, create a temporary object URL, read HTMLAudioElement.duration, format M:SS or H:MM:SS, revoke the URL, then upload. Backend validation remains authoritative.

- [ ] **Step 4: Implement controls and status**

Render a native preview only on the edit page. Use the existing confirmation dialog before removal. Show 已上传 · 32:05 or 无音频 in the list without list players. Add Chinese comments around metadata probing and replacement ordering.

- [ ] **Step 5: Verify GREEN and build**

Run npm test and npm run build in frontend.

- [ ] **Step 6: Checkpoint**

Run git diff --check. Do not commit.

### Task 6: Public Audio Fiction list, detail, and story navigation

**Files:**
- Modify: audio-novel/src/lib/api.js
- Modify: audio-novel/src/lib/routes.js
- Modify: audio-novel/src/router/index.js
- Modify: audio-novel/src/components/SiteHeader.vue
- Modify: audio-novel/src/views/StoryDetailView.vue
- Create: audio-novel/src/views/AudioListView.vue
- Create: audio-novel/src/views/AudioDetailView.vue
- Modify: audio-novel/src/styles.css
- Modify: audio-novel/tests/audio_novel.test.js

**Interfaces:**
- Produces fetchAudioNovelAudioList(code, page, pageSize) and fetchAudioNovelAudio(code, slug).
- Produces route names audio-list and audio-detail while retaining code.

- [ ] **Step 1: Write failing route/API/component tests**

Assert encoded list/detail paths, preload="metadata", absence of autoplay, Download and Read Story, conditional Podcast on text details, and an unavailable message that preserves the story link.

- [ ] **Step 2: Verify RED**

Run npm test in audio-novel. Expected: missing helpers, routes, or views.

- [ ] **Step 3: Implement helpers and routes**

Follow existing encodeURIComponent and readable-error patterns. Add routes under /audio-novel/:code/audio and /audio-novel/:code/audio/:slug with specific ordering.

- [ ] **Step 4: Implement pages and navigation**

AudioListView renders title, excerpt, date, duration, formatted size, native player, and More. AudioDetailView renders the Podcast heading, player, download, metadata, Read Story, WhatsApp action, loading, retry, 404, and media-error states. Add Audio Fiction to SiteHeader and conditionally add Podcast to StoryDetailView. Keep existing reading typography unchanged.

- [ ] **Step 5: Verify GREEN and build**

Run npm test and npm run build in audio-novel.

- [ ] **Step 6: Checkpoint**

Run git diff --check. Do not commit.

### Task 7: Extend importer and attach 11 authorized MP3 files

**Files:**
- Modify: scripts/import_bcs_2026.rb
- Modify: scripts/import_bcs_2026_test.rb

**Interfaces:**
- Produces BcsImport.parse_podcasts(html), with slug, audio_url, duration, and declared_size.
- Produces AdminClient.upload_audio(file) and update_story(id, payload).
- Adds --audio and --refresh-audio; existing article behavior remains unchanged.

- [ ] **Step 1: Write failing parser tests**

Extend the archive fixture with a Podcast block and assert exactly:

~~~ruby
assert_equal [{
  slug: "second-story",
  audio_url: "https://example.com/second-story.mp3",
  duration: "32:05",
  declared_size: "22.03MB"
}], BcsImport.parse_podcasts(html)
~~~

Add a Podcast title with no formal article and assert it is reported instead of guessed.

- [ ] **Step 2: Write failing streaming/continuation tests**

Use WEBrick endpoints for valid ID3, oversized content, truncated transfer, and a later valid file. Assert temporary-file removal, no update for failure, continuation, and final success/skipped/failed arrays.

- [ ] **Step 3: Verify RED**

Run ruby scripts/import_bcs_2026_test.rb. Expected: missing parser/downloader/client methods.

- [ ] **Step 4: Implement parsing and streaming**

Parse Podcast subheader, title link, audio source, and Duration text. Match normalized Podcast title to the formal story slug; never derive it from the episode number. Stream Net::HTTP read_body chunks into a binary Tempfile, count bytes, stop above 100 << 20, validate ID3/MPEG header, and remove the temporary file in ensure.

- [ ] **Step 5: Implement upload/update and idempotency**

Fetch all admin rows with page_size=100 and map by slug. Skip non-empty audio_path unless --refresh-audio. Upload and PATCH the complete existing editable payload plus the new audio fields. Exclude ID, timestamps, body_html, and deleted_at. Print deterministic per-item progress and a final summary.

- [ ] **Step 6: Verify GREEN and live dry-run**

Run:

~~~bash
ruby scripts/import_bcs_2026_test.rb
ruby scripts/import_bcs_2026.rb --audio
~~~

The second command downloads and validates but does not mutate without --apply. Expected: 11 eligible and 0 attached during dry-run.

- [ ] **Step 7: Import locally**

Run ruby scripts/import_bcs_2026.rb --audio --apply. Expected: 11 attachments; a safe rerun skips them. Do not use --refresh-audio for the initial import.

- [ ] **Step 8: Checkpoint**

Run git diff --check. Do not commit.

### Task 8: End-to-end verification and browser QA

**Files:**
- Modify only if evidence needs recording: docs/verification.md
- Verify every file changed by Tasks 1–7.

**Interfaces:**
- Consumes all earlier APIs, files, and UI.
- Produces fresh automated and browser evidence.

- [ ] **Step 1: Verify database and file counts**

Use a read-only query to assert exactly 11 active rows have non-empty paths, positive sizes, valid durations, and 11 unique paths. Resolve the Docker volume directory and confirm all returned basenames exist.

- [ ] **Step 2: Verify Range delivery**

For the first and last queried paths, issue HEAD and Range: bytes=0-15 requests. Expect audio/mpeg, positive full length, 206, Content-Range, and exactly 16 downloaded bytes. Use queried filenames, never guessed paths.

- [ ] **Step 3: Run the complete suite**

~~~bash
ruby -c scripts/import_bcs_2026.rb
ruby scripts/import_bcs_2026_test.rb
cd backend && go test ./... && go vet ./...
cd ../frontend && npm test && npm run build
cd ../audio-novel && npm test && npm run build
cd .. && docker compose build
~~~

Every command must exit 0. Report any pre-existing failure by command and test name.

- [ ] **Step 4: Functional browser QA**

At http://127.0.0.1:8080/audio-novel/hello verify Audio Fiction navigation, 11 episodes, play/pause/volume/seek on short and long files, More, Read Story, no Podcast on text-only content, direct route refresh, and invalid slug recovery.

- [ ] **Step 5: Visual QA**

Inspect 390×844, 768×1024, and 1440×900. Confirm no overflow, usable native controls, wrapping long titles, readable metadata, and unchanged story reading layout.

- [ ] **Step 6: Final safety audit**

Run git diff --check, git status --short, and git diff --stat. Confirm no free-novel API exposes audio, no credentials or MP3 files are Git-tracked, and no commit or push was created.

