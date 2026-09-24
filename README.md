# LinkScope · WhatsApp 投放分析

Vue 3 + JavaScript 管理后台，Go + Gin 采集和跳转服务，PostgreSQL 数据库。后台、产品落地页与语音小说站分别使用独立 Vue 工程，由同一个 Go 服务提供页面、API、统计与跳转。短链接支持直接跳转与网站落地页两种模式。

## 已实现

- 管理员登录、12 小时会话、注销、后台接口鉴权、跨站请求防护、登录限速。
- 链接创建／编辑／启停、随机短码、多号码、广告 ID / 广告组 / 系列绑定。
- 访问时间、设备／系统／浏览器、来源、地区、匿名 Cookie 状态、过滤分类。
- 同步持久化后跳转；事件写入失败仍跳转并写错误日志，计数在运行状态显示。
- 范围 UV、日／小时趋势、设备／国家／来源分布、分页明细、CSV 导出。
- 广告花费 CSV 导入（事务、重复导入覆盖、不同币种独立展示、日期与时区匹配）。
- Meta 多账户与多 Pixel 配置、真实广告点击识别、咨询 CAPI 队列、测试事件与失败重试。配置和验收见 [Meta 接入说明](docs/meta-setup.md)。
- 免费小说支持 Meta/TikTok 二选一投放、TikTok Pixel + Events API 去重事件、动态广告参数和逐短链阅读漏斗统计。
- 审计记录、定期清理、UTC 每日加和指标归档、Docker 部署和 HTTPS 代理示例。
- 语音小说支持 MP3 上传与原生播放、Meta/TikTok 二选一投放、多投手独立短链、真实播放漏斗、前台可见时长和逐链接事件送达统计。

## 最快启动

需要 Docker 与 Docker Compose。执行目录为本文件所在目录。

```sh
cp .env.example .env
```

在 `.env` 中填写 `POSTGRES_PASSWORD`、`ADMIN_PASSWORD` 和 `APP_SECRET`。建议三个值分别使用 `openssl rand -hex 32` 生成。数据库密码必须使用 URL 安全字符（推荐纯十六进制），因为它会拼接到连接 URL。

```sh
docker compose up -d --build
```

打开 `http://localhost:8080`，以 `.env` 配置的账号密码登录。端口只绑定本机，不会自动发布到公网。数据保存在 Docker 卷，重建应用不会删除数据库。不要执行 `docker compose down -v`，它会删除数据卷。

## 上线配置

1. 准备短域名及管理域名，解析到服务器。
2. 配置 HTTPS 证书，按 `deploy/nginx.conf.example` 配置反向代理。
3. 设置 `PUBLIC_BASE_URL=https://你的短域名`，重启应用。管理后台复制的链接使用此地址。
4. **必须设置可信代理。** 后端只接受 `TRUSTED_PROXIES` 列出的代理转发 IP。宿主机 Nginx 转 Docker 通常呈现 Docker 网桥网关地址；通过 `docker inspect` 确认实际网关后填写精确地址，不要填写 `0.0.0.0/0`。代理必须覆盖而非拼接来自公网的 `X-Forwarded-For`。
5. 从两台不同网络设备访问并验证代理转发；未配置时所有访问可能被识别为网关 IP，地区和频率判断均会失真。没有代理时保持为空。
6. 小说投放统计默认使用 `COOKIE_MODE=all` 创建匿名访客 Cookie，以计算独立访客；它不用于登录，也不读取第三方 Cookie。部署前仍需确认目标地区适用的采集策略；需要同意而没有同意依据时改为 `off`。
7. 已附带 DB-IP City Lite 2026-09 地区数据库，`.env.example` 已配置容器路径。它使用 CC BY 4.0，后台显示署名链接。数据精度有限，建议按月更新；可替换为合法取得的兼容 City MMDB。取消 GEOIP_DB_PATH 时地区显示 unknown。
8. 开始投放前实测 iPhone、Android、电脑、Facebook 内置浏览器。当前本地验收不能替代真实设备和广告账户落地页检查。

