# 免费小说 H5 独立项目设计

## 目标

在现有 LinkScope 仓库中新增一个与 `frontend/`、`landing/`、`audio-novel/` 并列的 `novel-h5/` Vue 3 + JavaScript 项目，并为它新增独立的小说后端模块和运营后台管理能力。

新站参考用户提供的 thoc.mespery.com 截图，实现移动端优先的首页、搜索、小说列表、作品阅读和目录交互。它是纯免费阅读站，不包含登录、支付、订阅或章节锁定。现有短链接、产品落地页和语音小说业务保持不变。

## 已确认业务边界

- 新小说内容与现有 `audio_novels` 完全独立，不共享作品、封面或正文。
- 运营后台新增“小说管理”，不改名或替代“语音小说管理”。
- 所有已上架章节均可免费阅读，目录不显示锁定状态。
- 不增加用户账号、在线支付、订阅、金币或 WhatsApp 解锁流程。
- 新站复用短码有效性、入口访问记录、Meta PageView 和停留时间统计，但使用独立的 `novel` surface。
- 代码实现保持简洁；只在非直观的业务、事务、安全和统计逻辑旁添加中文注释。
- 不创建或推送 Git 提交。

## 项目和路由边界

仓库包含四个独立前端构建：

- `frontend/`：现有 Vue 3 运营后台。
- `landing/`：现有短链接产品落地页。
- `audio-novel/`：现有语音小说站，保持原状。
- `novel-h5/`：新增免费小说 H5。

新站页面路由：

- `GET|HEAD /novel/:code`：首页。
- `GET|HEAD /novel/:code/search`：搜索。
- `GET|HEAD /novel/:code/stories`：全部小说列表。
- `GET|HEAD /novel/:code/stories/:slug`：作品信息与阅读页面。
- `GET|HEAD /novel-assets/*filepath`：带构建版本号的 H5 资源。
- `GET|HEAD /novel-uploads/*filepath`：后台上传的小说封面。

`/novel/*` 路由必须在通用 `/:code` 路由之前注册。之前的语音小说重命名已明确移除旧 `/novel/*`；本次重新启用该前缀时，它代表全新的免费小说模块，而不是语音小说兼容地址。

Vue Router 从地址中读取 `code`，站内导航始终保留该值。首次打开或刷新任意 H5 页面时由 Gin 记录一次入口访问；SPA 内部切换路由不重复创建访问记录。

## 页面设计

### 首页

首页结构参考用户提供的移动端长截图：

1. 固定顶部栏：品牌标识和搜索入口。
2. 推荐轮播：竖版封面、左右相邻卡片露出、标题和 `Start Reading` 按钮。
3. Ranking：紧凑横向榜单，展示名次、缩略图、标题和作者。
4. 双列作品流：封面、最多三行标题和最多两行摘要。
5. 固定底部导航：首页为有效入口；书架和个人中心使用清晰的未开放状态，不伪造账号能力。

首页由后台数据驱动，不把演示作品写死在前端。推荐为空时回退到最新上架作品；没有任何上架作品时显示明确空状态。

### 搜索

搜索页包含返回按钮、自动聚焦的搜索框、清空按钮、加载状态、结果数量、空结果和错误重试。搜索在提交或短暂停顿后请求公开列表接口，匹配标题、作者和简介。搜索结果复用首页作品卡片，不维护第二套卡片样式。

### 小说列表

列表页展示全部上架作品，按后台排序、发布时间和 ID 稳定排序。移动端使用双列卡片，桌面端只增加容器宽度和列数，不把移动布局无限拉宽。分页使用“加载更多”，防止长列表一次返回全部内容。

### 阅读页

阅读页将作品信息和正文放在同一条路径中：

- 顶部提供返回和目录按钮。
- 首屏展示模糊封面背景、原始封面、标题、作者、简介和浏览量。
- `Start Reading` 滚动至第一章或本地记录的阅读位置。
- 正文区切换为稳定纯色背景，正文最大宽度为 760px。
- 当前章节结束显示 `Continue Reading`，最后一章显示完成状态和相关推荐。
- 目录弹层展示章节序号、章节标题和当前阅读状态；所有上架章节均可打开。

本地存储只记录最近作品、当前章节和滚动位置，不保存身份信息。数据格式损坏或章节被下架时自动忽略旧进度并回到第一章。

### 响应式与可访问性

- 以 360–430px 手机宽度为视觉基准，同时检查 768px 和 1440px。
- 桌面端内容使用居中最大宽度，不放大成整屏正文。
- 按钮和图标提供文本标签或可访问名称，点击区域不小于 44px。
- 键盘可操作搜索、轮播、目录和章节导航，焦点样式清晰。
- 摘要、次级图标和遮罩文字满足可读对比度；尊重减少动画偏好。
- 图片使用明确尺寸、懒加载和后端上传的真实封面；无封面时显示统一的中性占位资源。

## 数据模型

### `novels`

- `id bigint generated always as identity primary key`
- `title text not null`
- `slug text not null`
- `author text not null default ''`
- `category text not null default ''`
- `excerpt text not null`
- `cover_path text not null default ''`
- `published_at date not null`
- `enabled boolean not null default true`
- `featured boolean not null default false`
- `sort_order integer not null default 0`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`
- `deleted_at timestamptz null`

活动记录的 slug 唯一。数据库约束保证最多只有一部有效推荐作品。公开排序采用 `sort_order DESC, published_at DESC, id DESC`。

### `novel_chapters`

- `id bigint generated always as identity primary key`
- `novel_id bigint not null references novels(id)`
- `chapter_number integer not null`
- `title text not null default ''`
- `body_markdown text not null`
- `enabled boolean not null default true`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`
- `deleted_at timestamptz null`

