# Audio Novel Multi-Buyer Attribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为语音小说增加一部内容对应任意多条投放短链、每条短链独立播放漏斗统计，以及 Meta/TikTok 二选一的网页事件和服务端回传，同时保留旧语音短链的公共首页行为。

**Architecture:** 继续以 short_links 和 click_events 作为链接配置与访问事实表，在第一次正常 GET 时冻结语音小说、平台、Pixel、门槛和动态归因参数。audionovel 模块拥有语音投放链接、播放票据、播放状态和统计；tracking 只负责入口访问；audio-novel H5 依据原生 audio 事件用单调时钟累计真实播放时间；现有 Meta/TikTok 队列负责异步发送，广告平台异常不影响播放和内部统计。

**Tech Stack:** Go 1.27.1、Gin、pgx/PostgreSQL、Vue 3 JavaScript、Vue Router、Element Plus、Node node:test、Meta Pixel/CAPI、TikTok Pixel/Events API。

**Spec:** docs/superpowers/specs/2026-09-24-audio-novel-multi-buyer-attribution-design.md

## Global Constraints

- 一条新式语音投放链接必须只绑定一部启用、未删除且有 MP3 的语音小说。
- 一条链接只允许 Meta 或 TikTok 一个平台，不允许双平台；需要双投时创建两条链接。
- 新式语音链接固定动态归因，运营后台不展示渠道、Campaign、Ad Group 或广告 ID 输入框。
- 播放达标门槛默认 10 秒；0 表示关闭 ViewContent；非零值必须在 1–3600 秒。
- PageView 在入口成功打开时确认一次；StartListening 在第一次真实 playing 时确认一次；ViewContent 在真实墙钟播放秒数达到门槛时确认一次。
- 后台或锁屏只要音频仍处于 playing 就继续累计；pause、waiting、stalled、seeking 和 ended 期间不累计；拖动进度不能增加播放秒数。
- 播放完成只写内部状态，不回传 Meta/TikTok；必须由 ended 触发且累计媒体消费达到已知总时长的 90%。
- 浏览器 Pixel 与服务端事件对同一业务事件使用相同确定性 event_id；所有写入和回传都必须幂等。
- Pixel、凭证、SDK 或广告 API 故障不能阻断音频播放、页面访问和内部统计。
- 旧 product_type=legacy 的语音短链继续进入公共归档首页，不自动绑定内容、不补算历史播放。
- 文字小说、普通短链接、WhatsApp 行为和现有 Meta/TikTok 配置接口保持兼容。
- Access Token、完整 ttclid、IP、User-Agent 和加密匹配数据不得进入 H5 启动数据或普通日志；统计页只返回脱敏点击标识。
- 新增或修改代码只添加解释业务约束、安全或非直观状态机的必要注释，保持实现直接，不建立通用广告 Provider 框架。
- 工作区已有修改全部保留；不执行 git commit、git push、创建 PR 或生产部署。

## File Structure

### Backend

- Create: backend/internal/platform/database/migrations/023_audio_novel_distribution.sql
- Create: backend/internal/platform/database/audio_novel_distribution_migration_test.go
- Modify: backend/internal/modules/audionovel/model.go
- Modify: backend/internal/modules/audionovel/repository.go
- Modify: backend/internal/modules/audionovel/audio_lifecycle.go
- Create: backend/internal/modules/audionovel/distribution.go
- Create: backend/internal/modules/audionovel/distribution_admin.go
- Create: backend/internal/modules/audionovel/distribution_stats.go
- Create: backend/internal/modules/audionovel/playback.go
- Create: backend/internal/modules/audionovel/playback_test.go
- Modify: backend/internal/modules/audionovel/handler.go
- Modify: backend/internal/modules/audionovel/public.go
- Modify: backend/internal/modules/audionovel/render.go
- Modify: backend/internal/modules/audionovel/render_test.go
- Modify: backend/internal/modules/links/model.go
- Modify: backend/internal/modules/links/repository.go
- Modify: backend/internal/modules/tracking/handler.go
- Modify: backend/internal/modules/tracking/repository.go
- Modify: backend/internal/modules/meta/events.go
- Modify: backend/internal/modules/meta/model.go
- Modify: backend/internal/modules/tiktok/events.go
- Modify: backend/internal/modules/tiktok/model.go
- Modify: backend/internal/modules/tiktok/handler.go
- Modify: backend/internal/transport/http/router.go
- Modify: backend/internal/app/audio_novel_content_test.go
- Create: backend/internal/app/audio_novel_distribution_test.go
- Modify: backend/internal/app/routes_test.go
- Modify: backend/internal/app/meta_delivery_rules_test.go
- Modify: backend/internal/app/tiktok_test.go

### Audio Novel H5

