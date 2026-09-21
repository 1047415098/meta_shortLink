# 小说前端接入现有系统实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 在不改变现有 `/:code` 短链接行为的前提下，新增由同一个 Gin 服务托管的独立 Vue 3 奇幻文学站，并在统计中区分短链接与小说站入口。

**架构：** 仓库新增 `novel/` Vite 应用，文学内容以安全的结构化 JavaScript 数据随构建发布；Gin 继续通过 `short_links.code` 解析 WhatsApp 目标、归因和 Meta 配置，并向 `/novel/:code/*` 注入启动数据。访问记录通过新增 `surface` 字段区分 `short_link` 与 `novel`，现有 `event_type` 和漏斗语义保持不变。

**技术栈：** Vue 3、JavaScript、Vue Router、Vite、Go 1.27.1、Gin 1.12、pgx/PostgreSQL、Node Test Runner、Go `testing`、Docker 多阶段构建。

**规格：** `docs/superpowers/specs/2026-09-20-novel-frontend-integration-design.md`

## 全局约束

- 保留仓库中全部现有未提交改动；修改前后检查差异，禁止覆盖用户已有工作。
- 不创建本地 Git 提交，也不推送 GitHub；每个任务以测试和差异检查作为检查点。
- 现有 `/:code`、`/:code/view`、`/:code/contact`、`/admin/*` 和 `/api/v1/*` 行为必须兼容。
- 小说站只允许手动 WhatsApp 咨询，不启用倒计时或自动跳转。
- 所有 code 共用同一套英文文学内容；第一版不增加小说 CRUD 或管理后台。
- 不复制参考站 Logo、插画或正文；视觉只参考“全屏封面 + 白色阅读纸张”的结构与阅读节奏。
- 非直观逻辑添加简洁中文注释，避免为简单页面逻辑过度封装。

## 重点复核项

- `/novel/:code/*` 必须在 `/:code` 之前匹配，未知小说子路由不能被误当成短码。
- 历史 `click_events` 必须自动得到 `surface='short_link'`，数据库升级不能破坏现有报表。
- 短链接票据与小说票据必须互相不可重放，但各自重复提交仍保持幂等。
- 小说站直接打开详情页时生成一次访问；SPA 内部换页不得重复生成访问。
- 小说访问写入失败时页面仍可阅读和直接咨询，同时不能伪造 PageView 或咨询统计。

---

### 任务 1：建立入口类型、签名和统计底层契约

**文件：**
- 新建：`backend/internal/platform/database/migrations/012_click_surface.sql`
- 修改：`backend/internal/modules/tracking/repository.go`、`handler.go`
- 修改：`backend/internal/modules/landing/handler.go`、`render.go`、`repository.go`
- 修改：`backend/internal/app/app_test.go`、`landing_test.go`

**接口：**
- 新增 `tracking.Event.Surface string`，仅允许 `short_link` 或 `novel`。
- `MarkContact(ctx, eventID, linkID, surface, automatic, input)`。
- `MarkView(ctx, eventID, linkID, surface, input)`。
- 票据签名固定为 `contact:<surface>:<code>:<eventID>`。

- [ ] **步骤 1：先写数据库字段与现有短链行为测试**

访问 `/hello` 后增加断言：

```go
var surface string
if err := a.DB.QueryRow(context.Background(),
    `SELECT surface FROM click_events ORDER BY occurred_at DESC LIMIT 1`,
).Scan(&surface); err != nil || surface != "short_link" {
    t.Fatalf("short-link surface=%q err=%v", surface, err)
}
```

增加测试确认非法 surface 写入失败、历史默认值为 `short_link`、现有落地页票据不能作为 novel 票据使用。

- [ ] **步骤 2：运行定向测试并确认因字段或签名契约缺失而失败**

运行 `TEST_DATABASE_URL="$TEST_DATABASE_URL" go test ./internal/app -run 'TestRedirectAndDedup|TestClickSurface|TestCrossSurfaceTicket' -count=1`。

- [ ] **步骤 3：实现迁移与显式写入**

```sql
ALTER TABLE click_events
ADD COLUMN IF NOT EXISTS surface text NOT NULL DEFAULT 'short_link';
ALTER TABLE click_events DROP CONSTRAINT IF EXISTS click_events_surface_check;
ALTER TABLE click_events ADD CONSTRAINT click_events_surface_check
CHECK (surface IN ('short_link', 'novel'));
CREATE INDEX IF NOT EXISTS clicks_surface_time
ON click_events(surface, occurred_at DESC);
```

在追踪 INSERT 中加入 `surface`。现有 `Redirect` 必须显式传入 `short_link`，不能只依赖默认值。

- [ ] **步骤 4：让现有落地页签名绑定 `short_link`**