同一作品内活动章节的 `chapter_number` 唯一且必须大于零。删除小说采用软删除，并使其所有章节不再公开；章节也采用软删除，避免后台误操作立即丢失正文。

## 公开接口

- `GET /novel-api/:code/home`：推荐轮播、Ranking 和首页作品流。
- `GET /novel-api/:code/stories?q=&page=&page_size=`：搜索和分页列表。
- `GET /novel-api/:code/stories/:slug`：作品公开信息与章节目录，不返回全部正文。
- `GET /novel-api/:code/stories/:slug/chapters/:number`：单章安全渲染后的正文与相邻章节信息。

公开接口先验证短码存在且启用，但不新增入口访问事件。接口只返回已上架、未删除的作品和章节。作品或章节不存在、已停用或已删除时统一返回 404，不泄露后台内容。

正文按章节读取，避免长篇小说一次返回全部内容。Markdown 保存原文，服务端使用与语音小说一致的白名单规则渲染安全 HTML；禁止原始 HTML、脚本、iframe、图片 Markdown 和危险协议。

## 管理接口和运营后台

运营后台新增 `/admin/novels` 列表、`/admin/novels/new` 新增页和 `/admin/novels/:id/edit` 编辑页。

管理接口：

- `GET|POST /api/v1/novels`
- `GET|PATCH|DELETE /api/v1/novels/:id`
- `PATCH /api/v1/novels/:id/status`
- `PATCH /api/v1/novels/:id/featured`
- `GET|POST /api/v1/novels/:id/chapters`
- `GET|PATCH|DELETE /api/v1/novels/:id/chapters/:chapterId`
- `POST /api/v1/novels/preview`
- `POST /api/v1/novel-covers`

小说列表支持关键词、状态和分页，展示封面、标题、作者、章节数、推荐、排序、发布日期和更新时间。新增或编辑作品时管理基础信息；保存作品后在同页管理章节。章节支持新增、编辑、启停、软删除和按章节号排序。

封面上传复用现有安全规则：只接受 JPEG、PNG、WebP，最大 5MB，按文件头检测 MIME，随机生成文件名，使用原子重命名并拒绝路径片段。封面独立保存到 `NOVEL_UPLOAD_DIR`，不与 `AUDIO_NOVEL_UPLOAD_DIR` 混用。

## 统计与后端装配

`click_events.surface` 新增 `novel` 合法值，现有 `short_link` 和 `audio_novel` 保持不变。统计总览、访问明细和筛选增加免费小说站维度，但保留现有响应字段，避免破坏调用方。

后端新增独立 `novel` 模块，复用现有短码查询、请求分类、Meta PageView、入口渲染和停留时间基础能力，不复制一套追踪算法。启动数据使用独立标记，只包含公开短码信息、统计开关、事件 ID、公开 Pixel ID 和可选页面错误，不暴露密钥或内部凭证。

配置新增：

- `NOVEL_DIR`：本地默认 `../novel-h5/dist`。
- `NOVEL_UPLOAD_DIR`：本地默认 `../data/novel-uploads`。

Docker 增加 `novel-h5` 构建阶段，将产物复制到 `/app/novel-h5`，并挂载独立小说封面持久卷。现有三个前端的构建和目录保持不变。

## 错误处理

- 短码不存在：小说样式 404 页面和 HTTP 404。
- 短码停用：小说样式不可用页面和 HTTP 410。
- 数据库读取失败：HTTP 503，不伪造作品或章节。
- 入口记录失败：仍展示免费小说站，并记录服务错误；不显示虚假的统计成功状态。
- 公开接口网络失败：页面保留上下文并提供重试。
- 搜索无结果、首页无内容和作品无章节分别使用明确空状态。
- 后台重复 slug 或章节号返回 409；字段错误返回 400；不存在返回 404。
- 同时设置推荐作品时由事务和唯一索引保证最终最多一部有效推荐。

## 测试与验证

所有业务实现遵循测试先行。

后端测试覆盖：

- 数据迁移、约束、作品和章节 CRUD、软删除、启停、推荐唯一性与排序。
- 公开首页、搜索、分页、详情、目录和单章接口。
- 短码验证、仅公开已上架内容，以及 API 不重复创建访问记录。
- `/novel/*` 与 `/:code` 路由优先级。
- `novel` surface 的入口、统计、筛选和历史业务兼容。
- Markdown 清洗、封面 MIME、大小、路径和写入失败。

运营后台测试覆盖：

- 请求字段和接口路径。
- 小说表单、章节增删改查、排序、状态和冲突提示。
- 封面上传、Markdown 预览和删除确认。

`novel-h5` 测试覆盖：

- 路由始终保留 code。
- 首页数据映射、搜索参数、分页和错误状态。
- 目录打开关闭、章节切换、继续阅读与最后一章状态。
- 本地阅读进度的保存、恢复和损坏数据降级。
- Markdown 输出不执行危险内容。

最终运行运营后台、落地页、语音小说和新小说 H5 的测试与生产构建，再运行 `go test ./...`、`go vet ./...` 和 Docker 构建。浏览器视觉验收对照用户提供的四张截图，覆盖首页、搜索、作品首屏、长正文和目录弹层，并检查 360px、768px、1440px 三档视口。

## 完成标准

- 四个前端均可独立构建，旧业务无回归。
- 运营后台可以完整管理小说和章节。
- 新 H5 的首页、搜索、列表、阅读和目录可真实调用新增接口。
- 全部上架章节免费可读，无登录、支付或锁定入口。
- 页面主要布局和交互与参考截图一致，且桌面端阅读宽度合理。
- 自动化测试、生产构建、Go 测试和静态检查通过。
- 不创建 Git 提交。
