# TikTok 小说网页事件与广告归因 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为免费小说投放链接增加 Meta/TikTok 二选一配置、TikTok Pixel + Events API 双通道事件、动态广告归因和独立统计，同时保持现有 Meta、普通短链、语音小说和多语言阅读链路不变。

**Architecture:** 保留现有 `meta` 模块，新增职责单一的 `tiktok` 模块；小说入口仍由 `click_events` 作为访问事实表，并在第一次正常 GET 时冻结平台、Pixel 和归因快照。H5 复用现有前台可见计时器，`StartReading` 与达标 `ViewContent` 都先由签名接口幂等落库，再使用相同事件 ID 发送浏览器 Pixel；服务端事件由加密 outbox 和带 fencing token 的 Worker 异步发送。

**Tech Stack:** Go 1.27.1、Gin、pgx/PostgreSQL、Vue 3（JavaScript）、Vue Router、Element Plus、Node `node:test`、TikTok Pixel、TikTok Events API v1.3。

**Spec:** `docs/superpowers/specs/2026-09-24-tiktok-novel-attribution-design.md`

## Global Constraints

- 每条小说投放链接的 `ad_platform` 只能是 `meta` 或 `tiktok`；不能同时绑定两个平台的 Pixel。
- 现有小说投放链接迁移后为 `meta`，现有 Meta 行为、事件和统计不得被改写。
- TikTok 只采用动态归因；运营后台不显示手动渠道、Campaign、Ad Group 或广告 ID 输入项。
- 主转化固定为达到链接阈值后的标准事件 `ViewContent`；默认阈值 10 秒，允许 5–3600 秒，`0` 关闭回传但保留停留统计。
- `StartReading` 和 `ViewContent` 的 Pixel 与 Events API 必须使用相同事件名和确定性 `event_id`。
- Access Token、IP、User-Agent、完整页面 URL 和完整匹配数据只在服务端加密保存，不进入 H5 启动数据、普通日志或管理 API。
- TikTok SDK/API/配置异常不能阻塞小说首页、简介页、章节页、语言切换和内部统计。
- 自动重试最长 24 小时；`accepted` 只表示 TikTok API 接收，不能显示为“TikTok 已归因”。
- `TIKTOK_ENABLED=false` 时不加载 TikTok Pixel、不启动 Worker；Meta 链路保持运行。
- 所有新增或修改代码写必要的中文或英文注释，说明业务约束或安全原因；不为简单赋值增加噪声注释。
- 不执行 `git commit`、`git push` 或创建 GitHub PR；每个任务以测试结果作为复查点。
- 保持实现直接、清晰，不新增通用广告 Provider 框架，不为两个平台过度抽象。

## File Structure

### Backend

- Create `backend/internal/platform/database/migrations/022_tiktok_novel_attribution.sql`: TikTok 配置、事件 outbox、链接平台及访问快照迁移。
- Create `backend/internal/platform/database/tiktok_migration_test.go`: 增量迁移、历史 Meta 默认值、约束和索引验证。
- Create `backend/internal/modules/tiktok/model.go`: Connection、Pixel、EventRecord 及校验模型。
- Create `backend/internal/modules/tiktok/crypto.go`: 使用现有服务端密钥环和 TikTok 专属 AAD 加解密。
- Create `backend/internal/modules/tiktok/client.go`: Events API v1.3 请求、响应、错误分类和 `Retry-After`。
- Create `backend/internal/modules/tiktok/client_test.go`: 成功、业务错误、429、5xx、超时、重定向和响应体上限测试。
- Create `backend/internal/modules/tiktok/configuration.go`: Connection/Pixel CRUD、引用删除规则和 Pixel 测试事件。
- Create `backend/internal/modules/tiktok/events.go`: 访问快照解密、事件 payload、幂等入队和人工重试。
- Create `backend/internal/modules/tiktok/event_worker.go`: 租约、fencing token、24 小时截止、退避和发送状态。
- Create `backend/internal/modules/tiktok/lifecycle.go`: 幂等 Start/Close 和总开关。
- Create `backend/internal/modules/tiktok/handler.go`: `/api/v1/tiktok-*` 管理接口。
- Create `backend/internal/modules/tiktok/tiktok_test.go`: 加密、CRUD、事件幂等、Worker 竞争和重试单元测试。
- Modify `backend/internal/config/config.go`: 增加 `TikTokEnabled` 和可注入的 Events API URL。
- Modify `backend/internal/modules/links/model.go`: 链接平台和 TikTok Pixel 字段。
- Modify `backend/internal/modules/tracking/handler.go`: 解析 TikTok 参数、正常 GET 冻结和加密访问上下文。
- Modify `backend/internal/modules/tracking/repository.go`: 持久化冻结的 TikTok 快照。
- Modify `backend/internal/modules/novel/distribution.go`: 平台互斥校验、锁定规则、列表字段和 TikTok 模板。
- Modify `backend/internal/modules/novel/handler.go`: `_ttp` 补充、`StartReading` 签名接口。
- Modify `backend/internal/modules/novel/reading_time.go`: 在一个事务中更新可见时长并生成达标事件。
- Modify `backend/internal/modules/novel/render.go`: 互斥启动数据和 TikTok CSP 白名单。
- Modify `backend/internal/modules/novel/distribution_stats.go`: 阅读漏斗、事件状态和 TikTok 归因明细。
- Modify `backend/internal/bootstrap/app.go`: 创建并注入 TikTok Service。
- Modify `backend/internal/transport/http/router.go`: 注册 TikTok 管理和公共事件路由。
- Modify `backend/cmd/server/main.go`: 按总开关启动 TikTok Worker 并在退出时关闭。
- Modify `backend/internal/app/novel_distribution_test.go`: 平台、Pixel、锁定、归因和统计集成测试。
- Create `backend/internal/app/tiktok_test.go`: 管理 API、公共事件、签名、幂等和回归集成测试。

### Novel H5

- Create `novel-h5/src/lib/tiktok.js`: Pixel 初始化、事件队列、`_ttp` 读取和幂等发送。
- Create `novel-h5/tests/tiktok.test.js`: SDK 单次加载、事件 ID、失败降级和 Cookie 测试。
- Modify `novel-h5/src/lib/timeSpent.js`: 解析达标接口 JSON，并把服务器确认事件交给调用方。
- Modify `novel-h5/src/App.vue`: 平台互斥初始化、访问确认和达标回传。
- Modify `novel-h5/src/views/ReaderView.vue`: 第一章成功加载后幂等报告 `StartReading`。
- Modify `novel-h5/tests/tracking.test.js`: Meta/TikTok 互斥和章节进入回归测试。

### Admin Frontend