签名改为 `eventID + "." + a.Sign("contact:short_link:"+l.Code+":"+eventID)`。仓储查询增加 surface 条件，现有 landing handler 的咨询与 view 始终传入 `short_link`；自动咨询保持不变。

- [ ] **步骤 5：运行回归测试并检查差异**

运行 `go test ./internal/modules/landing ./internal/modules/meta -count=1`，再使用两个测试数据库运行 `go test ./internal/app -run 'Redirect|Landing|Surface|Meta' -count=1`，最后运行 `git diff --check`。现有直接跳转、产品落地、PageView、手动和自动咨询必须全部通过。

### 任务 2：增加小说入口的 Gin 输出与操作路由

**文件：**
- 新建：`backend/internal/modules/novel/handler.go`、`render.go`、`render_test.go`
- 新建：`backend/internal/app/novel_test.go`
- 修改：`backend/internal/config/config.go`、`backend/internal/bootstrap/app.go`
- 修改：`backend/internal/modules/tracking/handler.go`
- 修改：`backend/internal/modules/requestlogs/middleware.go`
- 修改：`backend/internal/transport/http/router.go`
- 修改：`backend/internal/app/app_test.go`、`request_logs_test.go`

**接口：**
- `novel.Handler.Render`、`Unavailable`、`Contact`、`View`。
- `tracking.Handler.Novel` 使用共享追踪流程写入 `surface='novel'`、`event_type='landing'`。
- `Config.NovelDir` 从 `NOVEL_DIR` 读取，默认 `../novel/dist`。

- [ ] **步骤 1：在测试配置中准备小说 HTML，并先写路由测试**

测试 HTML 只包含一个 `<!--NOVEL_BOOTSTRAP-->` 标记。覆盖 `/novel/hello`、`/novel/hello/stories`、`/novel/hello/stories/the-glass-orchard`，并断言响应含 `"surface":"novel"`。同时测试未知 code=404、停用 code=410、HEAD 无正文、访问写入失败时无票据降级，以及小说路由没有进入 `/:code`。请求日志测试确认小说 HTML、view 和 contact 被标记为访客请求，`/novel-assets/*` 不写日志。

- [ ] **步骤 2：运行 `TEST_DATABASE_URL="$TEST_DATABASE_URL" go test ./internal/app -run 'TestNovel' -count=1`，确认路由尚未注册。**

- [ ] **步骤 3：实现安全输出器和最小启动数据**

```go
type Bootstrap struct {
    Link                *links.Link `json:"link"`
    Ticket              string      `json:"ticket"`
    Surface             string      `json:"surface"`
    CookieEnabled       bool        `json:"cookie_enabled"`
    MetaMeasurement     bool        `json:"meta_measurement"`
    MetaBrowserPixelID  string      `json:"meta_browser_pixel_id,omitempty"`
    MetaPageViewEventID string      `json:"meta_pageview_event_id,omitempty"`
    MetaManualEventID   string      `json:"meta_manual_event_id,omitempty"`
    Error               *PageError  `json:"error,omitempty"`
}
```

输出器使用 `encoding/json`、nonce 和独立 CSP，安全替换 title、description 与 Open Graph 标签；不暴露内部 Meta 凭证。

- [ ] **步骤 4：复用追踪流程并注册高优先级路由**

把 `tracking.Redirect` 中链接解析、分类、归因和事件写入收敛到带 surface 的私有方法。`Redirect` 走 `short_link`，`Novel` 走 `novel`。在 `/:code` 之前注册首页、列表、详情、contact、view 及对应 HEAD 路由，并添加中文注释解释路由优先级。

请求日志中间件按 Gin 的完整路由模板识别上述小说入口、view 和 contact，继续沿用现有脱敏与 32KB 限制；静态资源不记录。

- [ ] **步骤 5：实现仅手动的小说咨询**

小说票据使用 `contact:novel:<code>:<eventID>`。`Contact` 只接受空值或 `manual` 并拒绝 `auto`；`View` 使用相同票据。两者都要求一小时内当前链接的 novel 访问。

- [ ] **步骤 6：运行测试并检查差异**

运行 `go test ./internal/modules/novel ./internal/modules/landing ./internal/modules/tracking -count=1`，再运行带测试数据库的 `go test ./internal/app -run 'Novel|Landing' -count=1`，最后运行 `git diff --check`。

### 任务 3：建立独立小说前端、内容模型和页面体验

**文件：**
- 新建：`novel/package.json`、`package-lock.json`、`vite.config.js`、`index.html`
- 新建：`novel/src/main.js`、`App.vue`、`bootstrap.js`、`router/index.js`
- 新建：`novel/src/content/stories.js`、`novel/src/lib/content.js`
- 新建：`novel/src/components/SiteHeader.vue`、`WhatsAppAction.vue`
- 新建：`novel/src/views/HomeView.vue`、`StoryListView.vue`、`StoryDetailView.vue`、`UnavailableView.vue`
- 新建：`novel/src/lib/contact.js`、`reading.js`、`styles.css`
- 新建：`novel/public/novel-assets/images/*`
- 新建：`novel/tests/content.test.js`、`router.test.js`、`contact.test.js`、`reading.test.js`

