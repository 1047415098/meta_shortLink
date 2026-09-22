# 语音小说 Podcast 本地化设计

## 目标

为现有语音小说项目补齐真正的音频能力，使测试站在内容结构和用户行为上对齐生产站 `beneath-ceaseless-skies.com`：一篇文章可以关联一个完整 MP3；正文与 Podcast 使用独立页面但可以互相跳转；音频文件下载到 LinkScope 自己的服务器持久化存储，不依赖生产站外链。

首批数据来自已经导入 `audio_novels` 的 2026 年文章。来源站当前共有 11 个 Podcast，导入工具需要下载这些 MP3 并按文章 slug 关联。没有 Podcast 的文章继续正常展示正文，不伪造音频入口。

本次只修改语音小说模块。免费小说、普通短链接及其内容接口不共享音频数据，也不新增音频能力。

## 已确认的生产站行为

- 每篇有音频的文章对应一个完整 MP3，不按章节拆分。
- 正文详情和 Podcast 详情是两个页面。
- 有音频的正文详情显示 `Podcast` 入口；无音频时不显示。
- Podcast 详情显示标题、摘要、播放器、时长、文件大小、下载入口和 `Read Story` 返回入口。
- 独立 Audio Fiction 列表只展示具备音频的文章。
- 播放器不自动播放，允许暂停、继续、调节音量和拖动进度。

## 数据模型

在现有 `audio_novels` 表增加三个可空字段：

- `audio_path text not null default ''`：本地公开路径，仅允许 `/audio-novel-audio/<32位十六进制文件名>.mp3`。
- `audio_duration text not null default ''`：展示用时长，格式为 `M:SS` 或 `H:MM:SS`。
- `audio_size_bytes bigint not null default 0`：MP3 实际字节数，必须大于等于零。

当前业务是一篇文章最多一个音频，因此不创建独立音频节目表。Audio Fiction 列表和 Podcast 详情从 `audio_novels` 中筛选 `audio_path <> ''` 的已启用、未删除记录。这样可以保持公开行为与生产站一致，同时避免引入当前不需要的一对多模型。

后台模型、创建、编辑和详情响应包含这三个字段；普通列表可以返回时长和大小，但继续不返回正文。公开正文详情返回音频是否存在及 Podcast 路由所需信息；公开音频列表和详情只返回已经启用且文件路径非空的记录。

## 文件存储

新增配置 `AUDIO_NOVEL_AUDIO_DIR`：

- 本地默认：`../data/audio-novel-audio`
- Docker：`/app/data/audio-novel-audio`

Compose 新增独立持久卷 `audio_novel_audio`。该卷只保存 MP3，不与封面目录 `AUDIO_NOVEL_UPLOAD_DIR` 或免费小说目录混用。更新或重建应用容器后音频仍然保留。

公开文件路由：

- `GET|HEAD /audio-novel-audio/*filepath`

继续使用现有安全静态文件处理器。该处理器基于 `http.ServeContent`，支持 `Range`、`Content-Length` 和 `Last-Modified`，因此浏览器播放器可以按需加载并拖动播放位置。MP3 使用普通重新验证缓存策略，不使用一年不可变缓存。

## 后台上传与生命周期

新增管理接口：

- `POST /api/v1/audio-novel-audio`：上传一个 MP3，返回 `path` 和 `size_bytes`。
- `DELETE /api/v1/audio-novels/:id/audio`：清空当前文章的音频字段，并删除当前本地文件。

现有 `POST /api/v1/audio-novels` 和 `PATCH /api/v1/audio-novels/:id` 接受 `audio_path`、`audio_duration` 和 `audio_size_bytes`。音频字段必须作为一个整体：路径为空时，时长为空且大小为零；路径非空时，时长格式有效且大小大于零。

上传规则：

- 单个文件最大 100 MiB。
- 只接受 MP3；同时校验扩展名、浏览器 MIME 和文件头，不仅相信客户端声明。
- 使用 16 字节随机值生成 32 位十六进制文件名。
- 先写入同目录临时文件，`Sync`、关闭成功后原子重命名。
- 拒绝路径片段、符号链接目标和非普通文件。
- 上传接口只保存新文件，不直接修改文章，避免上传失败破坏现有音频。