- Create `frontend/src/api/tiktok.js`: TikTok Connection、Pixel、测试和事件接口。
- Create `frontend/src/utils/tiktok.js`: 动态宏模板和脱敏显示辅助函数。
- Create `frontend/src/views/TikTokConnectionsView.vue`: 集中凭证管理。
- Create `frontend/src/views/TikTokPixelsView.vue`: Pixel 管理和测试。
- Create `frontend/src/views/TikTokEventsView.vue`: 事件筛选、状态、诊断和人工重试。
- Create `frontend/tests/tiktok_api.test.js`: 请求契约和模板单元测试。
- Modify `frontend/src/api/novelLinks.js`: 平台互斥 payload 和默认动态归因。
- Modify `frontend/src/views/NovelLinkListView.vue`: 平台选择、对应 Pixel、阈值和两种复制地址。
- Modify `frontend/src/views/NovelLinkStatsView.vue`: 阅读漏斗、TikTok 状态和归因明细。
- Modify `frontend/src/router/index.js`: TikTok 三个管理页面路由。
- Modify `frontend/src/layouts/AdminLayout.vue`: 独立“TikTok 管理”二级菜单。
- Modify `frontend/tests/novel_links.test.js`: 表单、平台和模板行为。
- Modify `frontend/tests/admin_navigation.test.js`: 菜单回归。

## Review Focus

1. **TikTok URL 带未展开宏或超长 `ttclid`：** 未展开宏必须丢弃，超长/控制字符值不能进入数据库或 payload；Task 4 的参数化测试覆盖。
2. **服务端已入队但浏览器响应丢失：** 下一次同票据上报应返回相同确定性事件 ID，H5 在当前文档只发送一次；Task 6 和 Task 7 覆盖。
3. **Pixel 或凭证在访问后停用：** 小说与内部统计继续，Worker 保留 pending 并暂停发送，恢复后在 24 小时内继续；Task 3 和 Task 5 覆盖。
4. **多个 Worker 租约过期后同时完成：** 旧 fencing token 不能覆盖新 Worker 的结果；Task 5 覆盖。
5. **历史 Meta 链接或普通短链接字段为空：** 迁移和新约束不能使启动失败，且不会加载 TikTok SDK；Task 1、Task 4 和 Task 9 覆盖。

---

### Task 1: Add the additive TikTok attribution schema

**Files:**
- Create: `backend/internal/platform/database/migrations/022_tiktok_novel_attribution.sql`
- Create: `backend/internal/platform/database/tiktok_migration_test.go`

**Interfaces:**
- Consumes: existing `short_links`, `click_events`, `novels`, migration runner and `TEST_DATABASE_URL` integration-test convention.
- Produces: `tiktok_connections`, `tiktok_pixels`, `tiktok_events`; `short_links.ad_platform`, `short_links.tiktok_pixel_id`; frozen TikTok columns on `click_events`.

- [ ] **Step 1: Write the migration test and verify it fails before migration 022 exists**

```go
func TestTikTokAttributionMigration(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()
	for _, table := range []string{"tiktok_connections", "tiktok_pixels", "tiktok_events"} {
		var exists bool
		err := db.QueryRow(ctx, `SELECT to_regclass('public.' || $1) IS NOT NULL`, table).Scan(&exists)
		if err != nil || !exists {
			t.Fatalf("expected %s after migrations, exists=%v err=%v", table, exists, err)
		}
	}
	var platform string
	if err := db.QueryRow(ctx, `SELECT column_default FROM information_schema.columns WHERE table_name='short_links' AND column_name='ad_platform'`).Scan(&platform); err != nil {
		t.Fatal(err)
	}
	if platform != "'meta'::text" {
		t.Fatalf("historical links must default to meta, got %q", platform)
	}
}
```

Run: `cd backend && go test ./internal/platform/database -run TikTokAttributionMigration -v`

Expected: FAIL because migration 022 and its tables/columns do not exist.

- [ ] **Step 2: Add the complete additive migration**

Use these exact state names and constraints:

```sql
CREATE TABLE tiktok_connections (
  id bigserial PRIMARY KEY,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  access_token_cipher text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  credential_status text NOT NULL DEFAULT 'unverified'
    CHECK (credential_status IN ('unverified','valid','invalid','error')),
  validated_at timestamptz,
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tiktok_pixels (
  id bigserial PRIMARY KEY,
  connection_id bigint NOT NULL REFERENCES tiktok_connections(id),
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  pixel_code text NOT NULL UNIQUE CHECK (char_length(pixel_code) BETWEEN 5 AND 64),
  test_event_code text NOT NULL DEFAULT '' CHECK (char_length(test_event_code) <= 120),
  enabled boolean NOT NULL DEFAULT true,
  validated_at timestamptz,
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE short_links
  ADD COLUMN ad_platform text NOT NULL DEFAULT 'meta',
  ADD COLUMN tiktok_pixel_id bigint REFERENCES tiktok_pixels(id);

ALTER TABLE short_links ADD CONSTRAINT short_links_ad_platform_check
  CHECK (ad_platform IN ('meta','tiktok'));
ALTER TABLE short_links ADD CONSTRAINT short_links_novel_platform_binding_check
  CHECK (product_type <> 'novel' OR
    (ad_platform='meta' AND meta_pixel_id IS NOT NULL AND meta_connection_id IS NOT NULL AND tiktok_pixel_id IS NULL) OR
    (ad_platform='tiktok' AND tiktok_pixel_id IS NOT NULL AND meta_pixel_id IS NULL AND meta_connection_id IS NULL));

ALTER TABLE click_events
  ADD COLUMN ad_platform text NOT NULL DEFAULT 'meta',
  ADD COLUMN tiktok_pixel_id bigint REFERENCES tiktok_pixels(id),
  ADD COLUMN tiktok_pixel_code text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_ttclid text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_ttp text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_adgroup_id text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_creative_id text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_ad_id_v2 text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_placement text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_context_cipher text NOT NULL DEFAULT '',
  ADD COLUMN tiktok_start_reading_at timestamptz,
  ADD COLUMN tiktok_view_content_at timestamptz;

CREATE TABLE tiktok_events (
  id text PRIMARY KEY,
  visit_id text NOT NULL,
  link_id bigint NOT NULL,
  novel_id bigint,
  connection_id bigint NOT NULL REFERENCES tiktok_connections(id),
  pixel_record_id bigint NOT NULL REFERENCES tiktok_pixels(id),
  pixel_code text NOT NULL,
  event_name text NOT NULL CHECK (event_name IN ('StartReading','ViewContent','PageView')),
  event_id text NOT NULL,
  event_time timestamptz NOT NULL,
  payload_cipher text NOT NULL,
  is_test boolean NOT NULL DEFAULT false,
  status text NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','sending','accepted','retry','failed')),
  attempts integer NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  locked_until timestamptz,
  lock_token text NOT NULL DEFAULT '',
  http_status integer NOT NULL DEFAULT 0,
  business_code bigint NOT NULL DEFAULT 0,
  request_id text NOT NULL DEFAULT '',
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(pixel_code,event_name,event_id)
);

CREATE INDEX tiktok_events_delivery
  ON tiktok_events(status,next_attempt_at,event_time)
  WHERE status IN ('pending','retry','sending');
CREATE INDEX tiktok_events_visit ON tiktok_events(visit_id,event_name);
CREATE INDEX click_events_tiktok_attribution
  ON click_events(link_id,tiktok_ad_id_v2,tiktok_adgroup_id,occurred_at DESC)
  WHERE ad_platform='tiktok' AND surface='novel';
```