**接口：**
- `featuredStory`、`findStory(slug)`、`listStories(page, pageSize)`、`relatedStories(story, limit)`。
- 路由名固定为 `home`、`stories`、`story`，每个内部链接保留 code。
- `submitNovelContact({code, ticket, request})` 只提交 `trigger=manual`。

- [ ] **步骤 1：先写内容、路由、阅读和咨询纯函数测试**

```js
assert.equal(listStories(1, 6).items.length, 6);
assert.equal(findStory("missing-story"), null);
assert.ok(relatedStories(stories[0], 3).every((x) => x.slug !== stories[0].slug));
assert.equal(storyPath("hello", "the-glass-orchard"), "/novel/hello/stories/the-glass-orchard");
assert.equal(clampReadingSize(11), 14);
assert.equal(clampReadingSize(30), 24);
```

咨询测试确认请求目标为 `/novel/hello/contact`，正文包含 `trigger=manual`，并且代码中没有定时器或 `auto` 分支。

- [ ] **步骤 2：创建最小 Vue 工程**

依赖只包含 Vue 与 Vue Router，版本与现有项目保持同一主版本。开发端口固定为 5175，`/novel/:code/contact|view` 代理到 8080，构建资源目录固定为 `novel-assets`。`index.html` 保留唯一 `<!--NOVEL_BOOTSTRAP-->` 标记。

- [ ] **步骤 3：实现共享原创文学内容**

准备至少 8 篇原创英文奇幻演示文章，结构固定为：

```js
{
  slug: "the-glass-orchard",
  title: "The Glass Orchard",
  author: "Mara Venn",
  publishedAt: "2026-09-17",
  category: "Mythic Fantasy",
  excerpt: "At dawn, every tree in Veyra began to remember its dead.",
  authorBio: "Mara Venn writes secondary-world fantasy about memory and place.",
  image: "/novel-assets/images/story-glass-orchard.webp",
  body: [
    { type: "paragraph", text: "The first apple rang like a bell when it fell." },
    { type: "break" },
    { type: "paragraph", text: "By noon, the whole orchard was singing." },
  ],
}
```

正文按类型渲染文本，禁止使用 `v-html`。

- [ ] **步骤 4：生成并验收原创视觉素材**

使用图像生成能力制作无文字、无 Logo 的原创横向奇幻山谷主背景和少量文章缩略图。方向为明亮天空、远山瀑布、遗迹、前景旅人和足够标题负空间，但不得复刻参考站具体构图、角色或素材。检查桌面与移动裁切、清晰度和版权独立性后保存为 WebP；Logo 使用纯文字排版。

- [ ] **步骤 5：实现首页、列表和详情**

首页以插画为主体并只突出最新文章；列表和详情使用固定背景上的白色阅读纸张。列表每页 6 篇。详情逐块渲染段落与章节分隔，显示最多 3 篇同分类推荐。

阅读控制常量固定为：

```js
const STORAGE_KEY = "novel-reading-size";
const MIN_SIZE = 14;
const DEFAULT_SIZE = 18;
const MAX_SIZE = 24;
```

沉浸模式只隐藏站点导航并扩大阅读容器，Escape 退出，不使用浏览器原生全屏 API。

- [ ] **步骤 6：实现 code 保留、Meta 与 WhatsApp 行为**

路由守卫验证 URL code 与启动数据 `link.code` 一致。所有导航通过命名路由携带 code。有票据时先写咨询再导航到返回的 `target_url`；无票据时使用启动数据中的目标地址降级。Meta PageView 和手动事件沿用现有事件 ID 去重规则，但小说前端保持源码独立。

- [ ] **步骤 7：运行测试、构建和视觉检查**

运行 `npm test`、`npm run build` 和 `git diff --check`。在 1440px、768px、360px 检查首页、列表、详情、菜单、沉浸模式和错误页，确认无横向滚动、遮挡、破图或参考站版权素材。

### 任务 4：在 API 和管理后台区分两类入口

**文件：**
- 修改：`backend/internal/modules/analytics/model.go`、`service.go`、`repository.go`、`handler.go`、`link_stats.go`
- 修改：`backend/internal/app/app_test.go`、`link_stats_test.go`
- 修改：`frontend/src/components/AnalyticsFilter.vue`
- 修改：`frontend/src/composables/useReport.js`
- 修改：`frontend/src/views/DashboardView.vue`、`VisitListView.vue`
- 修改：`frontend/tests/utils.test.js`

