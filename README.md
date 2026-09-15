# LinkScope · WhatsApp 投放分析

Vue 3 + JavaScript 管理后台，Go + Gin 采集和跳转服务，PostgreSQL 数据库。后台和落地页分别使用独立 Vue 工程，由同一个 Go 服务提供页面、API、统计与跳转。短链接支持直接跳转与网站落地页两种模式。

## 已实现

- 管理员登录、12 小时会话、注销、后台接口鉴权、跨站请求防护、登录限速。
- 链接创建／编辑／启停、随机短码、多号码、广告 ID / 广告组 / 系列绑定。
- 访问时间、设备／系统／浏览器、来源、地区、匿名 Cookie 状态、过滤分类。
- 同步持久化后跳转；事件写入失败仍跳转并写错误日志，计数在运行状态显示。
- 范围 UV、日／小时趋势、设备／国家／来源分布、分页明细、CSV 导出。
- 广告花费 CSV 导入（事务、重复导入覆盖、不同币种独立展示、日期与时区匹配）。
- Meta 多账户与多 Pixel 配置、真实广告点击识别、咨询 CAPI 队列、测试事件与失败重试。配置和验收见 [Meta 接入说明](docs/meta-setup.md)。
- 审计记录、定期清理、UTC 每日加和指标归档、Docker 部署和 HTTPS 代理示例。

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
6. 设置目标地区适用的采集策略。`COOKIE_MODE=off` 默认不采集访客 Cookie；确定适用条件后可设为 `all`。此版本没有按地区同意管理系统，需同意而没有同意依据时使用 `off`。
7. 已附带 DB-IP City Lite 2026-09 地区数据库，`.env.example` 已配置容器路径。它使用 CC BY 4.0，后台显示署名链接。数据精度有限，建议按月更新；可替换为合法取得的兼容 City MMDB。取消 GEOIP_DB_PATH 时地区显示 unknown。
8. 开始投放前实测 iPhone、Android、电脑、Facebook 内置浏览器。当前本地验收不能替代真实设备和广告账户落地页检查。

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
| `/admin/visits` | 访问明细 |
| `/admin/ads` | 广告分析 |
| `/admin/logs` | 用户端请求日志 |
| `/admin/settings` | 设置 |
| `/admin/meta/connections` | Meta 广告账户分组 |
| `/admin/meta/events` | 咨询回传记录 |
| `/:code` | 原短链接入口，记录访问并返回落地页或跳转 |
| `/:code/contact` | 原咨询提交接口 |
| `/api/v1/*` | 后台 API |
| `/admin-assets/*` | 后台构建资源 |
| `/landing-assets/*` | 落地页构建资源及图片 |

## 本地开发与测试

需要 Go 1.27.1、Node 22 与 PostgreSQL。先分别构建两个前端：

```sh
(cd frontend && npm ci && npm run build)
(cd landing && npm ci && npm run build)
```

然后在 `backend/` 中设置 `DATABASE_URL`、`ADMIN_PASSWORD`、`APP_SECRET` 后启动：

```sh
go run ./cmd/server
```

本地默认读取 `../frontend/dist` 和 `../landing/dist`，可通过 `FRONTEND_DIR`、`LANDING_DIR` 覆盖。Docker 已分别构建并复制两端资源，无需启动两个 Node 服务。

后台热更新：在 `frontend/` 执行 `npm run dev`，API 代理到本机 8080。落地页由 Go 注入真实短链接数据与签名凭证，因此开发时在 `landing/` 执行 `npx vite build --watch`，通过 Go 的 `/:code` 页面刷新检查。正式验收必须重新 `npm run build`，生成配套压缩文件。

```sh
(cd frontend && npm test && npm run build)
(cd landing && npm test && npm run build)
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