## TikTok 小说与语音小说投放上线

TikTok Events API 默认关闭。先保持以下生产配置，再部署数据库迁移、后端、运营后台和小说 H5：

```dotenv
TIKTOK_ENABLED=false
TIKTOK_EVENTS_URL=https://business-api.tiktok.com/open_api/v1.3/event/track/
```

安全启用顺序如下：

1. 完成迁移并部署全部代码，但保持 `TIKTOK_ENABLED=false`，现有 Meta 小说链接不需要修改。
2. 在“TikTok 管理”中新增凭证、Pixel Code 和 Test Event Code，并创建一条专用测试小说或语音小说链接。
3. 只在一个应用实例设置 `TIKTOK_ENABLED=true`，重启后使用 TikTok Pixel Helper、Test Events 和 Payload Helper 验证 PageView、StartReading（文字）或 StartListening（语音）、ViewContent 及事件去重。
4. 验证通过后清空 Test Event Code，再在全部应用实例开启 TikTok；新建正式 TikTok 链接进行低流量投放。
5. 至少观察一个完整归因窗口。后台“TikTok 已接收”只表示 Events API 接收成功，最终是否归因必须到 TikTok Ads Manager 查看。

Access Token 只保存在服务端加密字段中，不写入前端或投放 URL。Pixel/凭证停用时小说阅读和内部统计继续工作，服务端事件暂停发送；恢复后仅在 24 小时安全窗口内继续重试。

## 语音小说投放与统计

语音内容只维护一份，但每位投手或每次 Campaign 都要创建独立投放链接。推荐按以下顺序操作：

1. 先在“语音小说管理”新增内容并上传 MP3；没有可播放 MP3 的内容不能创建正式投放链接。
2. 在该内容行点击“投放链接”，为每位投手或每次投放分别新建链接。
3. 每条链接只选择一个广告平台和该平台的一个 Pixel：Meta 与 TikTok 不能同时选择。链接首次产生正常访问后，绑定内容、平台和 Pixel 会锁定；需要更换时请新建链接。
4. “达标播放时长”默认 10 秒。只有原生音频实际播放累计达到门槛才产生一次 ViewContent；暂停、缓冲和拖动进度条不会虚增时间。播放至音频结束且媒体消费达到 90% 只记为站内“完成”，不会新增广告平台事件。
5. Meta 广告 URL 应按 [Meta 接入说明](docs/meta-setup.md) 添加动态 campaign_id、adset_id、ad_id 等参数。TikTok 使用后台的“复制 TikTok 投放模板”，由广告系统展开 campaign_id、adgroup_id、creative_id、ad_id_v2 和 placement；ttclid 由 TikTok 点击自动附加，不应由运营人员手填或复用。
6. 在链接列表点击“统计”查看该链接自己的访问、匿名 UV、前台可见时长、实际播放漏斗与事件送达状态。后台“平台已接收”只表示 Events API 已接收；是否最终归因必须在 Meta Ads Manager 或 TikTok Ads Manager 查看，不能用站内完成数或 API 接收数替代。

旧的 /audio-novel/{code} 普通短链仍进入公共语音小说首页。新投放链接直接打开绑定的音频详情。内容停用、删除或移除 MP3 时，新投放链接返回 410 且不新增正常访问；恢复内容并重新上传 MP3 后，原链接可继续使用，历史统计不会被删除。

## 免费小说多语言翻译

运营后台始终录入英文原文。在“小说管理 → 编辑小说”的翻译区域勾选目标语言，再点击独立的“生成翻译”按钮。当前支持印度尼西亚语、日语、韩语、马来语、葡萄牙语、菲律宾语、泰语和越南语。后端异步生成翻译，其中越南语使用 DeepL，其余语言继续使用 APIHZ；标题、简介和全部启用章节全部成功后才一次性发布，中途失败不会把残缺内容提供给读者，重新生成期间旧译文仍可继续访问。