- Create: audio-novel/src/lib/playback.js
- Create: audio-novel/src/lib/tiktok.js
- Modify: audio-novel/src/lib/api.js
- Modify: audio-novel/src/lib/meta.js
- Modify: audio-novel/src/lib/routes.js
- Modify: audio-novel/src/bootstrap.js
- Modify: audio-novel/src/App.vue
- Modify: audio-novel/src/views/AudioDetailView.vue
- Create: audio-novel/tests/playback.test.js
- Create: audio-novel/tests/tiktok.test.js
- Modify: audio-novel/tests/api.test.js
- Modify: audio-novel/tests/audio_ui.test.js
- Modify: audio-novel/tests/router.test.js

### Admin Frontend

- Create: frontend/src/api/audioNovelLinks.js
- Create: frontend/src/views/AudioNovelLinkListView.vue
- Create: frontend/src/views/AudioNovelLinkStatsView.vue
- Modify: frontend/src/views/AudioNovelListView.vue
- Modify: frontend/src/views/AudioNovelFormView.vue
- Modify: frontend/src/router/index.js
- Modify: frontend/src/layouts/AdminLayout.vue
- Modify: frontend/src/utils/tiktok.js
- Create: frontend/tests/audio_novel_links.test.js
- Modify: frontend/tests/audio_novels.test.js
- Modify: frontend/tests/admin_navigation.test.js
- Modify: frontend/tests/routes.test.js

## Review Focus

1. **旧语音短链没有 audio_novel_id：** migration、入口分流和 H5 bootstrap 必须继续把 legacy 链接送往公共首页；Task 1、3、9 覆盖。
2. **播放计时被快进、重复事件或后台切换放大：** 客户端只累加单调墙钟区间，服务端只接受单调增长并按访问存活时间裁剪；Task 4、5 覆盖。
3. **绑定内容被停用、删除或移除 MP3：** 入口返回 410，不产生正常访问；恢复内容或 MP3 后同一链接自动恢复；Task 2、3 覆盖。
4. **Pixel/凭证停用或广告 API 失败：** 播放和内部状态仍成功，事件保持可诊断状态；Task 4、8、9 覆盖。
5. **并发重复 start、playback-time 或 complete：** 数据行锁与确定性事件 ID 保证时间、漏斗计数和 outbox 各只推进一次；Task 4、6、9 覆盖。

---

### Task 1: Add the additive audio distribution schema and integer duration

**Files:**
- Create: backend/internal/platform/database/migrations/023_audio_novel_distribution.sql
- Create: backend/internal/platform/database/audio_novel_distribution_migration_test.go
- Modify: backend/internal/modules/audionovel/model.go
- Modify: backend/internal/modules/audionovel/repository.go
- Modify: backend/internal/modules/audionovel/audio_lifecycle.go
- Modify: backend/internal/modules/audionovel/audio_lifecycle_test.go

**Interfaces:**
- Consumes: migrations 020 and 022, short_links, click_events, tiktok_events, audio_novels.
- Produces: audio_novel_id bindings, server playback state, audio_duration_seconds.

- [ ] **Step 1: Add a failing migration test**

Test the following after running the normal migration runner:

~~~go
func TestAudioNovelDistributionMigration(t *testing.T) {
    db := openMigrationTestDB(t)
    ctx := context.Background()

    assertColumn(t, db, "short_links", "audio_novel_id")
    assertColumn(t, db, "audio_novels", "audio_duration_seconds")
    assertColumn(t, db, "click_events", "playback_seconds")
    assertColumn(t, db, "click_events", "media_consumed_seconds")
    assertColumn(t, db, "tiktok_events", "audio_novel_id")

    // A legacy link is still valid without a content binding.
    _, err := db.Exec(ctx, "INSERT INTO short_links(name,code,target_url,product_type) VALUES($1,$2,$3,'legacy')", "legacy audio", "legacy-audio-schema", "https://example.com")
    if err != nil {
        t.Fatal(err)
    }
}
~~~

Also insert invalid rows and assert PostgreSQL rejects:
- product_type=audio_novel with no audio_novel_id.
- product_type=audio_novel with both novel_id and audio_novel_id.
- tiktok_events with both novel_id and audio_novel_id.
- negative playback_seconds or media_consumed_seconds.

Run: cd backend && go test ./internal/platform/database -run AudioNovelDistributionMigration -v

Expected: FAIL because migration 023 is absent.

- [ ] **Step 2: Add migration 023**

Add these columns and constraints without rewriting existing rows:

~~~sql
ALTER TABLE audio_novels
  ADD COLUMN audio_duration_seconds integer NOT NULL DEFAULT 0
  CHECK (audio_duration_seconds >= 0 AND audio_duration_seconds <= 86400);