Add comments in the migration explaining that TikTok event rows intentionally keep logical IDs instead of foreign keys to visit/link rows so the existing retention job can remove expired visit details without breaking delivery history.

- [ ] **Step 3: Extend the migration test with compatibility and constraint cases**

Test that a pre-existing novel Meta link reads `ad_platform='meta'`, a TikTok novel link with a Meta Pixel is rejected, a Meta link with `tiktok_pixel_id` is rejected, and ordinary legacy links with no Pixel still pass.

Run: `cd backend && go test ./internal/platform/database -run 'TikTokAttributionMigration|Migration' -v`

Expected: PASS, or integration tests SKIP only when `TEST_DATABASE_URL` is absent.

### Task 2: Add TikTok configuration models, encryption and HTTP client

**Files:**
- Create: `backend/internal/modules/tiktok/model.go`
- Create: `backend/internal/modules/tiktok/crypto.go`
- Create: `backend/internal/modules/tiktok/client.go`
- Create: `backend/internal/modules/tiktok/client_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/app/config_test.go`

**Interfaces:**
- Consumes: `runtime.Core`, `Config.MetaEncryptionKeyID`, `Config.MetaEncryptionKeys`, `Config.Secret`.
- Produces: `tiktok.New(core) *Service`, `Client.Post(ctx, token, EventRequest) (DeliveryResponse, error)`, `Service.seal/open`, `Config.TikTokEnabled`, `Config.TikTokEventsURL`.

- [ ] **Step 1: Write failing configuration and client tests**

```go
func TestTikTokConfigDefaultsDisabled(t *testing.T) {
	t.Setenv("TIKTOK_ENABLED", "")
	c := validConfig(t)
	if c.TikTokEnabled {
		t.Fatal("TikTok must be disabled until operators explicitly enable it")
	}
	if c.TikTokEventsURL != "https://business-api.tiktok.com/open_api/v1.3/event/track/" {
		t.Fatalf("unexpected endpoint %q", c.TikTokEventsURL)
	}
}

func TestClientRejectsRedirectWithoutLeakingToken(t *testing.T) {
	token := "secret-access-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com", http.StatusFound)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	_, err := client.Post(context.Background(), token, validRequest())
	if err == nil || strings.Contains(err.Error(), token) {
		t.Fatalf("expected redacted redirect error, got %v", err)
	}
}
```

Run: `cd backend && go test ./internal/modules/tiktok ./internal/app -run TikTok -v`

Expected: FAIL because the package and config fields are missing.

- [ ] **Step 2: Add configuration parsing**

Add fields:

```go
TikTokEnabled   bool
TikTokEventsURL string
```

Parse only `true` or `false` with `strconv.ParseBool`, default false, and default the official endpoint above. Validate that an override is HTTPS, except `http://127.0.0.1` and `http://localhost` used by tests. Add a comment that the token remains in the database and the endpoint override exists only for deterministic tests and controlled proxies.

- [ ] **Step 3: Define exact event request and response models**

```go
type UserContext struct {
	TTCLID    string `json:"ttclid,omitempty"`
	TTP       string `json:"ttp,omitempty"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

type PageContext struct {
	URL      string `json:"url"`
	Referrer string `json:"referrer,omitempty"`
}

type Content struct {
	ContentID string `json:"content_id"`
	Quantity  int    `json:"quantity"`
}

type EventData struct {
	Event      string         `json:"event"`
	EventTime  int64          `json:"event_time"`
	EventID    string         `json:"event_id"`
	User       UserContext    `json:"user"`
	Page       PageContext    `json:"page"`
	Properties map[string]any `json:"properties,omitempty"`
}

type EventRequest struct {
	EventSource   string      `json:"event_source"`
	EventSourceID string      `json:"event_source_id"`
	Data          []EventData `json:"data"`
	TestEventCode string      `json:"test_event_code,omitempty"`
}

type DeliveryResponse struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}
```

The client must set `Access-Token` and `Content-Type: application/json`, use a 10-second timeout, reject redirects, read at most 1 MiB, and return a typed error containing HTTP status, business code, redacted message and parsed `Retry-After`.

- [ ] **Step 4: Implement TikTok-specific encryption**

Use the configured key ring but use these distinct purposes:

```go
func (s *Service) sealToken(token string, connectionID int64) (string, error) {
	return s.seal(token, fmt.Sprintf("tiktok:token:%d", connectionID))
}

func (s *Service) sealEvent(payload string, eventID string) (string, error) {
	return s.seal(payload, "tiktok:event:"+eventID)
}
```

The legacy derived key prefix must be `linkscope:tiktok:encryption:v1:` so a Meta ciphertext cannot be opened by TikTok code. Error messages must not include ciphertext or plaintext.

- [ ] **Step 5: Cover all response classes**

Add tests for HTTP 200 + `code=0`, HTTP 200 + nonzero code, 400, 429 with seconds and HTTP-date `Retry-After`, 500, invalid JSON, oversized response, context timeout and redirect. Assert that tokens and event identifiers are absent from error strings.

Run: `cd backend && go test ./internal/modules/tiktok ./internal/app -run 'TikTok|Client' -v`

Expected: PASS.

### Task 3: Implement centralized TikTok Connection and Pixel management

**Files:**
- Create: `backend/internal/modules/tiktok/configuration.go`
- Create: `backend/internal/modules/tiktok/handler.go`
- Create: `backend/internal/modules/tiktok/configuration_test.go`
- Modify: `backend/internal/transport/http/router.go`

**Interfaces:**
- Consumes: Task 1 tables and Task 2 `Service`, encryption and `Client`.
- Produces: authenticated `GET/POST/PATCH/DELETE /api/v1/tiktok-connections`, `GET/POST/PATCH/DELETE /api/v1/tiktok-pixels`, `POST /api/v1/tiktok-pixels/:id/test`.

- [ ] **Step 1: Write failing CRUD and secret-redaction tests**

Cover: blank/long name, missing token on create, token replacement on edit, response never includes token/ciphertext, duplicate Pixel Code returns 409, immutable Pixel Code/connection after creation, in-use delete returns 409, and disabled configuration remains readable.

```go
func TestConnectionJSONNeverReturnsSecret(t *testing.T) {
	item := Connection{ID: 7, Name: "TikTok Production", Enabled: true, HasAccessToken: true}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "token") && !strings.Contains(string(raw), "has_access_token") {
		t.Fatalf("unexpected secret-shaped field: %s", raw)
	}
}
```

Run: `cd backend && go test ./internal/modules/tiktok -run 'Connection|Pixel' -v`

Expected: FAIL because CRUD is absent.

- [ ] **Step 2: Implement service models and validation**

Use these public contracts:

```go
type Connection struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Enabled          bool       `json:"enabled"`
	HasAccessToken   bool       `json:"has_access_token"`
	CredentialStatus string     `json:"credential_status"`
	ValidatedAt      *time.Time `json:"validated_at,omitempty"`
	LastError        string     `json:"last_error"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ConnectionInput struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	AccessToken string `json:"access_token"`
}

type Pixel struct {
	ID            int64      `json:"id"`
	ConnectionID  int64      `json:"connection_id"`
	Name          string     `json:"name"`
	PixelCode     string     `json:"pixel_code"`
	TestEventCode string     `json:"test_event_code"`
	Enabled       bool       `json:"enabled"`
	ValidatedAt   *time.Time `json:"validated_at,omitempty"`
	LastError     string     `json:"last_error"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