后端需要设置 `APIHZ_TRANSLATION_ID`、`APIHZ_TRANSLATION_KEY` 和越南语使用的 `DEEPL_AUTH_KEY`，两个服务的 URL 均已有默认值。凭证只能放在服务器 `.env`，不能使用 `VITE_` 前缀，也不能写入前端源码。缺少对应供应商凭证时英文小说和既有译文照常工作，仅该语言的“生成翻译”任务会返回配置错误。

读者语言依次按 URL 的 `lang` 参数、此前手动选择、IP 国家和英文兜底决定。页面右上角可切换已成功发布且启用的语言，选择会保存在同站 Cookie 与本地存储中。运营人员修改英文原文后，现有译文继续展示并标记为需要重新生成；新版本完整成功后才替换旧版本。

## Cookie 与统计定义

生产 HTTPS Cookie 使用 `__Host-sid`，`Secure; HttpOnly; SameSite=Lax; Path=/`，30 天；随机值附 HMAC 防篡改。HTTP 本地预览使用不同开发 Cookie 名称。首次下发不代表浏览器成功保存。只读取自有短域名 Cookie，不读取其他应用 Cookie。

- 总访问：已成功持久化的短链接 GET/HEAD 请求，不含不存在或停用链接。
- 过滤后点击：未命中当前异常规则的 GET 访问，**不等于确认真人**。
- UV：筛选范围内正常访问的不同匿名访客 ID；无 Cookie 访问不臆测人数。
- 日／小时 UV 分桶计算，范围 UV 独立去重，不能累加。
- `bot` 为请求头命中规则，并非密码学验证的机器人身份；`suspicious` 为高频等异常。
- 单个 IP 摘要 1 分钟超过 120 次时标可疑，不拦截跳转。同网用户可能误判。
- 分类包括 normal、bot、suspicious、unclassified、head、prefetch；保留原因及规则版本。
- 地址绑定广告优先，请求参数冲突另外记录。转发后的访问仍归属这个投放入口。
- 访问分析表不持久化原始 IP、完整 Referrer 或原始 User-Agent。请求日志用于排障，单独保存来源 IP、脱敏请求头和参数，保留 7 天。
- 广告参数白名单保存，长度受限；fbclid 不作为个人身份或真人证据。
- UTC 保存事件，可切换报表时区；地区是 IP 推断，不是 GPS。

设置页面是部署配置的只读展示；修改环境变量后重启生效。

## 广告花费导入

UTF-8 CSV，最多 2 MB / 10000 行，固定表头：

```csv
date,ad_id,amount,currency,time_zone
2026-09-08,广告ID,25.5000,USD,Asia/Shanghai
```

金额非负且最多四位小数，币种为三位大写代码。相同日期、广告、币种、时区重复导入会覆盖原金额；同一个文件不允许重复键。以广告账户的日预算统计时区填写。

**必须覆盖筛选期间每一天，零花费日期也导入 0。** 缺失日期、时区不匹配或筛选单链接时，成本显示“—”，不会用部分花费错误计算整个期间成本。不同币种不可相加。点击不代表已发送消息或成交。

## 数据保留、故障和备份

每 6 小时运行保留任务，默认保留 90 天明细（按 UTC 日界清理，最多多保留不足一天）。UTC 每日总访问、过滤后、机器人、可疑指标保留 365 天，存于 `daily_totals`；当前交互报表查询明细，超过明细保留期的 UV 不可用。归档不包含跨期 UV，也不伪造它。

数据库可读但写入失败时继续跳转，输出 `CLICK_WRITE_FAILED`，进程内计数可见；应用重启后计数归零，持久告警应由运维日志平台接收。数据库完全不可读时无法取得链接，返回 503。没有承诺故障期间零丢数。