UPDATE audio_novels
SET audio_duration_seconds =
  CASE
    WHEN audio_duration ~ '^[0-9]{2}:[0-5][0-9]$'
      THEN split_part(audio_duration, ':', 1)::integer * 60
         + split_part(audio_duration, ':', 2)::integer
    WHEN audio_duration ~ '^[0-9]+:[0-5][0-9]:[0-5][0-9]$'
      THEN split_part(audio_duration, ':', 1)::integer * 3600
         + split_part(audio_duration, ':', 2)::integer * 60
         + split_part(audio_duration, ':', 3)::integer
    ELSE 0
  END
WHERE audio_duration_seconds = 0;

ALTER TABLE short_links
  ADD COLUMN audio_novel_id bigint REFERENCES audio_novels(id);

ALTER TABLE click_events
  ADD COLUMN audio_novel_id bigint REFERENCES audio_novels(id),
  ADD COLUMN playback_seconds integer NOT NULL DEFAULT 0,
  ADD COLUMN media_consumed_seconds numeric(12,3) NOT NULL DEFAULT 0,
  ADD COLUMN playback_updated_at timestamptz,
  ADD COLUMN audio_started_at timestamptz,
  ADD COLUMN audio_qualified_at timestamptz,
  ADD COLUMN audio_completed_at timestamptz;

ALTER TABLE tiktok_events
  ADD COLUMN audio_novel_id bigint REFERENCES audio_novels(id);
~~~

Drop and recreate only the existing product-binding and event-binding checks so they express:
- novel means novel_id set and audio_novel_id null.
- audio_novel means audio_novel_id set and novel_id null.
- legacy means both content IDs null.
- tiktok event has at most one content ID.
- playback counters are non-negative.

Add indexes:
- short_links(audio_novel_id, enabled).
- click_events(link_id, audio_novel_id, occurred_at DESC).
- click_events(link_id, audio_qualified_at, audio_completed_at).

- [ ] **Step 3: Extend AudioNovel and AudioNovelInput**

Add AudioDurationSeconds int to both structs. Update audioNovelColumns, audioNovelSummaryColumns, scanAudioNovel, Create, Update and ClearAudio.

Validation rules:
- no audio: path empty, formatted duration empty, size 0 and seconds 0.
- audio present: valid MP3 path, size positive, formatted duration valid and seconds 1–86400.
- parse formatted duration and allow at most one second difference from audio_duration_seconds.

Add a small parseAudioDurationSeconds helper beside validation with a comment explaining compatibility with MM:SS and HH:MM:SS.

- [ ] **Step 4: Run focused tests**

Run:
- cd backend && go test ./internal/platform/database -run AudioNovelDistributionMigration -v
- cd backend && go test ./internal/modules/audionovel -run 'Audio|Duration' -v

Expected: PASS.

### Task 2: Implement audio campaign link CRUD and locking rules

**Files:**
- Create: backend/internal/modules/audionovel/distribution.go
- Create: backend/internal/modules/audionovel/distribution_admin.go
- Modify: backend/internal/modules/audionovel/handler.go
- Modify: backend/internal/modules/links/model.go
- Modify: backend/internal/modules/links/repository.go
- Modify: backend/internal/modules/meta/pixels.go
- Modify: backend/internal/modules/tiktok/configuration.go
- Modify: backend/internal/transport/http/router.go
- Create: backend/internal/app/audio_novel_distribution_test.go

**Interfaces:**
- Produces: GET/POST /api/v1/audio-novel-links and PATCH/DELETE /api/v1/audio-novel-links/:id.
- Reuses: novel distribution platform validation, generated code rules, Pixel ownership and reference checks.

- [ ] **Step 1: Add failing API integration tests**

Cover:
- create Meta link with generated code and default threshold 10.
- create TikTok link with explicit globally unique code.
- reject both Pixels, missing selected Pixel, disabled Pixel, unknown audio novel and MP3-less audio novel.
- reject threshold below 0 or above 3600.
- list only product_type=audio_novel and never leak novel or legacy links.
- allow binding/platform/Pixel changes before the first normal visit.
- after first normal visit reject changes to audio_novel_id, ad_platform and Pixel, while allowing name, enabled and threshold.
- delete only when no visits; otherwise return conflict and allow disable.
- prevent Meta/TikTok Pixel deletion while an audio campaign link references it.

Run: cd backend && go test ./internal/app -run AudioNovelDistribution -v

Expected: FAIL with missing routes.

- [ ] **Step 2: Add focused request and response models**

Use direct audionovel-owned models:

~~~go
type DistributionInput struct {
    Name               string
    Code               string
    AudioNovelID       int64
    Enabled            bool
    AdPlatform         string
    MetaConnectionID   *int64
    MetaPixelID        *int64
    TikTokPixelID      *int64
    TimeSpentThreshold int
}

type DistributionLink struct {
    links.Link
    AudioNovelTitle string
    VisitCount      int64
    FirstVisitedAt  *time.Time
    PublicURL       string
}
~~~