```

Connection create must insert first, then encrypt with the real numeric ID inside the same transaction. Edits preserve the old cipher when `access_token` is empty. Pixel Code accepts only `^[A-Za-z0-9_-]{5,64}$`; Access Token accepts 1–8192 non-whitespace-control characters.

- [ ] **Step 3: Implement safe deletion rules**

`DeleteConnection` must lock the row and reject deletion when any Pixel or event references it. `DeletePixel` must reject when any short link, click event or TikTok event references it. Return a typed `ConfigInUseError` so the handler returns 409; missing rows return 404. Only unreferenced rows are physically deleted, and audit records contain IDs/names only.

- [ ] **Step 4: Implement Pixel-level send test**

Require a non-empty `test_event_code`. Build one `PageView` test request using event ID `tiktok_test_<runtime token>`, `event_source=web`, the selected Pixel Code and `PublicURL + "/admin/tiktok/pixels"`. Send synchronously with a 10-second context. On success update Pixel `validated_at` and clear `last_error`; on failure save a redacted error. Never persist or return the Access Token.

- [ ] **Step 5: Register authenticated routes and response statuses**

```go
func (h *Handler) Register(api *gin.RouterGroup) {
	api.GET("/tiktok-connections", h.listConnections)
	api.POST("/tiktok-connections", h.createConnection)
	api.PATCH("/tiktok-connections/:id", h.updateConnection)
	api.DELETE("/tiktok-connections/:id", h.deleteConnection)
	api.GET("/tiktok-pixels", h.listPixels)
	api.POST("/tiktok-pixels", h.createPixel)
	api.PATCH("/tiktok-pixels/:id", h.updatePixel)
	api.DELETE("/tiktok-pixels/:id", h.deletePixel)
	api.POST("/tiktok-pixels/:id/test", h.testPixel)
}
```

Run: `cd backend && go test ./internal/modules/tiktok ./internal/app -run 'TikTok|Connection|Pixel' -v`

Expected: PASS.

### Task 4: Add platform-selectable novel links and frozen TikTok attribution

**Files:**
- Modify: `backend/internal/modules/links/model.go`
- Modify: `backend/internal/modules/novel/distribution.go`
- Modify: `backend/internal/modules/novel/distribution_admin.go`
- Modify: `backend/internal/modules/tracking/handler.go`
- Modify: `backend/internal/modules/tracking/repository.go`
- Modify: `backend/internal/app/novel_distribution_test.go`

**Interfaces:**
- Consumes: Task 1 link/snapshot columns and Task 3 Pixel records.
- Produces: `DistributionInput.AdPlatform`, `DistributionInput.TikTokPixelID`, frozen normal-GET visit snapshot, `DistributionLink.TikTokTemplateURL`.

- [ ] **Step 1: Write failing platform, lock and attribution tests**

Add table-driven service/integration tests for:

```go
tests := []struct {
	name      string
	platform  string
	metaPixel *int64
	tikPixel  *int64
	wantError string
}{
	{"meta requires Meta Pixel", "meta", nil, nil, "请选择 Meta Pixel"},
	{"tiktok requires TikTok Pixel", "tiktok", nil, nil, "请选择 TikTok Pixel"},
	{"meta rejects TikTok Pixel", "meta", metaID, tikID, "只能绑定一个广告平台"},
	{"tiktok rejects Meta Pixel", "tiktok", metaID, tikID, "只能绑定一个广告平台"},
}
```

Also verify a HEAD, bot or suspicious visit does not set `first_visited_at`; the first normal GET does; after that, novel/platform/selected Pixel changes return 409. Verify two TikTok codes bound to the same novel preserve different `link_id` and attribution snapshots.

Run: `cd backend && go test ./internal/app ./internal/modules/novel -run 'NovelDistribution|TikTokAttribution' -v`

Expected: FAIL.

- [ ] **Step 2: Extend link and distribution contracts**

Add:

```go
AdPlatform    string `json:"ad_platform"`
TikTokPixelID *int64 `json:"tiktok_pixel_id,omitempty"`
```

For novel links, normalize `AttributionMode="dynamic"`, clear client-supplied Campaign/Ad Set/Ad IDs, and set `Channel` to `facebook` for Meta or `tiktok` for TikTok. Validate the selected Pixel exists; a disabled Pixel may remain on an existing link but cannot be selected for a new link. When `first_visited_at != nil`, reject changes to `novel_id`, `ad_platform`, `meta_pixel_id`, `meta_connection_id` or `tiktok_pixel_id`.

- [ ] **Step 3: Generate the official dynamic TikTok template**

Set `tiktok_template_url` only for TikTok rows:

```go
func TikTokTemplate(publicURL, code string) string {
	return strings.TrimRight(publicURL, "/") + "/novel/" + url.PathEscape(code) +
		"?utm_source=tiktok&utm_medium=paid_social&campaign_id=__CAMPAIGN_ID__" +
		"&adgroup_id=__AID__&creative_id=__CID__&ad_id_v2=__ADID_V2__&placement=__PLACEMENT__"
}
```

Keep `public_url` for both platforms.

- [ ] **Step 4: Parse and sanitize TikTok attribution fields**

Extend the accepted query keys with `ttclid`, `adgroup_id`, `creative_id`, `ad_id_v2`. Use one helper with exact rules:

```go
func resolvedAttributionValue(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > limit || strings.ContainsAny(value, "\r\n") ||
		strings.Contains(value, "__") || strings.Contains(value, "{{") || strings.Contains(value, "}}") {
		return ""
	}
	return value
}
```

Use 2048 for `ttclid`, 512 for other dynamic values. Map TikTok `adgroup_id` independently; do not overwrite it with Meta `adset_id`. `source` becomes `tiktok` when a TikTok link has no resolved source.

- [ ] **Step 5: Freeze only on the first normal GET**

Move classification before `FreezeNovelLink` without writing a visitor Cookie for unavailable content. Call freeze only when `surface=="novel"`, method is GET and classification is `normal`; then reload the link snapshot before recording. Continue to render HEAD/bot/suspicious requests without locking editable link configuration.

- [ ] **Step 6: Encrypt the private visit context and persist one snapshot**

Add a TikTok service method:

```go
type VisitContext struct {
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	PageURL   string `json:"page_url"`
	Referrer  string `json:"referrer"`
}