数据库备份示例（部署账号执行）：

```sh
mkdir -p backups
docker compose exec -T db pg_dump -U analytics -d analytics -Fc > backups/analytics.dump
```

请把备份复制到独立加密存储，并通过服务器备份调度每天执行；本项目不擅自创建用户机器的定时任务。定期在独立空数据库使用 `pg_restore` 验证恢复。生产恢复应先暂停写入并备份当前库，不直接覆盖线上数据。

## 目录与路由

```text
frontend/                       运营后台 Vue 3 + Element Plus
  src/views/                    每个业务页面一个 Vue 文件
  src/router/                   /admin/* 路由与登录校验
  src/layouts/                  后台公共布局
  src/components/               仅跨页面复用组件
  src/api/                      按业务模块封装接口
  src/stores/                   共享登录状态与设置
  src/composables/               报表筛选、加载与分页逻辑
  src/utils/                    纯函数
  src/styles/                   全局基础样式
landing/                        用户落地页独立 Vue 3 工程
  src/views/                    落地页与错误页
  src/router/                   短码路由
  src/lib/                      手动提交与倒计时逻辑
  public/landing-assets/images/ 产品图片
audio-novel/                    独立语音小说 Vue 3 工程
  src/lib/api.js                从同一 Gin 服务读取全局语音小说内容
  src/views/                    首页、列表、详情和错误页
  public/audio-novel-assets/images/ 项目原创奇幻视觉素材
novel-h5/                       独立免费小说 Vue 3 H5 工程
  src/views/                    首页、搜索、列表、阅读与错误页
  src/components/               小说卡片、底部导航和章节目录
backend/
  cmd/server/                   唯一服务入口
  internal/bootstrap/           初始化与关闭服务
  internal/config/              环境配置
  internal/transport/http/      注册路由
  internal/modules/             auth、links、analytics、tracking 等业务
  internal/platform/            数据库、资源缓存、地理信息、运行状态
  internal/jobs/                数据保留任务
  internal/platform/database/migrations/ 版本化数据库迁移
```

页面遵循 `template → script setup → style scoped`，使用 JavaScript。页面专用表单、弹窗直接放在页面中，不创建页面内的 `components/` 目录。

