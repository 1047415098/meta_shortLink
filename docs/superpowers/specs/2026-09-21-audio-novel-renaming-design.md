# 语音小说项目整体重命名设计

## 目标

把现有“小说”子系统整体重命名为“语音小说”。本次只迁移项目命名、路径和配置，不增加音频上传、音频转码、播放器或语音合成功能。

迁移完成后，新代码中不再使用 `novel` 表示当前业务模块；历史设计文档中的背景说明可以保留，但所有可执行配置、源代码、测试和当前使用文档必须使用 `audio-novel` 或 `audio_novel`。

## 已确认边界

- 系统尚未发布，不保留旧 `/novel/*` 地址兼容。
- 已有小说正文、封面路径、访问记录和 Meta 事件关系必须保留。
- 短链接 `code`、WhatsApp 目标、Meta Pixel、归因和 TimeSpent 规则保持不变。
- 仍由同一套 Gin 服务构建和托管管理后台、短链接落地页和语音小说前端。
- 不创建 Git 提交。

## 统一命名

| 层级 | 旧名称 | 新名称 |
| --- | --- | --- |
| 前端目录 | `novel/` | `audio-novel/` |
| npm 包名 | `linkscope-novel` | `linkscope-audio-novel` |
| Go 模块目录 | `internal/modules/novel` | `internal/modules/audionovel` |
| Go 包名 | `novel` | `audionovel` |
| 配置字段 | `NovelDir` | `AudioNovelDir` |
| 上传配置字段 | `NovelUploadDir` | `AudioNovelUploadDir` |
| 环境变量 | `NOVEL_DIR` | `AUDIO_NOVEL_DIR` |
| 上传环境变量 | `NOVEL_UPLOAD_DIR` | `AUDIO_NOVEL_UPLOAD_DIR` |
| 数据表 | `novels` | `audio_novels` |
| 访问 surface | `novel` | `audio_novel` |
| 管理页面名称 | 小说管理 | 语音小说管理 |

Go 标识符使用 `AudioNovel`，URL 和磁盘目录使用 `audio-novel`，数据库及访问类型使用 `audio_novel`，避免在同一层混用多个写法。

## 路由契约

### 公开页面

- `GET|HEAD /audio-novel/:code`
- `GET|HEAD /audio-novel/:code/stories`
- `GET|HEAD /audio-novel/:code/stories/:slug`
- `POST /audio-novel/:code/view`
- `POST /audio-novel/:code/contact`
- `POST /audio-novel/:code/time-spent`

### 公开内容接口

- `GET /audio-novel-api/:code/home`
- `GET /audio-novel-api/:code/stories`
- `GET /audio-novel-api/:code/stories/:slug`

### 管理接口

- `GET|POST /api/v1/audio-novels`
- `GET|PATCH|DELETE /api/v1/audio-novels/:id`
- `PATCH /api/v1/audio-novels/:id/status`
- `PATCH /api/v1/audio-novels/:id/featured`
- `POST /api/v1/audio-novels/preview`
- `POST /api/v1/audio-novel-covers`

### 静态资源

- `/audio-novel-assets/*`
- `/audio-novel-uploads/*`

所有旧 `/novel/*`、`/novel-api/*`、`/novel-assets/*`、`/novel-uploads/*` 和管理接口在迁移后不注册，访问时返回 404。

## 前端结构

独立 Vue 3 工程移动到 `audio-novel/`，保留首页、列表、详情、WhatsApp 咨询和可见停留计时功能。Vue Router、API 客户端、Meta/TimeSpent 上报地址、Vite 代理、静态资源基路径和测试断言全部切换到新路由。

管理后台菜单和页面路由切换为 `/admin/audio-novels`，API 客户端切换到 `/api/v1/audio-novels`。表单仍管理 Markdown 正文和封面，本次不显示尚未实现的音频字段。

## Gin 与配置

后端模块目录和包名改为 `audionovel`，应用装配字段改为 `AudioNovel`。渲染目录读取 `AUDIO_NOVEL_DIR`，本地默认 `../audio-novel/dist`；上传目录读取 `AUDIO_NOVEL_UPLOAD_DIR`，本地默认 `../data/audio-novel-uploads`。

路由继续注册在通用 `/:code` 之前，防止 `audio-novel` 被识别为普通短码。请求日志、删除短链接时的日志清理、Meta 事件来源 URL 和 TimeSpent 签名范围全部改用新路径及 `audio_novel` surface。

签名上下文由 `contact:novel:` 改为 `contact:audio_novel:`。迁移后旧页面票据失效符合“不保留旧兼容”的要求。

## 数据库迁移

新增顺序迁移文件，执行以下原地变更：

1. `ALTER TABLE novels RENAME TO audio_novels`，保留所有行、主键、时间和软删除状态。
2. 重命名仍带 `novels_` 前缀的索引及 identity sequence，使数据库对象名称一致。
3. 将 `click_events.surface='novel'` 更新为 `audio_novel`。
4. 重建 `click_events_surface_check`，只允许 `short_link` 与 `audio_novel`。
5. 保留名称已经通用的 `clicks_surface_time` 索引，不删除历史访问或 Meta 事件。

迁移必须可在已有数据和空数据库上执行。应用只使用新表名，不创建第二份内容表，避免数据分叉。

## Docker 与文件迁移

Docker 构建阶段改名为 `audio-novel`，构建上下文读取 `audio-novel/`，产物复制到 `/app/audio-novel`。运行环境使用：

- `AUDIO_NOVEL_DIR=/app/audio-novel`
- `AUDIO_NOVEL_UPLOAD_DIR=/app/data/audio-novel-uploads`

Compose 持久卷改名为 `audio_novel_uploads`。部署前将旧 `novel_uploads` 卷中的文件复制到新卷，再启动新容器；即使当前卷为空，也采用同一安全迁移流程。迁移完成前不删除旧卷，便于回退。

## 文档范围

README、部署说明、本地预览说明和当前验证文档更新为新名称。历史方案文档只修改会被复制执行的文件路径和命令，不改写当时的业务决策记录。

## 测试与验收

- 数据库迁移测试确认内容数量、ID、slug、featured 状态和历史访问数量不变。
- Go 全量测试和静态检查通过，代码中不存在运行时 `modules/novel`、`NovelDir` 或旧路由注册。
- 管理后台测试覆盖列表、新增、编辑、删除、启用、推荐、Markdown 预览和封面上传。
- 语音小说前端测试覆盖首页、列表、详情、咨询、PageView 和 TimeSpent。
- 三个前端均能完成生产构建，Docker 镜像能完成构建。
- 浏览器检查桌面和窄屏下的首页、列表、详情与后台表单。
- 新路由返回正常内容，旧路由返回 404。
- 服务 `/healthz` 正常，数据库迁移记录存在。

## 回退原则

代码切换前保留旧数据库备份和旧上传卷。若新版本启动失败，停止新容器并恢复旧镜像；数据库表和 surface 需要通过明确的反向迁移恢复名称，禁止直接删除新表或上传卷。