func (s *Service) SealVisitContext(input VisitContext, visitID string) (string, error)
```

For a normal GET TikTok visit, seal sanitized IP/User-Agent/full landing URL/referrer using AAD `tiktok:visit:<visitID>`. Persist `ad_platform`, Pixel record/code, sanitized ad fields and cipher in the same `INSERT click_events`. For Meta and legacy visits, keep TikTok fields empty.

- [ ] **Step 7: Run focused and regression tests**

Run: `cd backend && go test ./internal/modules/tracking ./internal/modules/novel ./internal/app -run 'Novel|TikTok|Meta' -v`

Expected: PASS; existing Meta tests remain unchanged.

### Task 5: Implement the durable TikTok event outbox and Worker

**Files:**
- Create: `backend/internal/modules/tiktok/events.go`
- Create: `backend/internal/modules/tiktok/event_worker.go`
- Create: `backend/internal/modules/tiktok/lifecycle.go`
- Create: `backend/internal/modules/tiktok/event_worker_test.go`
- Modify: `backend/internal/modules/tiktok/handler.go`

**Interfaces:**
- Consumes: Task 1 `tiktok_events`, Task 2 client/crypto, Task 4 frozen visit snapshot.
- Produces: `QueueVisitEventTx`, `ProcessEvent`, `RetryEvent`, event-list API, idempotent `Start/Close`.

- [ ] **Step 1: Write failing idempotency, retry and fencing tests**

Cover one insert for repeated `(pixel_code,event_name,event_id)`, same original event time on retry, 429/5xx retry, permanent 4xx failure, business-code failure classification, Pixel/Connection disabled pause, 24-hour expiry, two workers competing, expired lease reclaimed, and stale token unable to save.

```go
func TestDeterministicEventIDs(t *testing.T) {
	if got := VisitEventID("visit_123", "StartReading"); got != "novel_visit_123_start" {
		t.Fatalf("unexpected start id %q", got)
	}
	if got := VisitEventID("visit_123", "ViewContent"); got != "novel_visit_123_qualified" {
		t.Fatalf("unexpected qualified id %q", got)
	}
}
```

Run: `cd backend && go test ./internal/modules/tiktok -run 'Event|Worker' -v`

Expected: FAIL.

- [ ] **Step 2: Build encrypted payloads from the frozen visit**

Expose:

```go
type BrowserEvent struct {
	Name    string `json:"name"`
	EventID string `json:"event_id"`
}

