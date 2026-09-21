# 小说内容后台管理设计方案

## 目标

在现有 LinkScope 管理后台和 Gin 服务中增加小说内容管理能力，使管理员可以新增、编辑、查询、启停、设为首页推荐和删除小说。公开小说站改为从 PostgreSQL 动态读取内容，后台修改后立即生效，不需要重新构建 Vue 应用。

小说内容继续全局共享，不绑定某个短链接 code。`/novel/:code/*` 中的 code 仍只负责解析现有短链接记录、WhatsApp 目标、Meta 配置和访问统计归属。

## 功能范围

### 包含内容

- PostgreSQL 小说内容表及现有演示内容初始化。
- Gin 管理员小说 CRUD、启停、首页推荐和封面上传 API。
- Gin 公开小说列表、详情和首页推荐 API。
- 管理后台“小说管理”菜单、列表、搜索、新增、编辑、状态操作和删除确认。
- Markdown 正文编辑与实时预览。
- 公开小说首页、列表和详情页改用动态 API 数据。
- 公开页面的加载、空列表、文章不存在和接口错误状态。
- 封面文件的类型、大小、路径和响应安全控制。

### 暂不包含

- 作者字段。
- 小说与短链接 code 的内容绑定。
- 多语言内容和多版本正文。
- 回收站或后台恢复按钮。
- 定时发布、审核工作流、修订历史和多人权限。
- 富文本 HTML 编辑器。
- 删除小说时立即删除其历史封面文件。

## 数据模型

新增 `novels` 表：

