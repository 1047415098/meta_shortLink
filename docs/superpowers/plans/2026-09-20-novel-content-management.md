# 小说内容后台管理实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 为现有 LinkScope 增加全局小说内容的数据库管理、封面上传、Markdown 编辑和公开动态读取能力。

**架构：** 小说存入 PostgreSQL，由现有 Gin 进程同时提供鉴权管理 API、公开只读 API、封面文件和三个 Vue 应用。公开路由继续使用短链接 code 解析 WhatsApp、Meta 与统计配置，但小说内容不与 code 绑定；Markdown 只在服务端通过统一白名单渲染器转换为安全 HTML。

**技术栈：** Go 1.27.1、Gin 1.12、pgx/PostgreSQL、goldmark、Vue 3、JavaScript、Vue Router、Element Plus、Vite、Node Test Runner。

**规格：** `docs/superpowers/specs/2026-09-20-novel-content-management-design.md`

## 全局约束

- 小说内容全局共享，不绑定短链接 code。
- 小说模型不包含作者字段，公开页面不得显示作者。
- 正文使用 Markdown，禁止原始 HTML、Markdown 图片和危险链接协议。
- 封面由后台上传，只允许 JPEG、PNG、WebP，单文件最大 5MB。
- 删除采用软删除；第一版不提供恢复界面，不立即删除历史封面。
- 同时最多只有一篇有效首页推荐；没有推荐时回退至最新启用文章。
- 保留现有 `/:code`、`/novel/:code/*`、WhatsApp、Meta 和 surface 统计行为。
- 保留全部现有未提交改动，不创建本地 Git 提交，不推送 GitHub。
- 非直观的事务、上传和 Markdown 安全逻辑添加简洁中文注释，避免过度封装。

## 重点复核项

- 伪造文件扩展名、SVG、HTML、超限文件和路径片段必须被拒绝，临时文件必须清理。
- 两个并发推荐请求最终只能保留一篇推荐文章。
- 软删除、停用或修改 slug 后，旧公开详情不得继续返回内容。
- 公开内容 API 只验证 code，不得额外写入 `click_events`。
- Markdown 中的原始 HTML、`javascript:`、事件属性和图片语法不得进入公开 HTML。

---

### 任务 1：建立小说数据模型、Markdown 安全渲染和仓储契约

**文件：**
- 新建：`backend/internal/platform/database/migrations/013_novels.sql`
- 新建：`backend/internal/modules/novel/model.go`
- 新建：`backend/internal/modules/novel/markdown.go`
- 新建：`backend/internal/modules/novel/markdown_test.go`
- 新建：`backend/internal/modules/novel/repository.go`
- 新建：`backend/internal/modules/novel/repository_test.go`
- 修改：`backend/go.mod`、`backend/go.sum`

**接口：**
- `Novel`：数据库完整模型，不包含作者。
- `NovelInput`：创建与编辑输入。
- `ListFilter`：`Query`、`Status`、`Page`、`PageSize`。
- `ValidateNovelInput(input NovelInput) error`。
- `RenderMarkdown(source string) string`。
- `Repository.ListAdmin`、`ByID`、`Create`、`Update`、`SetEnabled`、`SetFeatured`、`SoftDelete`。
- `Repository.PublicHome`、`PublicList`、`PublicBySlug`、`Related`。

- [ ] **步骤 1：先写模型与 Markdown 失败测试**

测试以下行为：不接受空标题、非法 slug、空分类、超长摘要、空正文、非法日期和非 `/novel-uploads/` 封面；Markdown 段落、标题、列表、引用和分隔线正常输出；原始 HTML、图片、`javascript:` 和事件属性不出现在输出中。

```go
func TestRenderMarkdownRemovesUnsafeContent(t *testing.T) {
    got := RenderMarkdown(`<script>alert(1)</script>