func (s *Service) QueueVisitEventTx(
	ctx context.Context,
	tx pgx.Tx,
	visitID string,
	linkID int64,
	eventName string,
	eventAt time.Time,
) (BrowserEvent, bool, error)
```

The method must read the frozen `ad_platform`, Pixel/Connection, novel ID, `ttclid`, `ttp`, encrypted context and status. For non-TikTok visits return `(BrowserEvent{}, false, nil)`. For TikTok, decrypt context, build `event_source=web`, `event_source_id=<pixel code>`, `contents:[{content_id:"novel:<id>",quantity:1}]`, omit money/currency/personal fields, encrypt the final request, and insert with `ON CONFLICT(pixel_code,event_name,event_id) DO NOTHING`. A repeated call returns the same `BrowserEvent` and `created=false`.

- [ ] **Step 3: Implement safe claiming and status updates**

Claim one eligible event with `FOR UPDATE SKIP LOCKED`, set `status='sending'`, increment attempts, set a random `lock_token`, and set `locked_until=now()+interval '2 minutes'`. The save statement must include:

```sql
WHERE id=$1 AND status='sending' AND lock_token=$2
```

Only HTTP success plus `business_code=0` becomes `accepted`. Retry network timeout, 429, 5xx and documented transient business failures with exponential backoff capped at 6 hours and honoring a longer `Retry-After`. Mark non-retryable failures `failed`. Once `event_time <= now()-interval '24 hours'`, mark failed, clear `payload_cipher`, and state that the safe dedup retry window expired.

- [ ] **Step 4: Pause disabled configuration without consuming attempts**

The claim query joins enabled Pixel and Connection rows. Disabled rows remain `pending`/`retry` with payload intact and are not claimed. Add a test that re-enabling resumes delivery while the event is younger than 24 hours.

- [ ] **Step 5: Add list and manual retry APIs**

Register:

```go
api.GET("/tiktok-events", h.listEvents)
api.POST("/tiktok-events/:id/retry", h.retryEvent)
```

Filters: page, status, event_name, pixel_record_id, link_id. Return event ID, event name/time, link ID, novel ID, Pixel name/code, status, attempts, HTTP status, business code, request ID, redacted error and timestamps. `RetryEvent` only accepts failed/retry events younger than 24 hours with a non-empty ciphertext.

- [ ] **Step 6: Add lifecycle control**

`Start` must be idempotent and do nothing when `Config.TikTokEnabled` is false. `Close` cancels and waits. Worker logs must contain task/status/error category only, never token, payload, `ttclid`, `_ttp`, IP, User-Agent or event ciphertext.

Run: `cd backend && go test ./internal/modules/tiktok -run 'Event|Worker|Lifecycle' -v`

Expected: PASS.

### Task 6: Add signed StartReading and qualified ViewContent server flows

**Files:**
- Modify: `backend/internal/modules/novel/handler.go`
- Modify: `backend/internal/modules/novel/reading_time.go`
- Modify: `backend/internal/transport/http/router.go`
- Create: `backend/internal/app/tiktok_test.go`
- Modify: `backend/internal/modules/novel/reading_time_test.go`

**Interfaces:**
- Consumes: existing signed `ticket`, Task 5 `QueueVisitEventTx`.
- Produces: `POST /novel/:code/start-reading`; JSON result from `/view` and `/reading-time`; `_ttp` supplement; transactional `ViewContent` qualification.

- [ ] **Step 1: Write failing public-flow tests**

Test wrong origin, bad ticket, wrong link, non-normal/non-GET visit, `_ttp` invalid/overlong, `_ttp` cannot overwrite, start event only once, threshold not reached, exact threshold reached, repeated/downgraded/forged seconds, 2-hour cap, disabled threshold, and Meta visit generating no TikTok event.

Expected qualified response:

```json
{
  "visible_seconds": 10,
  "tiktok_event": {
    "name": "ViewContent",
    "event_id": "novel_visit123_qualified"
  }
}
```

Run: `cd backend && go test ./internal/modules/novel ./internal/app -run 'Reading|StartReading|TikTok' -v`

Expected: FAIL.

- [ ] **Step 2: Add strict `_ttp` normalization**

Accept `_ttp` only from signed same-origin requests, trim it, reject control characters, enforce 1–512 characters, and ignore unresolved macro syntax. Update only when the frozen visit column is empty:

```sql
UPDATE click_events
SET tiktok_ttp=$3
WHERE id=$1 AND link_id=$2 AND ad_platform='tiktok' AND tiktok_ttp=''
```

The endpoint returns success if a valid existing value is already present; it never overwrites one visit with another browser's Cookie.

- [ ] **Step 3: Implement `StartReading` transaction**

Validate the existing ticket and origin, require `chapter=1`, lock the normal GET novel visit, set `tiktok_start_reading_at=COALESCE(existing,now())`, supplement `_ttp`, call `h.TikTok.QueueVisitEventTx(ctx, tx, eventID, link.ID, "StartReading", startAt)`, and commit. Return the deterministic browser event on both first and idempotent repeated calls. For Meta visits return `{ "ok": true }`.

- [ ] **Step 4: Make visible-time update return the accepted server-clipped value**

Replace the current plain update with a transaction helper:

```go
func (r Repository) UpdateVisibleSecondsTx(
	ctx context.Context,
	tx pgx.Tx,
	eventID string,
	linkID int64,
	reported int,
) (int, error)
```

Use the existing `min(reported,7200,server observed + 5)` rule and `GREATEST` monotonic update, returning the final `visible_seconds`.

- [ ] **Step 5: Generate `ViewContent` in the same transaction**

After the visible-time update, lock the visit. If TikTok, threshold > 0, accepted seconds >= frozen threshold and `tiktok_view_content_at` is null, set the timestamp and queue `ViewContent` before commit. Repeated calls read the existing timestamp/event and return the same browser event without inserting another row. An early or disabled-threshold request returns only `visible_seconds`.

- [ ] **Step 6: Preserve Meta behavior**

Keep `/novel/:code/time-spent` unchanged for Meta links. `/reading-time` continues to collect real visible duration for both platforms; it only adds TikTok qualification when the frozen platform is TikTok.

Run: `cd backend && go test ./internal/modules/novel ./internal/app -run 'Novel|Reading|TimeSpent|TikTok|Meta' -v`

Expected: PASS.

### Task 7: Load TikTok Pixel and send deduplicated H5 events

**Files:**
- Create: `novel-h5/src/lib/tiktok.js`
- Create: `novel-h5/tests/tiktok.test.js`
- Modify: `novel-h5/src/lib/timeSpent.js`
- Modify: `novel-h5/src/App.vue`
- Modify: `novel-h5/src/views/ReaderView.vue`
- Modify: `novel-h5/tests/tracking.test.js`
- Modify: `backend/internal/modules/novel/render.go`
- Modify: `backend/internal/modules/novel/render_test.go`

**Interfaces:**
- Consumes: Task 6 JSON browser event and startup data.
- Produces: one document-scoped TikTok Pixel initialization, `PageView`, `StartReading`, `ViewContent`, `_ttp` supplement and CSP.

- [ ] **Step 1: Write failing TikTok browser tests**

```js
test("TikTok pixel initializes once and shares the server event id", () => {
  const scope = {}, dataset = {}, appended = [];
  const documentRef = fakeDocument(dataset, appended);
  assert.equal(installTikTokPixel({ pixelCode:"C0ABC123", scope, documentRef }), true);
  installTikTokPixel({ pixelCode:"C0ABC123", scope, documentRef });
  trackTikTokEvent({ name:"ViewContent", eventId:"novel_v1_qualified", scope, documentRef });
  trackTikTokEvent({ name:"ViewContent", eventId:"novel_v1_qualified", scope, documentRef });
  assert.equal(appended.filter((node) => node.id === "novel-tiktok-pixel").length, 1);
  assert.equal(dataset.novelTikTokViewContent, "novel_v1_qualified");
});
```

Also test invalid Pixel Code, missing event ID, SDK load failure, `document.cookie` `_ttp` extraction, Meta bootstrap never appends TikTok script, and TikTok bootstrap never appends Meta script.

Run: `cd novel-h5 && node --test tests/tiktok.test.js tests/tracking.test.js`

Expected: FAIL.

- [ ] **Step 2: Extend mutually exclusive startup data**

Add these public fields:

```go
AdPlatform          string `json:"ad_platform"`
TikTokEnabled       bool   `json:"tiktok_enabled"`
TikTokPixelCode     string `json:"tiktok_pixel_code,omitempty"`
TikTokStartEventID  string `json:"tiktok_start_event_id,omitempty"`
TikTokQualifiedID   string `json:"tiktok_qualified_event_id,omitempty"`
```

Meta visits populate only Meta fields. TikTok visits populate Pixel fields only when global switch, frozen Pixel, Connection and Pixel are enabled. Do not expose token, test event code, `ttclid`, `_ttp`, IP, UA or payload.

- [ ] **Step 3: Implement the official Pixel loader with graceful failure**

`installTikTokPixel` validates Pixel Code, installs one `window.ttq`, loads `https://analytics.tiktok.com/i18n/pixel/events.js`, then calls `ttq.load(pixelCode)` and `ttq.page()` exactly once for the current document. `trackTikTokEvent` calls:

```js
scope.ttq.track(name, { content_type:"product", contents:[] }, { event_id:eventId });
```

Use `document.documentElement.dataset` to suppress duplicate event IDs in the current document. Wrap SDK calls so errors return false and never throw into Vue rendering.

- [ ] **Step 4: Make reporting helpers return server-confirmed browser events**

`reportReadingTime` parses JSON for normal fetch and returns `{ ok, visibleSeconds, tiktokEvent }`. Beacon remains fire-and-forget and returns no browser event. The caller advances `lastReported` only after a successful response, keeps one in-flight request, and retries a failed report on a later tick without changing the event ID. Add:

```js
export async function reportStartReading({ code, ticket, ttp, request=fetch } = {})
```

It POSTs `ticket`, `chapter=1`, and optional `ttp` to `/novel/{code}/start-reading`, returning `tiktok_event` only on an OK JSON response.