JSON tags are required in the implementation. Keep attribution_mode fixed to dynamic and set channel/campaign/adset/ad fields to empty server-side even if a crafted client sends them.

- [ ] **Step 3: Implement validation and transactions**

Repository methods:
- ListDistributionLinks(ctx, audioNovelID).
- CreateDistributionLink(ctx, input, actor).
- UpdateDistributionLink(ctx, id, input, actor).
- DeleteDistributionLink(ctx, id, actor).

Within Create/Update:
- SELECT the audio novel FOR SHARE and require enabled, deleted_at null, audio_path non-empty.
- validate platform-specific Pixel and active connection.
- use existing global code generator and unique constraint.
- treat the first normal GET click as the lock boundary.
- write audit records with secret-free snapshots.

Do not call the generic link update path because it can overwrite product-specific fields.

- [ ] **Step 4: Register protected admin routes**

Register exact routes:
- GET /api/v1/audio-novel-links
- POST /api/v1/audio-novel-links
- PATCH /api/v1/audio-novel-links/:id
- DELETE /api/v1/audio-novel-links/:id

Use audio_novel_id as a required GET filter for list. Return validation errors as 400, not found as 404 and locked/history conflicts as 409.

- [ ] **Step 5: Run focused tests**

Run: cd backend && go test ./internal/app -run 'AudioNovelDistribution|MetaDelete|TikTok' -v

Expected: PASS.

### Task 3: Freeze attribution and direct-open the bound audio player

**Files:**
- Modify: backend/internal/modules/links/model.go
- Modify: backend/internal/modules/links/repository.go
- Modify: backend/internal/modules/tracking/handler.go
- Modify: backend/internal/modules/tracking/repository.go
- Modify: backend/internal/modules/audionovel/render.go
- Modify: backend/internal/modules/audionovel/render_test.go
- Modify: backend/internal/modules/audionovel/public.go
- Modify: backend/internal/app/audio_novel_distribution_test.go
- Modify: backend/internal/app/routes_test.go

**Interfaces:**
- Consumes: GET /audio-novel/:code.
- Produces: one surface=audio_novel click plus bootstrap entry_audio_slug, visit ticket, selected platform public config and threshold.

- [ ] **Step 1: Add failing entry tests**

Test:
- two codes bound to the same audio novel create clicks with different link_id values and the same frozen audio_novel_id.
- a new audio link renders bootstrap with entry_audio_slug and does not redirect through a second HTTP request.
- HEAD, bot and suspicious requests stay observable but do not freeze the campaign.
- the first normal GET freezes binding, platform, Pixel, threshold and dynamic query parameters.
- disabled/deleted/MP3-less content returns 410 and does not create a normal campaign visit.
- restoring enabled content and MP3 makes the same link work again.
- product_type=novel cannot enter audio surface.
- legacy audio code still renders the public audio home and has no entry_audio_slug.
- repeated SPA route changes do not create another click.

Run: cd backend && go test ./internal/app -run 'AudioNovelDistributionEntry|Routes' -v

Expected: FAIL.

- [ ] **Step 2: Extend the canonical link and tracking record**

Add AudioNovelID to links.Link and every scan list in links.Repository.

Add Repository.FreezeAudioNovelLink(ctx, linkID) with one transaction:
1. lock the short_links row.
2. verify product_type, enabled, audio binding, platform and Pixel.
3. verify bound content is enabled, not deleted and has an MP3.
4. set first_visited_at if null without changing already frozen meaning.
5. return not found when content is unavailable.

Extend click insert input with the frozen audio_novel_id and current threshold/platform fields. Continue saving only normalized dynamic attribution values and encrypted TikTok context.

- [ ] **Step 3: Keep surface isolation explicit**

In tracking.Handler.track:
- novel links only allow novel surface.
- new audio_novel links only allow audio_novel surface.
- legacy links keep existing surfaces.
- only a normal GET for a product-specific link invokes the freeze method.

Do not generalize the novel/audio checks into an opaque provider function; two short branches are clearer and safer.

- [ ] **Step 4: Extend bootstrap**

For a new audio campaign return only public values:
- entry_audio_slug.
- playback_ticket.
- playback_threshold_seconds.
- ad_platform.
- Meta public pixel ID or TikTok public pixel code, never both.
- PageView event ID.
- feature-enabled boolean needed to decide whether the browser SDK loads.

The ticket payload must bind visit_id, link_id, audio_novel_id, issued_at and expiry. Reuse the current server signing secret and constant-time validation.

- [ ] **Step 5: Run focused tests**

Run:
- cd backend && go test ./internal/app -run 'AudioNovelDistributionEntry|Routes' -v
- cd backend && go test ./internal/modules/audionovel -run Render -v

Expected: PASS.

### Task 4: Add idempotent playback endpoints and advertising events