[bad](javascript:alert(2)) ![image](https://evil.test/a.png)`)
    for _, unsafe := range []string{"<script", "javascript:", "<img"} {
        if strings.Contains(strings.ToLower(got), unsafe) {
            t.Fatalf("unsafe output %q", got)
        }
    }
}
```

- [ ] **步骤 2：运行 `go test ./internal/modules/novel -run 'Markdown|ValidateNovel' -count=1`，确认因接口缺失失败。**

- [ ] **步骤 3：实现迁移、模型和安全渲染器**

迁移创建 `novels` 表、活动 slug 部分唯一索引和有效推荐部分唯一索引，并写入 8 篇无作者字段的英文演示内容。引入 `github.com/yuin/goldmark`，禁用原始 HTML与 Markdown 图片扩展，并对生成结果执行标签、属性和链接协议白名单清理。

- [ ] **步骤 4：先写仓储集成测试并确认失败**

使用专用 PostgreSQL 测试库覆盖创建、编辑、关键词筛选、分页、启停、推荐唯一性、推荐回退、软删除、slug 重用和相关推荐。

- [ ] **步骤 5：实现仓储事务**

`SetFeatured` 在事务中锁定有效推荐记录，清除其他推荐后更新目标；`SetEnabled(false)` 和 `SoftDelete` 同步清除目标推荐状态。所有管理写操作增加 `audit_logs` 记录，正文不写入审计，只保存 ID、slug 和变更动作。

- [ ] **步骤 6：运行验证**

运行：

```bash
(cd backend && go test ./internal/modules/novel -count=1)
(cd backend && go test ./internal/app -run 'NovelContent|NovelRepository' -count=1)
git diff --check
```

未配置 `TEST_DATABASE_URL` 时明确记录集成测试跳过，不能把跳过报告为通过。

### 任务 2：实现管理员 CRUD、封面上传和公开内容 API

**文件：**
- 新建：`backend/internal/modules/novel/admin.go`
- 新建：`backend/internal/modules/novel/public.go`
- 新建：`backend/internal/modules/novel/upload.go`
- 新建：`backend/internal/modules/novel/upload_test.go`
- 新建：`backend/internal/app/novel_content_test.go`
- 修改：`backend/internal/config/config.go`
- 修改：`backend/internal/bootstrap/app.go`
- 修改：`backend/internal/transport/http/router.go`
- 修改：`backend/internal/app/app_test.go`

**接口：**
- 管理 API：`GET|POST /api/v1/novels`、`GET|PATCH|DELETE /api/v1/novels/:id`。
- 状态 API：`PATCH /api/v1/novels/:id/status`、`PATCH /api/v1/novels/:id/featured`。
- 预览 API：`POST /api/v1/novels/preview`，输入 Markdown，返回安全 `body_html`。
- 上传 API：`POST /api/v1/novel-covers`，返回形如 `{ "path": "/novel-uploads/8a2dc614.webp" }` 的随机安全路径，扩展名由真实 MIME 决定。
- 公开 API：`GET /novel-api/:code/home|stories|stories/:slug`。
- `Config.NovelUploadDir` 从 `NOVEL_UPLOAD_DIR` 读取，默认 `../data/novel-uploads`。

- [ ] **步骤 1：先写管理员 API 集成测试并确认失败**

覆盖未登录 401、缺少请求校验头 403、创建 200、重复 slug 409、读取详情、编辑、分页筛选、启停、推荐、软删除 200/404，以及响应中不存在作者字段。

- [ ] **步骤 2：实现 JSON 解码、校验和管理员处理器**

创建和编辑使用严格 JSON 解码并拒绝未知字段。单独状态接口只接受对应布尔字段。所有处理器使用 5 秒数据库超时，并把唯一冲突映射为 409。

- [ ] **步骤 3：先写上传测试并确认失败**

使用真实最小 JPEG、PNG、WebP fixture，覆盖成功上传、伪造扩展名、SVG、超过 5MB、缺少文件、嵌入路径名和不可写目录。测试成功文件名只含随机 token 和服务端确定的扩展名。

- [ ] **步骤 4：实现封面上传**

使用 `http.DetectContentType` 检测真实类型，`io.LimitReader` 限制读取，`os.CreateTemp` 写入同一目录，`Sync`、关闭后使用 `os.Rename` 原子完成。所有错误路径通过 `defer` 清理临时文件。

- [ ] **步骤 5：先写公开 API 集成测试并确认失败**

覆盖 code 不存在 404、停用 code 410、home 推荐与回退、列表每页 6 篇、详情与相关推荐、停用/删除文章 404，以及请求前后 `click_events` 数量不变。

- [ ] **步骤 6：实现公开处理器和路由优先级**

公开处理器先使用现有 `links.Repository.ByCode` 验证 code，只读取小说仓储。路由注册在通用 `/:code` 之前；`/novel-uploads/*filepath` 使用现有静态文件处理器，禁止目录遍历并设置 `nosniff`。

- [ ] **步骤 7：运行验证**

```bash
(cd backend && go test ./internal/modules/novel ./internal/app -run 'Novel|Upload|Markdown' -count=1)
(cd backend && go vet ./...)
git diff --check
```

### 任务 3：实现管理后台小说列表、编辑器和封面上传

**文件：**
- 新建：`frontend/src/api/novels.js`
- 新建：`frontend/src/views/NovelListView.vue`
- 新建：`frontend/src/views/NovelFormView.vue`
- 新建：`frontend/tests/novels.test.js`
- 修改：`frontend/src/router/index.js`
- 修改：`frontend/src/layouts/AdminLayout.vue`

**接口：**
- 路由名：`novels`、`novel-create`、`novel-edit`。
- API 函数：`listNovels`、`getNovel`、`createNovel`、`updateNovel`、`setNovelEnabled`、`setNovelFeatured`、`deleteNovel`、`uploadNovelCover`、`previewNovelMarkdown`。

- [ ] **步骤 1：先写 API 合约与表单纯函数测试**

断言分页和筛选参数编码正确，`FormData` 上传不手工设置 Content-Type，创建/编辑请求不包含作者，slug 规范化只保留小写字母、数字和单横线，删除使用正确 DELETE 路径。

- [ ] **步骤 2：运行 `npm test -- tests/novels.test.js`，确认因模块和函数缺失失败。**

- [ ] **步骤 3：实现 API、路由和导航**

新增 `/admin/novels`、`/admin/novels/new`、`/admin/novels/:id/edit`。详情路由通过 `meta.activeMenu="novels"` 保持侧栏选中；导航使用 Element Plus `Reading` 图标。

- [ ] **步骤 4：实现列表页**

列表支持关键词、启用状态、分页；显示封面、标题、slug、分类、发布日期、启用、推荐和更新时间。启停、推荐和删除分别调用独立 API；删除使用 `ElMessageBox.confirm` 二次确认，删除当前页最后一条时回退上一页。

- [ ] **步骤 5：实现新增/编辑页**

表单字段只有标题、slug、分类、摘要、Markdown 正文、封面、发布日期、启用和推荐。上传组件限制单文件并展示进度/错误；Markdown 输入变化后以 300ms 防抖调用预览 API，预览区只渲染后端返回的安全 HTML。

- [ ] **步骤 6：运行后台验证**

```bash
(cd frontend && npm test)
(cd frontend && npm run build)
(cd frontend && npm run format:check)
git diff --check
```

### 任务 4：让公开小说站使用动态内容 API

**文件：**
- 新建：`novel/src/lib/api.js`
- 新建：`novel/tests/api.test.js`
- 修改：`novel/src/views/HomeView.vue`
- 修改：`novel/src/views/StoryListView.vue`
- 修改：`novel/src/views/StoryDetailView.vue`
- 修改：`novel/src/styles.css`
- 修改：`novel/vite.config.js`
- 删除：`novel/src/content/stories.js`
- 删除：`novel/src/lib/content.js`
- 修改：`novel/tests/content.test.js`

**接口：**
- `fetchNovelHome(code, request)`。
- `fetchNovelList(code, page, pageSize, request)`。
- `fetchNovelStory(code, slug, request)`。
- 后端响应字段使用 snake_case，组件内保持相同命名，避免重复映射层。

- [ ] **步骤 1：先写 API 和动态页面状态测试**

测试 URL 对 code、slug 和分页参数正确编码；非 2xx 响应携带可读错误；响应缺少预期对象时拒绝。删除原静态内容测试，改为测试空列表和安全 `body_html` 合约。

- [ ] **步骤 2：运行 `npm test`，确认动态 API 尚未实现导致失败。**

- [ ] **步骤 3：实现公开 API 客户端与开发代理**

Vite 将 `/novel-api/*` 代理到 8080。API 客户端不缓存失败响应，统一返回 JSON；公开 GET 不携带管理后台请求头。

- [ ] **步骤 4：改造首页、列表和详情**

每个页面监听 code、page 或 slug 变化并请求对应 API，使用局部 loading、error、retry 状态。详情使用后端安全 `body_html`；保留字号、沉浸模式、Meta、WhatsApp、相关内容和所有 code 导航。

- [ ] **步骤 5：移除作者和静态正式数据源**

删除所有 `by author`、`authorBio` 和构建期文章导入。加载为空时展示明确空状态；封面为空或加载失败时使用现有原创背景渐变。

- [ ] **步骤 6：运行小说前端验证**

```bash
(cd novel && npm test)
(cd novel && npm run build)
git diff --check
```

### 任务 5：接入持久上传目录、文档和全链路验收

**文件：**
- 修改：`compose.yaml`
- 修改：`deploy/compose.production.yaml`
- 修改：`Dockerfile`
- 修改：`deploy/Dockerfile.runtime`
- 修改：`.env.example`
- 修改：`README.md`
- 修改：`docs/verification.md`
- 修改：`backend/internal/app/routes_test.go`（若现有路由测试位于其他 app 测试文件，则在对应文件增加断言）

**接口：**
- 容器环境：`NOVEL_UPLOAD_DIR=/app/data/novel-uploads`。
- Compose 命名卷：`novel_uploads:/app/data/novel-uploads`。

- [ ] **步骤 1：先写生产路由和静态文件回归测试**

验证 `/novel-uploads/<file>` 返回正确类型、缓存头和 `nosniff`；路径遍历返回 404；原 `/novel-assets/*`、`/:code` 和 `/admin/*` 不受影响。

- [ ] **步骤 2：更新容器与环境配置**

两个 Compose 配置增加持久卷和 `NOVEL_UPLOAD_DIR`。镜像创建 `/app/data/novel-uploads` 并把目录所有权交给非 root `app` 用户；运行时镜像保持同样目录结构。

- [ ] **步骤 3：更新项目文档**

README 增加小说管理入口、字段、API、上传目录、备份要求和公开动态内容说明；`.env.example` 说明本地上传目录；`docs/verification.md` 记录实际测试、跳过项和浏览器结果。

- [ ] **步骤 4：执行完整自动验证**

```bash
(cd frontend && npm test && npm run build && npm run format:check)
(cd landing && npm test && npm run build)
(cd novel && npm test && npm run build)
(cd backend && go test -count=1 ./... && go vet ./...)
git diff --check
```

有专用数据库时额外运行：

```bash
cd backend
TEST_DATABASE_URL="$TEST_DATABASE_URL" META_TEST_DATABASE_URL="$META_TEST_DATABASE_URL" go test -race -count=1 ./...
```

- [ ] **步骤 5：执行浏览器全链路验收**

验证“后台登录 → 小说列表 → 上传封面 → 新增 Markdown 小说 → 设为推荐 → 公开首页 → 列表 → 详情 → 编辑 → 停用 → 公开 404 → 删除”。检查公开 API 请求没有新增访问事件，并在 360、768、1440 宽度核对后台表单和公开页面。

- [ ] **步骤 6：最终差异检查，不提交**

检查 `git status --short`，确认未覆盖任务开始前的用户改动，未跟踪 `node_modules`、`dist`、数据库、上传测试文件或密钥。列出所有修改、验证结果和 Docker/数据库环境限制，不创建 Git commit。