- [ ] **Step 5: Wire platform-specific behavior in App and ReaderView**

`App.vue` selects one installer based on `bootstrap.ad_platform`. On each periodic reading-time response, call `trackTikTokEvent` only when the response contains a server-confirmed event. Keep the existing Meta threshold callback only for Meta; for TikTok, the threshold callback immediately invokes the same `reportReading()` path so a 15-second threshold is not delayed until the next 10-second periodic report. `ReaderView.vue` calls `reportStartReading` after chapter 1 data is successfully rendered; it passes the signed ticket from injected bootstrap and `_ttp`, and tracks the returned event. Backend idempotency and dataset guards make language reloads and repeated chapter-1 loads safe.

- [ ] **Step 6: Tighten CSP to official hosts only**

Keep current Meta hosts and add only:

```text
script-src https://analytics.tiktok.com
img-src https://analytics.tiktok.com https://business-api.tiktok.com
connect-src https://analytics.tiktok.com https://business-api.tiktok.com
```

Do not use `*.tiktok.com`. Add render tests that nonce/Meta rules remain and startup JSON contains no token.

Run: `cd novel-h5 && npm test && npm run build && npm run format:check`

Expected: PASS.

### Task 8: Add TikTok admin pages and platform-aware novel link form

**Files:**
- Create: `frontend/src/api/tiktok.js`
- Create: `frontend/src/utils/tiktok.js`
- Create: `frontend/src/views/TikTokConnectionsView.vue`
- Create: `frontend/src/views/TikTokPixelsView.vue`
- Create: `frontend/src/views/TikTokEventsView.vue`
- Create: `frontend/tests/tiktok_api.test.js`
- Modify: `frontend/src/api/novelLinks.js`
- Modify: `frontend/src/views/NovelLinkListView.vue`
- Modify: `frontend/src/router/index.js`
- Modify: `frontend/src/layouts/AdminLayout.vue`
- Modify: `frontend/tests/novel_links.test.js`
- Modify: `frontend/tests/admin_navigation.test.js`

**Interfaces:**
- Consumes: Tasks 3–5 admin APIs and Task 4 link payload.
- Produces: central TikTok management UI, platform selector, Pixel selector, ordinary URL and TikTok template copy actions.

- [ ] **Step 1: Write failing API/menu/form tests**

Assert exact API paths, secret omission on edit when left blank, TikTok menu with three children, Meta menu unchanged, link payload clears the other platform Pixel, default threshold 10, dynamic attribution fixed, and hidden manual fields still absent.

```js
test("novel link payload selects exactly one platform", () => {
  const tiktok = novelLinkPayload({
    ad_platform:"tiktok",
    novel_id:7,
    tiktok_pixel_id:9,
    meta_pixel_id:4,
    meta_connection_id:3,
  });
  assert.equal(tiktok.tiktok_pixel_id, 9);
  assert.equal(tiktok.meta_pixel_id, null);
  assert.equal(tiktok.meta_connection_id, null);
  assert.equal(tiktok.attribution_mode, "dynamic");
});
```

Run: `cd frontend && node --test tests/tiktok_api.test.js tests/novel_links.test.js tests/admin_navigation.test.js`

Expected: FAIL.

- [ ] **Step 2: Implement the admin API client and template helper**

Export list/create/update/delete functions for Connections and Pixels, `testTikTokPixel(id)`, `listTikTokEvents(filters)`, and `retryTikTokEvent(id)`. `tiktokTemplateURL(publicURL, code)` must produce the exact Task 4 template so frontend tests can compare it with the server value.

- [ ] **Step 3: Implement Connection and Pixel pages**

Connection page fields: name, Access Token, enabled; on edit, empty token means preserve. Never display existing token. Pixel page fields: name, Pixel Code, Connection, optional Test Event Code, enabled. Disable changing Code/Connection after create. “发送测试事件” requires Test Event Code and displays accepted vs redacted error; copy explains that test success means API accepted, not ad attribution.

- [ ] **Step 4: Implement event records page**

Filters: status, event name, Pixel, link ID, page. Columns: event name/ID, link, novel, Pixel, event time, status, attempts, HTTP/business code, request ID and redacted error. Status copy is exact: pending “已保存/待发送”, accepted “TikTok 已接收”, failed “发送失败”. Add a fixed note: “TikTok 是否归因：本系统未知，请到 TikTok Ads Manager 查看。” Only retryable rows show retry.

- [ ] **Step 5: Add routes and the isolated TikTok menu**

Routes:

```js
{ path:"tiktok/pixels", name:"tiktok-pixels", component:()=>import("../views/TikTokPixelsView.vue"), meta:{ title:"TikTok Pixel", requiresAuth:true } },
{ path:"tiktok/connections", name:"tiktok-connections", component:()=>import("../views/TikTokConnectionsView.vue"), meta:{ title:"TikTok 凭证", requiresAuth:true } },
{ path:"tiktok/events", name:"tiktok-events", component:()=>import("../views/TikTokEventsView.vue"), meta:{ title:"TikTok 事件记录", requiresAuth:true } },
```

Menu order: TikTok Pixel, TikTok 凭证, TikTok 事件记录. Keep the three frontend project groups separate.

- [ ] **Step 6: Make the novel link form platform-aware**

Show only: name, code, novel, platform, selected platform Pixel, threshold. Lock novel/platform/Pixel fields after first normal visit. Meta label explains `TimeSpent`; TikTok label explains qualified `ViewContent`. Row actions include “复制普通短链”; TikTok rows also include “复制 TikTok 投放模板”. Show platform tag and selected Pixel name in the table.

Run: `cd frontend && npm test && npm run build && npm run format:check`

Expected: PASS.

### Task 9: Extend per-link statistics without claiming TikTok attribution

**Files:**
- Modify: `backend/internal/modules/novel/distribution_stats.go`
- Modify: `backend/internal/app/novel_distribution_test.go`
- Modify: `frontend/src/views/NovelLinkStatsView.vue`
- Modify: `frontend/tests/novel_links.test.js`

**Interfaces:**
- Consumes: Task 1 visit/event fields and existing per-link stats API.
- Produces: reading funnel summary, event delivery summary, platform attribution filters and masked visit detail.

- [ ] **Step 1: Write failing stats tests**

Create two links for one novel and visits with different visitors, start/qualified timestamps and event statuses. Assert no cross-link mixing, UV de-duplication, correct qualified rate, historical null time excluded from average, and exact event-state counts. Add filters for `campaign_id`, `adgroup_id`, `creative_id`, `ad_id_v2` and `event_status`.

Expected summary contract:

```go
type DistributionStatsSummary struct {
	Visits                    int64   `json:"visits"`
	UniqueVisitors            int64   `json:"unique_visitors"`
	CollectedVisits           int64   `json:"collected_visits"`
	AverageVisibleSeconds     float64 `json:"average_visible_seconds"`
	TotalVisibleSeconds       int64   `json:"total_visible_seconds"`
	StartReadingCount         int64   `json:"start_reading_count"`
	StartReadingVisitors      int64   `json:"start_reading_visitors"`
	QualifiedCount            int64   `json:"qualified_count"`
	QualifiedVisitors         int64   `json:"qualified_visitors"`
	QualifiedRate             float64 `json:"qualified_rate"`
	TikTokPendingEvents       int64   `json:"tiktok_pending_events"`
	TikTokAcceptedEvents      int64   `json:"tiktok_accepted_events"`
	TikTokFailedEvents        int64   `json:"tiktok_failed_events"`
}
```

Run: `cd backend && go test ./internal/app ./internal/modules/novel -run 'DistributionStats|TikTokStats' -v`

Expected: FAIL.

- [ ] **Step 2: Extend validated filters and SQL**

All identifiers are strings with max 120 characters. Event status accepts empty, pending, sending, accepted, retry or failed. Reuse one filtered normal-GET CTE for summary and rows. Join event counts through `visit_id` and `link_id` without multiplying visits; use scalar/lateral aggregates per visit or pre-aggregate `tiktok_events` before joining.

- [ ] **Step 3: Return masked visit detail**

Add platform, Pixel name/code, Campaign, Ad Group, Creative, Ad ID v2, placement, start/qualified booleans and qualified server-event status. Mask `ttclid` before JSON:

```go
func maskTikTokID(value string) string {
	if len(value) <= 8 {
		return "********"
	}
	return value[:4] + "…" + value[len(value)-4:]
}
```

Never return `_ttp`, private context cipher, IP, UA or payload.

- [ ] **Step 4: Extend the stats page**

Keep the four existing cards, then add start readers, qualified readers, qualified rate and TikTok delivery states only for TikTok links. Show platform-specific filters/columns. Use exact copy “TikTok 已接收” and the Ads Manager attribution disclaimer. Preserve desktop/mobile responsive layouts.

- [ ] **Step 5: Run stats and frontend regression tests**

Run: `cd backend && go test ./internal/modules/novel ./internal/app -run 'Distribution|TikTok' -v`

Run: `cd frontend && npm test && npm run build && npm run format:check`

Expected: PASS.

### Task 10: Wire lifecycle, run the full regression suite and prepare controlled rollout

**Files:**
- Modify: `backend/internal/bootstrap/app.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/app/app_test.go`
- Modify: `backend/internal/app/routes_test.go`
- Modify: `README.md`
- Modify: `.env.example`

**Interfaces:**
- Consumes: completed TikTok Service/Handler and all UI/H5 behavior.
- Produces: application lifecycle, documented safe configuration and evidence that existing products still pass.

- [ ] **Step 1: Add failing bootstrap and route tests**

Assert `App` owns `TikTok *tiktok.Service`, `Close` is safe before/after Start, Start is idempotent, TikTok routes require admin auth, public TikTok writes require signed ticket, and Meta routes remain registered.

- [ ] **Step 2: Wire Service without changing Meta ownership**

```go
tiktokService := tiktok.New(core)
novelHandler := &novel.Handler{
	Core: core, Meta: metaService, TikTok: tiktokService, Translations: novelTranslations,
}
trackingHandler := &tracking.Handler{
	Core: core, Landing: landingHandler, AudioNovelPage: audioNovelHandler,
	NovelPage: novelHandler, TikTok: tiktokService,
}
```

Add `TikTok *tiktok.Service` to `App` and transport handlers. In `main`, call `a.TikTok.Start(ctx)` next to `a.Meta.Start(ctx)`. In `Close`, close translation workers, TikTok, then Meta; each Close remains idempotent.

- [ ] **Step 3: Document deployment configuration and safe enablement**

Add exact environment variables:

```dotenv
# TikTok stays off until one test Pixel completes Events Manager validation.
TIKTOK_ENABLED=false
TIKTOK_EVENTS_URL=https://business-api.tiktok.com/open_api/v1.3/event/track/
```

Document this rollout order: migrate; deploy backend/admin/H5 with switch false; configure Connection/Pixel/Test Event Code; enable in one instance; validate Pixel Helper + Test Events + Payload Helper; clear Test Event Code; enable all instances; create a new production TikTok link; run low traffic. Do not alter existing Meta links.

- [ ] **Step 4: Run backend formatting, static analysis and all tests**

Run:

```bash
cd backend
gofmt -w internal/modules/tiktok internal/modules/tracking internal/modules/novel internal/bootstrap internal/transport/http cmd/server internal/config
go vet ./...
go test ./...
```

Expected: `go vet` exits 0 and all unit tests PASS; database tests PASS when `TEST_DATABASE_URL` is configured, otherwise their established skip message is the only skip.

- [ ] **Step 5: Run both Vue applications' full verification**

Run:

```bash
cd frontend
npm test
npm run build
npm run format:check
cd ../novel-h5
npm test
npm run build
npm run format:check
```

Expected: all commands exit 0.

- [ ] **Step 6: Inspect the final diff and test the two public platforms manually**

Run `git diff --check` and `git status --short`; do not commit. Start the local server with TikTok enabled and verify:

1. Existing Meta novel link loads only `fbevents.js`, records existing Meta PageView/TimeSpent, and reads normally.
2. TikTok link loads only TikTok Pixel, opens the bound novel directly, sends one `StartReading`, waits for the configured foreground-visible threshold, then sends one server-confirmed `ViewContent`.
3. Backgrounding pauses visible time; language and chapter navigation do not create a new entrance visit.
4. Disabling TikTok Pixel/Connection does not block reading and pauses Worker delivery.
5. Event page says “TikTok 已接收” after API success and never claims final ad attribution.

Expected: all five checks pass, and `git status` contains only the reviewed implementation/docs changes with no generated secrets, local database files or build artifacts.

## Self-Review Record

- Spec coverage: all goals, non-goals, schema, event contracts, dynamic attribution, H5 behavior, APIs, Worker rules, admin UX, statistics, degradation, migration and rollout map to Tasks 1–10.
- Placeholder scan: plan contains no deferred implementation markers; each produced type/function/endpoint is named before consumers use it.
- Type consistency: `ad_platform`, `tiktok_pixel_id`, `BrowserEvent{name,event_id}`, deterministic event IDs and the five delivery statuses are consistent across database, Go JSON and Vue code.
- Review Focus coverage: unresolved macros, lost responses, disabled configuration, fencing races and historical Meta compatibility each have an explicit owning-task test.
- Project constraints: implementation is additive, code comments are required, no Provider framework is introduced, and no Git commit/push step is present.