- `id bigint generated always as identity primary key`
- `title text not null`
- `slug text not null`
- `category text not null`
- `excerpt text not null`
- `body_markdown text not null`
- `cover_path text not null default ''`
- `published_at date not null`
- `enabled boolean not null default true`
- `featured boolean not null default false`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`
- `deleted_at timestamptz null`

slug 只允许小写字母、数字和单横线，长度为 3–120。使用仅针对 `deleted_at IS NULL` 的唯一索引，软删除后允许未来重新使用相同 slug。

使用仅针对 `featured=true AND enabled=true AND deleted_at IS NULL` 的唯一索引，保证数据库层面同时最多只有一篇有效首页推荐。设为推荐时在事务中先取消其他文章推荐，再更新目标文章。

首次迁移写入当前 8 篇原创英文演示文章。迁移不保存作者信息，正文转换为 Markdown 段落和 `---` 分隔线。迁移使用固定 slug 并保证重复执行安全。

## 管理员 API

全部接口位于现有鉴权组 `/api/v1` 下，并继续要求登录会话和非 GET 请求的 `X-Requested-With: XMLHttpRequest` 校验。

- `GET /api/v1/novels`：分页、关键词和状态筛选。
- `GET /api/v1/novels/:id`：读取单篇完整内容。
- `POST /api/v1/novels`：创建小说。
- `PATCH /api/v1/novels/:id`：编辑小说。
- `PATCH /api/v1/novels/:id/status`：单独启用或停用。
- `PATCH /api/v1/novels/:id/featured`：设为或取消首页推荐。
- `DELETE /api/v1/novels/:id`：软删除。
- `POST /api/v1/novel-covers`：上传封面。

创建和编辑字段为：

```json
{
  "title": "The Glass Orchard",
  "slug": "the-glass-orchard",
  "category": "Mythic Fantasy",
  "excerpt": "At dawn, every tree in Veyra began to remember its dead.",
  "body_markdown": "The first apple rang like a bell.\n\n---\n\nBy noon...",
  "cover_path": "/novel-uploads/8a2d...webp",
  "published_at": "2026-09-17",
  "enabled": true,
  "featured": false
}
```

服务端校验：标题 1–200 字符、分类 1–80 字符、摘要 1–600 字符、Markdown 正文 1–200000 字符、合法日期、合法 slug、封面路径为空或以 `/novel-uploads/` 开头。重复 slug 返回 409；字段错误返回 400；不存在或已删除记录返回 404。

停用或删除推荐文章时同时清除其 `featured`。取消推荐允许全站暂时没有显式推荐，公开首页将回退到最新发布的启用文章。

## 公开 API 与 code 边界

公开内容接口位于 `/novel-api/:code` 下：

- `GET /novel-api/:code/home`：返回有效推荐文章；无显式推荐时返回最新启用文章。
- `GET /novel-api/:code/stories?page=1&page_size=6`：返回启用且未删除的小说摘要列表。
- `GET /novel-api/:code/stories/:slug`：返回一篇启用且未删除的完整小说及最多 3 篇同分类相关推荐。

每个公开 API 请求先验证 code 对应的 `short_links` 记录存在且启用，但不新增 `click_events`。页面 HTML 入口继续负责一次访问记录；SPA 内部加载内容不会重复生成访问。

公开响应不包含管理字段、删除时间或内部文件路径之外的信息。停用或删除的小说详情统一返回 404，避免泄露未发布内容。

## 封面上传

新增 `NOVEL_UPLOAD_DIR` 配置，本地默认 `../data/novel-uploads`，容器内为 `/app/data/novel-uploads`。上传目录通过 `/novel-uploads/*filepath` 提供只读访问，并沿用现有静态资源的路径清理、内容类型和缓存处理。

上传规则：

- 请求必须是 `multipart/form-data`，字段名固定为 `file`。
- 最大正文限制 6MB，单个文件最大 5MB。
- 仅允许 JPEG、PNG 和 WebP。
- Gin 根据文件头检测真实 MIME，不相信文件扩展名或请求头。
- 服务端生成随机文件名，并使用检测到的类型确定扩展名。
- 文件先写入上传目录内的临时文件，完成后原子重命名。
- 文件权限不允许执行；响应添加 `X-Content-Type-Options: nosniff`。
- 不接受 SVG、HTML、压缩包或路径片段。

软删除和更换封面时不立即删除旧文件，避免多个内容引用同一图片或回滚时产生破图。孤立文件清理作为后续独立运维功能，不纳入第一版。

## Markdown 安全渲染

后台编辑器保存原始 Markdown，不保存 HTML。前端只支持以下结构：

- 普通段落和换行。
- 一级至三级标题。
- 加粗、斜体。
- 有序和无序列表。
- 引用。
- `---` 章节分隔线。
- `http` 和 `https` 链接。

禁止原始 HTML、图片 Markdown、脚本、iframe、内联事件和 `javascript:` 等危险协议。公开前端使用受控 Markdown 解析器生成虚拟节点，或使用经过白名单清洗的 HTML；不直接把未经处理的正文传入 `v-html`。

后台实时预览和公开详情必须使用同一套解析规则，避免后台看到的结果与公开页面不一致。

## 管理后台体验

侧边栏新增“小说管理”。列表包含：封面、标题、slug、分类、发布日期、启用状态、首页推荐、更新时间和操作。

列表支持：

- 标题或 slug 关键词搜索。
- 全部、已启用、已停用筛选。
- 分页。
- 独立启停开关。
- 设为首页推荐或取消推荐。
- 编辑。
- 删除并二次确认。

新增和编辑复用一个表单页面。封面上传成功后写入 `cover_path`，表单保存失败时保留已上传图片路径，方便用户重试。Markdown 编辑区在宽屏下左右分栏，在窄屏下上下排列。

删除成功后返回列表并刷新当前页。如果删除导致当前页为空且不是第一页，自动退回上一页。所有操作使用中文提示；小说公开内容仍为英文。

## 公开小说站调整

移除构建期 `stories.js` 作为正式数据源。首页挂载后请求 home API；列表根据 URL 的 `page` 查询参数请求分页 API；详情根据 slug 请求详情 API。

页面保留现有视觉风格、阅读字号、沉浸模式、code 保留、Meta PageView 和手动 WhatsApp 行为。加载内容时显示与现有版式一致的骨架或加载状态；公开 API 失败时显示可重试错误，不影响已经注入的 WhatsApp 降级目标。

封面为空时显示现有原创奇幻背景的渐变裁切，不显示破图。

## 错误处理与一致性

- 数据库读取失败：管理员 API 和公开 API 返回 503。
- 上传目录不可写：返回 503，不创建小说记录。
- 上传中断或重命名失败：清理本次临时文件。
- 两名管理员同时设为推荐：事务和唯一索引保证最终只有一篇推荐。
- 两名管理员同时使用相同 slug：一个成功，另一个返回 409。
- 删除、停用或修改 slug 后，旧公开详情地址返回 404。
- 公开列表页码超出范围时返回空列表和正确总页数，不自动重定向。
- 数据库迁移后没有启用文章时，首页显示空内容状态，而不是虚构演示数据。

## 测试与验收

后端测试覆盖：

- 小说创建、编辑、列表、筛选、分页、启停、推荐和软删除。
- slug 格式、唯一性、字段长度和日期校验。
- 推荐唯一性，以及推荐文章停用/删除后的首页回退。
- 管理员鉴权和请求头校验。
- 公开 API 的 code 验证、仅返回启用内容、分页和相关推荐。
- JPEG、PNG、WebP 上传成功，以及伪造 MIME、超限、SVG、路径攻击和写入失败。
- 公开内容 API 不额外写入访问事件。

管理后台测试覆盖：

- 小说 API 请求参数。
- 表单校验与 Markdown 预览。
- 上传成功、失败和重试状态。
- 删除确认、分页回退、启停和推荐提示。

小说前端测试覆盖：

- 首页、列表、详情从 API 加载。
- 分页和站内导航始终保留 code。
- Markdown 白名单渲染和危险内容过滤。
- 加载、空状态、404 和 503 状态。
- 阅读字号、沉浸模式、Meta 和 WhatsApp 现有行为不回归。

最终验收运行三个前端的测试和构建、`go test ./...`、`go vet ./...` 和 Docker 构建；有专用测试数据库时运行完整数据库集成测试。浏览器检查后台列表与编辑器，以及 360、768、1440 三档公开首页、列表和详情。

## 实施约束

- 保留现有 `/:code` 和 `/novel/:code/*` 路由行为。
- 保留仓库全部已有未提交改动。
- 不创建或推送 GitHub 提交。
- 非直观的事务、上传和 Markdown 安全逻辑添加简洁中文注释。
- 避免过度封装；仓储、处理器和页面按现有项目模式保持清晰直接。