| 路径 | 用途 |
| --- | --- |
| `/` | 跳到 `/admin/overview` |
| `/admin/login` | 登录 |
| `/admin/overview` | 数据总览 |
| `/admin/links` | 短链接管理 |
| `/admin/audio-novels` | 语音小说列表、新增、编辑、启停、推荐和删除 |
| `/admin/audio-novels/:id/links` | 某篇语音小说的独立投放链接 |
| `/admin/audio-novel-links/:id/stats` | 单条语音投放链接的访问、播放漏斗与事件状态 |
| `/admin/novels` | 免费小说、封面和章节内容管理 |
| `/admin/visits` | 访问明细 |
| `/admin/ads` | 广告分析 |
| `/admin/logs` | 用户端请求日志 |
| `/admin/settings` | 设置 |
| `/admin/meta/connections` | Meta 广告账户分组 |
| `/admin/meta/events` | 咨询回传记录 |
| `/admin/tiktok/pixels` | TikTok Pixel 与 Test Event Code 管理 |
| `/admin/tiktok/connections` | TikTok Events API 凭证管理 |
| `/admin/tiktok/events` | TikTok 服务端事件状态与人工重试 |
| `/:code` | 原短链接入口，记录访问并返回落地页或跳转 |
| `/:code/contact` | 原咨询提交接口 |
| `/audio-novel/:code` | 语音小说站首页，并记录 `audio_novel` 入口访问 |
| `/audio-novel/:code/stories` | 内容列表，每页 6 篇 |
| `/audio-novel/:code/stories/:slug` | 内容详情与阅读控制 |
| `/audio-novel/:code/audio` | 可播放语音小说列表 |
| `/audio-novel/:code/audio/:slug` | 语音详情、原生播放器与投放播放跟踪 |
| `/audio-novel/:code/contact` | 语音小说站手动 WhatsApp 咨询 |
| `/audio-novel/:code/time-spent` | 达到 code 配置阈值后的可见停留上报 |
| `/audio-novel/:code/view` | 确认投放入口 PageView，不新增访问 |
| `/audio-novel/:code/start-listening` | 音频首次真实播放后的签名上报 |
| `/audio-novel/:code/playback-time` | 单调累计实际播放与媒体消费进度 |
| `/audio-novel/:code/visible-time` | 单调累计投放页前台可见秒数 |
| `/audio-novel/:code/complete` | 音频结束且消费达到 90% 后记录站内完成 |
| `/audio-novel-api/:code/home` | 公开首页推荐内容，只验证短码、不新增访问事件 |
| `/audio-novel-api/:code/stories` | 公开分页内容 |
| `/audio-novel-api/:code/stories/:slug` | 公开详情与相关推荐 |
| `/audio-novel-api/:code/audio` | 公开可播放语音列表 |
| `/audio-novel-api/:code/audio/:slug` | 公开语音详情 |
| `/audio-novel-uploads/*` | 后台上传并持久化的封面 |
| `/api/v1/audio-novel-links` | 语音小说投放链接管理 |
| `/api/v1/audio-novel-links/:id/stats` | 单条语音投放链接统计 |
| `/novel/:code` | 免费小说首页，并记录 `novel` 入口访问 |
| `/novel/:code/search` | 免费小说搜索页 |
| `/novel/:code/stories` | 免费小说列表页 |
| `/novel/:code/stories/:slug` | 小说详情、正文阅读与章节目录 |
| `/novel/:code/view` | 确认本次小说站 PageView，不重复增加入口访问 |
| `/novel/:code/start-reading` | 第一章成功显示后的签名 StartReading 上报 |
| `/novel/:code/time-spent` | 达到短码配置阈值后的可见停留上报 |
| `/novel/:code/reading-time` | 累计前台可见秒数并返回服务端确认的达标事件 |
| `/novel-api/:code/home` | 免费小说首页数据 |
| `/novel-api/:code/stories` | 免费小说搜索与分页列表 |
| `/novel-api/:code/stories/:slug` | 小说详情、目录及相关推荐 |
| `/novel-api/:code/stories/:slug/chapters/:number` | 单章安全 HTML 正文与前后章 |
| `/novel-uploads/*` | 免费小说封面 |
| `/api/v1/*` | 后台 API |
| `/admin-assets/*` | 后台构建资源 |
| `/landing-assets/*` | 落地页构建资源及图片 |
| `/audio-novel-assets/*` | 语音小说站构建资源及原创图片 |
| `/novel-assets/*` | 免费小说 H5 构建资源 |

## 本地开发与测试

需要 Go 1.27.1、Node 22 与 PostgreSQL。先分别构建四个前端：

```sh
(cd frontend && npm ci && npm run build)
(cd landing && npm ci && npm run build)
(cd audio-novel && npm ci && npm run build)
(cd novel-h5 && npm ci && npm run build)
```

然后在 `backend/` 中设置 `DATABASE_URL`、`ADMIN_PASSWORD`、`APP_SECRET` 后启动：

```sh
go run ./cmd/server
```

本地默认读取 `../frontend/dist`、`../landing/dist`、`../audio-novel/dist` 和 `../novel-h5/dist`，可通过对应的 `*_DIR` 环境变量覆盖。Docker 会构建并复制四端资源。

免费小说与语音小说是两套独立内容。免费小说正文按章节保存 Markdown，后端只向 H5 返回清洗后的 HTML；站点不包含登录、支付或章节锁。封面保存到 `NOVEL_UPLOAD_DIR`，Docker 使用独立 `novel_uploads` 持久卷。