**Files:**
- Create: backend/internal/modules/audionovel/playback.go
- Create: backend/internal/modules/audionovel/playback_test.go
- Modify: backend/internal/modules/audionovel/handler.go
- Modify: backend/internal/modules/meta/events.go
- Modify: backend/internal/modules/meta/model.go
- Modify: backend/internal/modules/tiktok/events.go
- Modify: backend/internal/modules/tiktok/model.go
- Modify: backend/internal/modules/tiktok/handler.go
- Modify: backend/internal/transport/http/router.go
- Modify: backend/internal/app/meta_delivery_rules_test.go
- Modify: backend/internal/app/tiktok_test.go
- Modify: backend/internal/app/audio_novel_distribution_test.go

**Interfaces:**
- Produces:
  - POST /audio-novel/:code/start-listening
  - POST /audio-novel/:code/playback-time
  - POST /audio-novel/:code/complete
- Returns confirmed_events entries containing name and event_id only after server state is durable.

- [ ] **Step 1: Write the playback service tests first**

Use a test clock and concurrent goroutines. Cover:
- valid ticket and wrong/expired/tampered/cross-code tickets.
- first start sets audio_started_at and returns audio_visit_start once.
- repeated and concurrent starts return the same ID without duplicate outbox rows.
- playback 5 then 12 advances monotonically and qualifies a 10-second link once.
- playback 12 then 8 does not decrease or requeue.
- a forged 1000 seconds shortly after entry is clipped to server-observed lifetime.
- media_consumed_seconds is capped by playback_seconds times the allowed maximum playback rate.
- threshold 0 never creates ViewContent.
- complete before ended proof or below 90% fails.
- valid completion sets audio_completed_at once and creates no Meta/TikTok event.
- disabled Pixel/connection or downstream广告 API失败不阻断内部播放状态；数据库 outbox 写入失败则与达标状态一起回滚，客户端重试后再原子完成。

Run: cd backend && go test ./internal/modules/audionovel -run Playback -v

Expected: FAIL because the service is absent.

- [ ] **Step 2: Implement request contracts and server clipping**

~~~go
type PlaybackUpdate struct {
    Ticket               string
    PlaybackSeconds      int
    MediaConsumedSeconds float64
}

type PlaybackResult struct {
    PlaybackSeconds      int
    MediaConsumedSeconds float64
    Started              bool
    Qualified            bool
    Completed            bool
    ConfirmedEvents      []ConfirmedEvent
}
~~~

The complete request additionally carries ended=true, but the server still validates stored counters and duration.

Within a transaction:
1. resolve code and validate signed ticket.
2. SELECT the click event FOR UPDATE and verify normal GET, surface=audio_novel, matching link/content.
3. clamp incoming counters to monotonic values and server-observed lifetime, maximum 24 hours.
4. persist state changes.
5. insert the selected platform outbox event with ON CONFLICT DO NOTHING.
6. commit, then return deterministic event IDs.

Use time.Now only through an injectable clock in service tests.

- [ ] **Step 3: Add focused Meta and TikTok enqueue methods**

Add small platform-specific methods rather than a generic provider interface:

~~~go
func (s *Service) EnqueueAudioEvent(
    ctx context.Context,
    tx pgx.Tx,
    visitID string,
    eventName string,
    eventID string,
    input AudioEventInput,
) error
~~~

For TikTok include linkID and return the browser event description when configuration is active.

Meta audio server events:
- PageView.
- StartListening.
- ViewContent.

TikTok audio server events:
- StartListening.
- ViewContent.

TikTok PageView 只由浏览器 Pixel 发送；不要额外创建 Events API PageView outbox，以免与现有小说策略不一致。

Payload content fields:
- content_id = audio_novel:{id}.
- content_name = frozen title.
- content_type = audio_novel.

Do not send value, currency, phone, email or purchase data.

If platform sending is disabled or its connection is inactive:
- keep internal started/qualified state and the deterministic outbox row.
- let the existing worker leave delivery pending/paused without network sending.
- do not return 5xx to the player for a downstream delivery failure.
- expose the blocked configuration reason in admin stats; do not invent a new database event status that violates the existing status constraint.

If inserting the outbox row itself fails, roll back the state transition in the same transaction and let the bounded client retry. This preserves the invariant that a durable qualified state never loses its matching server event.

- [ ] **Step 4: Register public routes with current abuse controls**

Use the existing origin, method, rate-limit and JSON body-size protections. pagehide Beacon requests must be accepted with application/json or text/plain JSON body. Never place the ticket in the URL or logs.

- [ ] **Step 5: Run focused tests**

Run:
- cd backend && go test ./internal/modules/audionovel -run Playback -v
- cd backend && go test ./internal/app -run 'AudioNovelDistribution|MetaDelivery|TikTok' -v

Expected: PASS.

### Task 5: Implement the H5 playback state machine and exclusive browser Pixel