替换流程为“上传新文件 → 保存文章新字段 → 成功后清理旧文件”。如果文章保存失败，旧音频继续有效，新上传文件作为未关联文件保留并记录服务日志，不能为了清理新文件而误删正在使用的内容。删除文章时，在数据库软删除成功后删除其音频文件；文件删除失败不回滚数据库软删除，但必须记录错误，避免用户看到已删除内容。

## 管理后台

语音小说编辑页在封面区域之后增加“Podcast 音频”区域：

- MP3 文件选择和上传按钮。
- 已上传时显示文件名、格式化大小、时长和可播放预览。
- 时长由浏览器读取 MP3 metadata 后自动填写，允许人工修正为 `M:SS` 或 `H:MM:SS`。
- 提供“替换音频”和“移除音频”。移除使用现有确认对话框，确认后调用文章音频删除接口。
- 新增文章时，音频先上传取得路径，再与文章一起保存。
- 文章保存失败时保留表单和已上传路径，允许重试。

管理列表增加“音频”状态列，显示“已上传 + 时长”或“无音频”，不在列表中加载播放器，避免一次请求加载大量媒体 metadata。

## 公开接口

新增：

- `GET /audio-novel-api/:code/audio?page=&page_size=`：只返回具有音频的公开文章，按发布日期和 ID 倒序分页。
- `GET /audio-novel-api/:code/audio/:slug`：返回 Podcast 信息和对应正文入口。

两个接口沿用现有短码有效性校验，但只读取 `audio_novels`，不与免费小说接口互通。公开接口本身不创建新的入口访问事件；首次打开或刷新前端页面仍由现有 Gin 页面路由记录一次访问。

音频列表响应包含：标题、slug、分类、摘要、封面、发布日期、音频路径、时长和大小。Podcast 详情不返回整篇正文，只返回上述信息和正文 slug，减少响应体积。

## 语音小说前端

新增页面路由：

- `/audio-novel/:code/audio`：Audio Fiction 列表。
- `/audio-novel/:code/audio/:slug`：Podcast 详情。

站点导航增加 `Audio Fiction`。站内所有 RouterLink 继续保留 `code`。

### Audio Fiction 列表

列表参考生产站 `/audio/2026/`：每项展示标题、摘要、发布日期、时长、格式化大小、原生播放器和 `More` 入口。只渲染当前分页项目，播放器使用 `preload="metadata"`，防止页面首次打开下载全部 MP3。

### 正文详情

当正文详情的 `audio_path` 非空时，标题区显示 `Podcast` 入口；否则不占位、不显示禁用按钮。正文阅读能力、字体缩放、沉浸模式和相关推荐保持不变。

### Podcast 详情

页面展示：

1. `Audio Fiction Podcast` 标识。
2. 文章标题、发布日期和摘要。
3. `<audio controls preload="metadata">` 原生播放器。
4. `Download` 链接、时长和文件大小。
5. `Read Story` 返回正文的入口。
6. 现有 WhatsApp 操作入口。

禁止自动播放。播放器加载或播放失败时显示“音频暂不可用”，保留下载重试和返回正文入口。不存在、未启用、已删除或没有音频的 slug 返回 404 页面。

## 2026 年 Podcast 导入

扩展现有 `scripts/import_bcs_2026.rb`：

- 从每期 `Audio Fiction Podcast` 区块读取 Podcast 页面、MP3 URL、时长和文件大小。
- 使用 Podcast 标题与正式文章 slug 对应，不创建第二条文章记录。
- 先完成 11 个远端 MP3 的下载和校验，再逐条上传到本地管理接口并更新对应文章。
- 下载使用流式写入临时文件，不把 10–70MB 文件全部保存在内存中。
- 校验 HTTP 成功状态、`audio/mpeg`、文件头、非零大小和 100 MiB 上限。
- 已有本地音频的文章默认跳过，保证脚本可安全重跑；显式刷新模式才允许替换。
- 单个文件失败时保留该文章原状态，继续处理后续项目，并在结束时列出成功、跳过和失败项目。