语音小说内容全局共享，只需维护和上传一次。旧普通短码不绑定单篇内容，继续打开公共语音小说首页并沿用原 WhatsApp、Meta 和页面停留配置；新语音投放链接绑定一篇带 MP3 的内容，并独立冻结平台、Pixel、门槛和动态归因。正文以 Markdown 保存，公开 HTML 由后端安全渲染。封面与 MP3 分别保存到语音小说上传目录；Docker 使用 `audio_novel_uploads` 持久卷。

从旧开发卷迁移时，先停止应用容器但保留数据库，再把 `linkscope_novel_uploads` 只读复制到 `linkscope_audio_novel_uploads`。确认新卷文件完整前不要删除旧卷，具体命令见 `docs/verification.md` 的 2026-09-21 记录。

本地热更新：保持 Go 服务运行在 8080，然后执行 `./scripts/dev-local.sh`。运营后台使用 `http://localhost:5173/admin/login`；产品落地页使用 `http://localhost:5174/:code`；语音小说站使用 `http://localhost:5175/audio-novel/:code`；免费小说站使用 `http://localhost:5176/novel/:code`。生产入口由 Gin 在 8080 端口注入真实短链接数据。

```sh
(cd frontend && npm test && npm run build)
(cd landing && npm test && npm run build)
(cd audio-novel && npm test && npm run build)
(cd novel-h5 && npm test && npm run build)
(cd backend && go test ./... && go vet ./...)
```

数据库集成测试必须使用专用可销毁测试库，测试会清空 public schema：

```sh
cd backend
TEST_DATABASE_URL='postgres://user:password@localhost:5432/app_test?sslmode=disable' \
META_TEST_DATABASE_URL='postgres://user:password@localhost:5432/meta_test?sslmode=disable' \
go test -race ./...
```

未配置测试库时，数据库测试会跳过。页面路由、表单和统计集成检查也应在独立测试库进行，避免污染投放数据。

## 范围限制

单管理员自用系统；账号来自部署配置，不含多人权限或自助密码修改。Meta 接入需要账户授权及服务端凭证，不包含 WhatsApp 消息回调或订单归因。链接通过启停状态控制；明细导出单次最多 100000 条。请求日志可在后台查看；运维告警、备份异地存储和地区数据库更新由部署方配置。