**Files:**
- Create: audio-novel/src/lib/playback.js
- Create: audio-novel/src/lib/tiktok.js
- Modify: audio-novel/src/lib/api.js
- Modify: audio-novel/src/lib/meta.js
- Modify: audio-novel/src/lib/routes.js
- Modify: audio-novel/src/bootstrap.js
- Modify: audio-novel/src/App.vue
- Modify: audio-novel/src/views/AudioDetailView.vue
- Create: audio-novel/tests/playback.test.js
- Create: audio-novel/tests/tiktok.test.js
- Modify: audio-novel/tests/api.test.js
- Modify: audio-novel/tests/audio_ui.test.js
- Modify: audio-novel/tests/router.test.js

**Interfaces:**
- Consumes: bootstrap entry_audio_slug, selected platform data, ticket and playback endpoints.
- Produces: monotonic playback updates, confirmed browser events and direct player routing.

- [ ] **Step 1: Add deterministic fake-clock tests**

Create a pure createPlaybackTracker dependency-injected with now, report and current playbackRate.

Test exact timelines:
- playing for 4 seconds, waiting 3 seconds, playing 6 seconds reports 10 wall-clock seconds.
- hidden/pagehide while still playing continues time.
- pause and ended stop time.
- seeking settles the current segment, seeked resumes only if the media is still playing.
- jumping currentTime from 5 to 200 does not add 195 seconds.
- playbackRate 2 adds 2 media seconds per 1 wall second but only 1 playback second.
- duplicate playing/pause events are idempotent.
- interval report and pagehide final report never submit a lower cumulative value.

Run: cd audio-novel && node --test --test-name-pattern=playback tests/playback.test.js

Expected: FAIL because playback.js is absent.

- [ ] **Step 2: Implement the state machine**

Keep a single state object:
- active boolean.
- segmentStartedAt.
- playbackSeconds.
- mediaConsumedSeconds.
- confirmedPlaybackSeconds.
- reportInFlight.
- pendingReport.

Settle an active segment using performance.now. Attach only these audio events:
- playing starts.
- pause, waiting, stalled, seeking and ended settle and stop.
- seeked resumes only when audio.paused is false and audio.ended is false.

Use a 10-second timer while the component is mounted. On pagehide call navigator.sendBeacon with the cumulative values and ticket. visibilitychange must not stop playback timing.

- [ ] **Step 3: Direct-route a bound entry**

When bootstrap.entry_audio_slug is present and the initial route is the campaign root, router.replace to audioDetailPath(code, slug). Do not call location.replace and do not trigger another entry request.

Legacy bootstrap without a bound slug remains on HomeView.

- [ ] **Step 4: Add platform-exclusive SDK behavior**

App.vue:
- Meta link loads only Meta Pixel.
- TikTok link loads only TikTok Pixel.
- feature disabled or missing public Pixel loads neither.
- PageView fires once after a valid bootstrap.

AudioDetailView.vue:
- on first server-confirmed StartListening, dispatch the matching browser event once.
- on server-confirmed ViewContent, dispatch the matching browser event once.
- use the exact event_id returned by the server.
- platform SDK exceptions are caught and never stop playback or reporting.

Persist sent event IDs only for the current document in a Set; server idempotency remains authoritative across retries.

- [ ] **Step 5: Run H5 tests and build**

Run:
- cd audio-novel && npm test
- cd audio-novel && npm run build

Expected: PASS.

### Task 6: Implement link-isolated playback statistics

**Files:**
- Create: backend/internal/modules/audionovel/distribution_stats.go
- Modify: backend/internal/modules/audionovel/distribution_admin.go
- Modify: backend/internal/modules/audionovel/handler.go
- Modify: backend/internal/transport/http/router.go
- Modify: backend/internal/app/audio_novel_distribution_test.go

**Interfaces:**
- Produces: POST /api/v1/audio-novel-links/:id/stats.
- Filters: start, end, tz, pagination and platform-specific dynamic attribution IDs.

- [ ] **Step 1: Add failing aggregate and detail tests**

Seed two links for the same audio novel and assert isolation for:
- visits.
- unique_visitors by anonymous cookie.
- average/total visible page seconds.
- start count and unique starters.
- qualified count and unique qualified visitors.
- completed count and unique completers.
- average/total playback seconds.
- start rate, qualified rate and completion rate.
- Meta/TikTok event delivery status.
- normal GET only; HEAD, bots and suspicious rows excluded.
- date/timezone boundaries.
- Meta ad_id and TikTok campaign_id, adgroup_id, creative_id, ad_id_v2 filters.
- detail pagination and descending visit order.
- ttclid masking, never the complete value.

Run: cd backend && go test ./internal/app -run AudioNovelDistributionStats -v

Expected: FAIL.

- [ ] **Step 2: Implement one aggregate query and one detail query**

Use click_events as the source of truth. Filter by link_id, audio_novel_id, surface=audio_novel, method=GET and classification=normal.