**接口：**
- 可选查询参数：`surface=short_link|novel`。
- 事件 JSON 增加 `surface`。
- 汇总增加 `short_link_views`、`novel_views`、`short_link_whatsapp_clicks`、`novel_whatsapp_clicks`。
- CSV 在 `event_type` 后追加 `surface`。

- [ ] **步骤 1：先写筛选、汇总和导出测试**

创建一条 short_link 和一条 novel 事件，断言两个访问分项各为 1；`surface=novel` 只返回 novel；`surface=unknown` 返回 400；CSV 表头和数据行都包含 surface。单链接统计也必须返回四个分项。

- [ ] **步骤 2：扩展筛选与 SQL 参数**

`Filter` 增加 `Surface string`，只接受空值、`short_link`、`novel`。`eventWhere` 增加参数化 surface 条件后，统一修正 Events、趋势和花费查询的占位符编号，禁止拼接用户输入。`eventCols` 与 Scan 顺序末尾加入 surface。

- [ ] **步骤 3：扩展汇总与单链接统计**

在保留 `landing_views`、`whatsapp_clicks` 等原字段的同时，使用 PostgreSQL `FILTER` 计算四个新分项。前端不得通过总数反推分项。

- [ ] **步骤 4：更新后台筛选和展示**

筛选器增加“全部 / 短链接 / 小说站”；URL 查询同步保存 surface。总览在现有访问和咨询卡片内展示两类入口分项；访问明细增加入口类型列。不要增加大量重复卡片。

- [ ] **步骤 5：运行后端与管理后台验证**

使用测试数据库运行 `go test ./internal/app ./internal/modules/analytics -count=1`，然后在 `frontend/` 运行 `npm test`、`npm run build`，最后运行 `git diff --check`。

### 任务 5：接入构建、静态资源、开发脚本与最终验收

**文件：**
- 修改：`Dockerfile`、`deploy/Dockerfile.runtime`
- 修改：`scripts/dev-local.sh`、`.env.example`、`README.md`
- 修改：`backend/internal/transport/http/router.go`
- 修改：`backend/internal/app/routes_test.go`
- 修改：`docs/verification.md`

**接口：**
- `NOVEL_DIR` 默认 `../novel/dist`，容器值 `/app/novel`。
- `/novel-assets/*` 使用现有压缩协商和哈希资源长期缓存规则。
- 本地小说开发服务器固定端口 5175。

- [ ] **步骤 1：先写静态资源与路由回归测试**

断言小说哈希 JS 使用长期缓存及压缩协商；小说深层路由返回小说 HTML；`/hello` 仍走原短链；未知 `/novel/...` 不落入 `/:code`。

- [ ] **步骤 2：扩展构建与运行镜像**

根 Dockerfile 增加 novel Node 构建阶段并复制到 `/app/novel`。两个运行镜像统一设置：

```dockerfile
ENV LISTEN_ADDR=0.0.0.0:8080 \
    FRONTEND_DIR=/app/admin \
    LANDING_DIR=/app/landing \
    NOVEL_DIR=/app/novel
```

不得改变现有二进制、数据库或 GeoIP 挂载方式。

- [ ] **步骤 3：扩展本地开发与文档**

`dev-local.sh` 同时管理 5173、5174、5175 三个进程。README 更新目录树、路由表、构建命令、统计定义和本地入口 `http://localhost:5175/novel/hello`；`.env.example` 增加可选 `NOVEL_DIR` 说明。

- [ ] **步骤 4：执行全量自动化验证**

依次运行：

```bash
(cd frontend && npm test && npm run build && npm run format:check)
(cd landing && npm test && npm run build)
(cd novel && npm test && npm run build)
(cd backend && go test ./... && go vet ./...)
docker build -t linkscope-novel:test .
```

配置专用测试库时，再运行 `TEST_DATABASE_URL="$TEST_DATABASE_URL" META_TEST_DATABASE_URL="$META_TEST_DATABASE_URL" go test -race ./...`。

- [ ] **步骤 5：执行全链路和视觉验收**

先验证现有直接跳转、产品落地、手动/自动咨询和后台登录均无回归，再使用同一 code 完成“小说首页 → 列表 → 详情 → 推荐 → 手动 WhatsApp”。确认 SPA 换页不重复写访问，刷新只增加一次 novel 访问，咨询只写一次。

对照参考站检查布局、层次、节奏和阅读体验，不比较或复制其素材。把实际命令、结果、跳过原因和 1440px、768px、360px 检查结果写入 `docs/verification.md`。

- [ ] **步骤 6：最终差异检查，不提交**

运行 `git diff --check` 和 `git status --short`。确认未覆盖原有未提交工作，未生成数据库、密钥或不应跟踪的构建文件，并向用户列出最终改动与验证结果。