代码仓库：[meta_shortLink](https://github.com/1047415098/meta_shortLink)。

## 网站落地页与请求日志（2026-09-09）

- 短链接的 `mode` 支持 `redirect` / `landing`。历史链接默认保持直接跳转，新建链接表单默认落地页。
- 落地页编辑：后台 → 短链接管理 → 编辑 → 网站落地页；可设置品牌、标题、产品简介、详细介绍。Go 在页面入口记录一次访问，再注入安全转义的 JSON 数据与签名凭证，由独立 Vue 页面渲染。所有访问者看到相同业务内容。新建链接默认 3 秒，已有链接设置不变。
- `GET /:code` 记录访问并返回 200；`POST /:code/contact` 在明确点击后返回 303。签名凭证有效期 1 小时，绑定访问记录和短码；不依赖访客 Cookie，重复提交只标记一次。停用或已改为直接跳转的链接拒绝旧表单。
- 总访问、时间趋势、来源等仍按入口访问计算，按钮提交不会额外增加访问总数。展示“用户端网站访问”、“手动咨询点击”和“自动跳转咨询”，均排除已分类异常；按钮指标按原始访问日期筛选（访问群组口径），不等于消息发送或进入聊天。访问明细及 CSV 增加页面类型和按钮点击时间。
- 落地页访问写入失败时仍展示页面，咨询按钮降级为直接 WhatsApp 链接，此时该次访问/按钮无法统计。点击提交时数据库失败则返回错误，允许用户重试。
- 后台 → 日志管理：每个到达应用的访客端请求（`/:code`、`/:code/contact`）记录请求编号、时间、路径、方法、来源 IP、请求头、URL 参数、正文、响应头、响应正文、状态码、处理耗时和大小。支持日期、路径、方法、状态码筛选和详情查看，仅管理员可访问。
- JSON/表单正文最多保存 32 KB。密码、Cookie、Token、Authorization、API key、ticket、session 等字段脱敏；登录接口正文及日志查询正文不保存，避免凭证泄漏及递归日志。文件、HTML、非法 JSON 及超限正文只保存说明/大小。不保证识别任意业务文本里混入的敏感信息。
- 日志自动保留 7 天（随现有每 6 小时清理任务运行），访问统计明细仍按原保留周期。来源 IP 的可靠性取决于可信代理配置。ngrok 自身拦截、TLS 握手失败等未到达应用的请求无法由本系统记录。
- 请求日志与业务写入失败计入运行状态中的写入失败数，同时输出服务日志。数据日志写入失败不回滚已经成功的业务操作。

日志范围为访客端：短链接/落地页访问 `/:code` 和咨询提交 `/:code/contact`。后台 `/api/v1/` 接口、静态资源和健康检查均不记录；历史后台/资源日志从列表和详情中隐藏，按原 7 天周期自动过期。既有短链接对应的历史访客日志仍可查询。业务点击统计不受影响。

统计总览突出两个维度：用户端网站访问（正常分类落地页请求）和咨询按钮点击（这些访问中已提交咨询的次数，同次访问去重）。趋势图展示网站访问、手动咨询和自动跳转三条序列，咨询计入原始访问日期；估算访客保留为独立指标；总入口请求、过滤后入口访问、可疑访问卡片已隐藏。请求日志额外展示“网站入口访问”或“咨询提交”动作，失败请求也会保留状态码，不等同于咨询成功。

## 定时跳转

后台短链接编辑 → 网站落地页 → 定时跳转（秒）：0 关闭，1–300 秒启用。页面显示倒计时，时间到达自动提交跳转表单；手动点击仍即时跳转并取消定时器。所有访客遵循同一配置。浏览器后台标签页可能延后执行，Vue 页面需要 JavaScript；禁用时展示明确提示。

自动提交携带 trigger=auto，写入 auto_redirected_at，同次访问去重，不增加 whatsapp_clicked_at。总览咨询卡片上下展示咨询点击总数、手动咨询点击、自动跳转咨询；总数按两项相加，趋势图分开显示两类序列。访客日志标记“定时跳转”，访问明细和导出包含自动跳转时间。两个主要维度仍为网站访问与手动咨询；行为分类来自客户端信号，不代表真人证明。

定时跳转会前往 WhatsApp 网页；自动唤起手机应用由浏览器和系统决定。广告平台仍可能识别最终目标，不能据此保证广告获批。

## English PEPLYRA landing page

The landing template is now English and uses the user-specified PEPLYRA catalog as its content/visual reference. Six linked product cards, a laboratory banner and the brand mark are served from local `/landing-assets/images/` files. Product prices and inventory are not cached or displayed; product cards open the corresponding official pages. Main copy stays editable in the admin link form. Admin UI stays Chinese. Existing WhatsApp targets, countdown settings, signed submissions and manual/automatic analytics are unchanged. See `docs/landing-reference.md` for source details.

### 静态资源压缩与缓存

生产构建会生成 JS/CSS 的 Brotli 和 gzip 副本，服务按浏览器的 Accept-Encoding 选择；未支持压缩的客户端仍可读取原文件。带 Vite 内容版本号的 JS/CSS 缓存一年，更新构建时文件名自动变化。后台 HTML、API 和访问统计入口保留 no-store。落地页图片使用 ETag/Last-Modified 校验缓存，未变化返回 304，修改内容后可重新获取。不要手动覆盖同一个带版本号的 JS/CSS 文件。