Return summary fields:
- visits, unique_visitors.
- average_visible_seconds, total_visible_seconds.
- started_count, started_visitors, start_rate.
- qualified_count, qualified_visitors, qualified_rate.
- completed_count, completed_visitors, completion_rate.
- average_playback_seconds, total_playback_seconds.

Use explicit denominators:
- start_rate = started_count / visits.
- qualified_rate = qualified_count / started_count.
- completion_rate = completed_count / started_count.
- average_playback_seconds averages only visits with audio_started_at set; total_playback_seconds sums all stored playback seconds.

Join only the selected platform event table/status needed for the link. Avoid multiplying clicks through one-to-many joins; aggregate event status in a lateral subquery or pre-aggregated CTE.

- [ ] **Step 3: Register and test the route**

Return 404 if the ID is not an audio campaign link. Validate timezone through the existing allowlist and cap page_size at 100.

Run: cd backend && go test ./internal/app -run AudioNovelDistributionStats -v

Expected: PASS.

### Task 7: Add the audio campaign management UI

**Files:**
- Create: frontend/src/api/audioNovelLinks.js
- Create: frontend/src/views/AudioNovelLinkListView.vue
- Modify: frontend/src/views/AudioNovelListView.vue
- Modify: frontend/src/views/AudioNovelFormView.vue
- Modify: frontend/src/router/index.js
- Modify: frontend/src/layouts/AdminLayout.vue
- Create: frontend/tests/audio_novel_links.test.js
- Modify: frontend/tests/audio_novels.test.js
- Modify: frontend/tests/routes.test.js
- Modify: frontend/tests/admin_navigation.test.js

**Interfaces:**
- Admin route: /admin/audio-novels/:id/links.
- API: audio-novel-links CRUD.

- [ ] **Step 1: Add failing source and request-contract tests**

Assert:
- audio list has 投放链接 action.
- route name audio-novel-links exists.
- create defaults platform=meta, threshold=10, enabled=true.
- form renders only link name, code, bound audio novel, platform, matching Pixel, threshold and enabled.
- form does not render attribution mode, channel, campaign ID, ad group ID or ad ID inputs.
- switching platform clears the other platform Pixel IDs.
- visited links disable content/platform/Pixel controls.
- TikTok template copy uses the existing dynamic macro builder with /audio-novel/{code}.
- delete disabled after first visit.
- request payload contains no hidden attribution fields supplied by stale form state.

Run: cd frontend && node --test --test-name-pattern='audio novel link' tests/audio_novel_links.test.js

Expected: FAIL.

- [ ] **Step 2: Implement the API module and page**

Follow NovelLinkListView visual language but keep the audio wording:
- 《title》语音小说投放链接.
- 达标播放时长.
- 开始收听/播放达标.
- one link per投手 or投放.

The table shows:
- name and public URL.
- platform and Pixel.
- playback threshold.
- visits.
- status.
- first visit.
- copy URL, copy TikTok template, statistics, edit, enable/disable and delete.

Use the existing selected Pixel helpers and avoid new wrappers.

- [ ] **Step 3: Add navigation**

AudioNovelListView action opens the selected audio novel links route. Keep the existing menu hierarchy for three frontend projects; do not merge audio and text novel menu items.

- [ ] **Step 4: Format, test and build**

Run:
- cd frontend && npm run format
- cd frontend && npm run format:check
- cd frontend && npm test
- cd frontend && npm run build

Expected: PASS.

### Task 8: Add playback funnel and delivery diagnostics UI

**Files:**
- Create: frontend/src/views/AudioNovelLinkStatsView.vue
- Modify: frontend/src/api/audioNovelLinks.js
- Modify: frontend/src/router/index.js
- Modify: frontend/src/utils/tiktok.js
- Modify: frontend/src/views/MetaEventsView.vue
- Modify: frontend/src/views/TikTokEventsView.vue
- Modify: frontend/tests/audio_novel_links.test.js
- Modify: frontend/tests/meta.test.js
- Modify: frontend/tests/tiktok_api.test.js
- Modify: frontend/tests/routes.test.js

**Interfaces:**
- Admin route: /admin/audio-novel-links/:id/stats.
- Uses the Task 6 stats response.

- [ ] **Step 1: Add failing UI contract tests**

Assert cards and detail columns exist for:
- visits and anonymous UV.
- page visible duration.
- playback duration.
- start count/rate.
- qualified count/rate.
- completion count/rate.
- platform delivery state.

Assert the page wording says:
- platform API accepted does not prove final ad attribution.
- completion is internal only.
- missing historic duration displays 未采集, not zero.
- ttclid is masked.

- [ ] **Step 2: Build the stats page**

Reuse the simple date/timezone and platform filters from NovelLinkStatsView, but name the funnel for listening. Use a responsive table with:
- visit time, visitor, location, device/browser.
- source and dynamic ad fields.
- page visible duration and playback duration.
- started, qualified and completed tags.
- selected platform event status.