首批应关联 11 篇：`Sous Lumière Aigre`、`The Eldest Bird of Paradise`、`My Blue Silk Fan`、`Dreams in Burnished Silver`、`Waiting for The Yellow Ships`、`The Voice and Her Knife`、`The Sparrow Tree`、`The Sea Child`、`Hollow in The Hope`、`Medusa’s Ship, or The Thing About Bodies` 和 `Sing`。

## 错误处理

- 非 MP3、空文件、伪造 MIME、损坏文件或超过 100 MiB：返回 400，不保留最终文件。
- 音频路径、时长和大小不一致：管理接口返回 400。
- 音频不存在或未上架：公开 Podcast 接口返回 404，不泄露后台状态。
- 数据库读取失败：返回 503。
- 静态文件意外丢失：文件请求返回 404；Podcast 页面仍显示文章信息、错误提示和正文入口。
- 磁盘写入、同步或重命名失败：返回 500，清理临时文件。
- 导入下载失败：不修改文章音频字段，并在最终报告列出 URL 和错误原因。
- 浏览器不支持 MP3：显示下载链接作为降级入口。

## 安全与性能

- 音频上传接口继续受后台登录和 `X-Requested-With` 校验保护。
- 文件名由服务端生成，客户端不能指定最终路径。
- 公开路由只允许配置目录内的本地普通文件。
- 播放器不自动加载完整文件；列表和详情均使用 `preload="metadata"`。
- `Range` 响应必须返回正确的 206、`Content-Range` 和音频 MIME。
- 不把 MP3 内容存入 PostgreSQL，数据库只保存路径和元数据。

## 测试与验证

后端测试覆盖：

- 数据迁移默认值、字段约束及历史记录兼容。
- 模型字段组合校验和时长格式。
- MP3 文件头、大小上限、空文件、随机文件名、原子写入和失败清理。
- 音频上传、替换、移除、文章软删除及文件清理。
- Audio Fiction 列表只返回有音频的公开记录。
- Podcast 详情的短码校验、404 和 503。
- 静态 MP3 的 GET、HEAD、MIME、Content-Length 和 Range 206。
- 免费小说及普通短链接路由不受影响。

管理后台测试覆盖：

- 音频字段白名单和接口路径。
- 上传、metadata 时长读取、替换、移除和保存失败提示。
- 列表音频状态，不加载批量播放器。

语音小说前端测试覆盖：

- Audio Fiction 列表和详情路由始终保留 `code`。
- 无音频正文不显示 Podcast 入口。
- 有音频正文正确跳转 Podcast 页面。
- 播放器使用本地路径、禁止 autoplay、设置 `preload="metadata"`。
- 下载、返回正文、加载失败和 404 状态。
- 390px、768px 和 1440px 下播放器、长标题和操作入口不溢出。

导入测试覆盖：

- 从归档区分正式文章、Podcast 和往期推荐。
- 11 个 Podcast 与文章 slug 一一匹配。
- 下载校验、已有音频跳过、单项失败继续和最终汇总。
- 实际导入后数据库存在 11 条非空音频路径，文件均可 HEAD 和 Range 读取。

最终验证运行 Ruby 导入测试、`go test ./...`、`go vet ./...`、运营后台测试与构建、语音小说前端测试与构建、Docker 构建，并在真实浏览器完成音频列表、正文跳转、播放、拖动、下载、错误降级和三档视口检查。

## 完成标准

- 11 个 MP3 均保存在 LinkScope 自己的持久卷中并关联正确文章。
- Audio Fiction 列表、正文 Podcast 入口和 Podcast 详情行为与生产站一致。
- 播放、暂停、拖动、音量和下载正常，网络请求不依赖生产站 MP3。
- 后台可以上传、替换和移除单个完整 MP3。
- 没有音频的文章继续正常阅读且不显示错误入口。
- 语音小说音频数据不进入免费小说接口或数据表。
- 所有自动化测试、构建和浏览器验收通过。
- 不创建或推送 Git 提交。