Do not expose encrypted context or complete click IDs.

- [ ] **Step 3: Add event labels**

Meta/TikTok event pages must label audio StartListening and audio ViewContent distinctly from text novel events using content_type or audio_novel_id. Keep raw platform event names visible for diagnostics.

- [ ] **Step 4: Format, test and build**

Run:
- cd frontend && npm run format
- cd frontend && npm run format:check
- cd frontend && npm test
- cd frontend && npm run build

Expected: PASS.

### Task 9: Run end-to-end regression and document the operator workflow

**Files:**
- Modify: README.md
- Modify: backend/internal/app/audio_novel_distribution_test.go
- Modify: backend/internal/app/compat_test.go
- Modify: backend/internal/app/routes_test.go
- Modify: backend/internal/app/link_stats_test.go
- Modify: audio-novel/tests/content.test.js

**Interfaces:**
- Verifies all three frontend products and existing public/admin routes.
- Does not deploy or change production data.

- [ ] **Step 1: Add a single integration journey**

The test creates:
1. one enabled MP3 audio novel.
2. one Meta campaign link and one TikTok campaign link bound to it.
3. separate anonymous visitors for each link.
4. PageView, StartListening, sub-threshold playback, qualifying playback and completion.
5. independent stats and deterministic event IDs.

Assert:
- each visit stays under its original link.
- audio_{visit_id}_view, audio_{visit_id}_start and audio_{visit_id}_qualified are stable.
- completion creates no additional advertising event.
- Meta has server PageView, StartListening and ViewContent; TikTok has browser PageView plus server StartListening and ViewContent, with no TikTok server PageView row.
- a duplicate playback update changes neither counts nor outbox.
- platform failure leaves playback and stats intact.

- [ ] **Step 2: Add regression coverage**

Explicitly verify:
- legacy /audio-novel/{code} still opens public home.
- public audio list/detail and story detail still work.
- generic link API never returns audio campaign links.
- text novel link stats and TikTok events still work.
- removing MP3 produces 410 for the bound campaign and re-upload restores it.
- Cookie mode and visitor identifiers follow existing behavior.
- disabled TikTok feature flag never loads TikTok SDK and does not affect Meta.

- [ ] **Step 3: Document the operator workflow**

README additions:
- create or upload the MP3 first.
- open 语音小说管理 → 投放链接.
- create a separate link per投手/campaign.
- choose exactly one platform and Pixel.
- explain default 10-second actual-play threshold.
- explain dynamic URL templates and that ttclid comes from TikTok.
- explain internal completion vs platform API accepted vs final Ads Manager attribution.
- explain legacy links and content unavailable recovery.

Do not include access tokens, production host credentials or real visitor identifiers.

- [ ] **Step 4: Run the complete verification suite**

Run:
- cd backend && go test ./...
- cd backend && go vet ./...
- cd frontend && npm run format:check
- cd frontend && npm test
- cd frontend && npm run build
- cd audio-novel && npm test
- cd audio-novel && npm run build
- cd novel-h5 && npm test
- cd novel-h5 && npm run build
- git diff --check

Expected: every command exits 0.

- [ ] **Step 5: Perform local browser acceptance**

With local backend, admin and audio H5:
- create Meta and TikTok links for the same audio novel.
- confirm each public link opens the bound player directly.
- play, pause, seek, background and resume; verify counters do not inflate.
- verify StartListening and ViewContent appear once.
- verify completion appears only in internal stats.
- disable content/remove MP3 and see 410, then restore.
- simulate blocked Meta/TikTok SDK and confirm playback remains usable.
- check desktop and mobile admin layouts.

Record the tested local URLs and outcomes in the final implementation report. Do not send real production platform events during acceptance.

## Final Review Checklist

- [ ] Database migration is additive and succeeds on existing legacy, novel and TikTok data.
- [ ] New audio links are isolated from generic and text-novel links.
- [ ] First normal GET freezes content/platform/Pixel/threshold/attribution.
- [ ] Real playback time cannot be increased by seeking, duplicated events or backward reports.
- [ ] Background playback counts only while the audio is actually playing.
- [ ] StartListening and ViewContent are exactly-once with browser/server event ID parity.
- [ ] Completion is internal-only and requires ended plus 90% media consumption.
- [ ] Meta-only links never load TikTok; TikTok-only links never load Meta.
- [ ] Platform failure never blocks playback or internal stats.
- [ ] Old audio links still open the public archive.
- [ ] Admin forms hide manual attribution fields and stats clearly separate internal, accepted and attributed meanings.
- [ ] No secrets or full click identifiers are exposed.
- [ ] All backend and three frontend regression suites pass.
- [ ] No unrelated file changes, Git commit, push, PR or production deployment were performed.
