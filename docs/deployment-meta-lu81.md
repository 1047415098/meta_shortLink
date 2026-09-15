# meta.lu81.com 部署记录

部署日期：2026-09-14。后台和用户落地页共用一个 Go 服务，使用全新 PostgreSQL 数据库；未迁移本地短链接、访问数据或 Meta 凭证。

## 地址与目录

- 后台：https://meta.lu81.com/admin/overview
- Meta Pixel：https://meta.lu81.com/admin/meta/pixels
- 服务器：154.219.116.227，SSH 端口 22。
- 项目目录：`/opt/linkscope`，包含 `backend/`、`frontend/`、`landing/` 源码及构建资源。
- 服务配置：`/opt/linkscope/deploy/compose.production.yaml`。
- 生产环境文件：`/opt/linkscope/.env`，权限 0600。数据库密码、应用签名密钥和独立 Meta 加密密钥在服务器现场生成，未写入源码或传回本地。
- 发布包与安装日志：`/opt/linkscope-release`；运行环境安装材料：`/opt/linkscope-bootstrap`。
- 管理员账号沿用用户指定的 `admin`；密码按用户指定值配置，不在此文档记录。

## 实际运行结构

CentOS 7 x86_64，内核 3.10，约 4 GB 内存、60 GB 磁盘。使用官方静态 Docker 29.8.0、Compose 5.5.1 和 Buildx 0.37.1；这些静态工具的版本升级需单独维护。

| 服务 | 运行方式 | 访问范围 |
| --- | --- | --- |
| Go + 两个 Vue 前端 | `linkscope-lu81-app-1` | 仅宿主机 `127.0.0.1:8080` |
| PostgreSQL 17 | `linkscope-lu81-db-1` | 容器内部网络，不发布数据库端口 |
| Nginx 1.30.4 | `linkscope-lu81-nginx-1`，宿主机网络 | 公网 80、443 |
| Certbot 5.8.0 | 申请和续期时按需启动容器 | 证书目录持久化到宿主机 |

Nginx 覆盖客户端传入的转发头，Go 只信任实测 Docker 网关。生产镜像摘要已固定在 Compose 配置中；应用、数据库和 Nginx 设置了自动恢复，Docker 服务已设为开机启动。

数据库保存在 `linkscope-lu81_postgres_data` 卷。重新创建应用容器不会清空数据库；不要运行带 `-v` 的 Compose 清理命令。保留生产 `.env` 和证书目录，尤其不要在接入 Meta 后更换或丢失加密密钥。

服务器原 DNS `223.5.5.5` 对 Docker 仓库返回异常地址，已备份后改用 `1.1.1.1`，并同步网卡的 DNS1 配置以便重启后保留。原配置位于 `/opt/linkscope-bootstrap/resolv.conf.before` 和 `ifcfg-eth0.before`；未修改网卡 IP、网关或 SSH 配置。

## HTTPS 与续期

- 域名 A 记录：`meta.lu81.com → 154.219.116.227`，没有 AAAA 记录。
- 证书机构：Let's Encrypt。
- 当前证书到期：2026-12-13 08:32:42 UTC。
- 证书目录：`/etc/letsencrypt/live/meta.lu81.com/`。
- HTTP 自动跳转 HTTPS；`/.well-known/acme-challenge/` 保留 HTTP 访问，用于续期。
- 定时器：`linkscope-certbot-renew.timer`，每日 UTC 00:00、12:00 检查（北京时间 08:00、20:00）。
- 续期成功后先检查 Nginx 配置，再平滑加载证书。
- 已通过 Certbot `--dry-run --no-random-sleep-on-renew` 模拟续期和重载检查。

```bash
# 查看服务状态。
cd /opt/linkscope
docker compose --env-file .env -f deploy/compose.production.yaml ps

# 检查证书续期定时器及上次任务日志。
systemctl status linkscope-certbot-renew.timer
journalctl -u linkscope-certbot-renew.service --no-pager -n 50

# 手动执行续期模拟验证，不替换正式证书。
sh /opt/linkscope/deploy/certbot-renew.sh --dry-run --no-random-sleep-on-renew

# 查看应用最近日志。
docker compose --env-file .env -f deploy/compose.production.yaml logs --tail 100 app
```

## 验收结果

- 公网 HTTPS 健康检查 200，客户端 TLS 验证结果为 0（通过），HTTP 跳转到同域名 HTTPS。
- 浏览器可以使用配置账号登录后台，刷新和应用容器重建后登录状态保留。
- 后台页面、Meta 配置页面和 5 个入口静态资源请求通过；带版本号资源返回长期缓存标记。
- 未登录查询 Meta 凭证接口返回 401。
- 专用测试链接 `deploycheck0914`：3 次接口模拟访问，2 位访客，手动咨询 1 次、自动咨询 1 次。重复提交同一访问、同一类型事件未重复计数。
- 浏览器实测 3 秒倒计时和头部咨询按钮，均通过本站接口跳转到目标号码的 WhatsApp 页面。
- 加上浏览器测试后，后台与数据库一致：5 次访问、3 位访客、手动咨询 2 次、自动咨询 2 次，咨询总数 4 次。重建应用容器后数据仍保留。
- 测试短链接、5 条访问记录、14 条相关请求日志和 2 条测试操作记录已按用户授权删除。保留系统初始化的默认 `hello` 入口，其访问数据为 0；没有迁移任何本地记录。
- 公网开放后收到的其他路径探测日志属于实际公网请求，未作为测试数据删除。

## 使用前的业务配置

生产环境 Meta 账户、Pixel、凭证和回传事件均为空，需在新后台重新填写并验证授权，再将短链接关联到所需 Pixel。本次部署测试没有向 Meta 发送事件。

访客 Cookie 已按此前确认开启，时间报表使用 Asia/Shanghai，IP 地区库已加载。咨询统计表示按钮点击或自动跳转，不能证明 WhatsApp 聊天界面已经打开或消息已经发送。

## 2026-09-14 日志 URL token 展示更新

已部署迁移 `004_request_log_query_token.sql`。新增请求的 URL 参数 `token` 加密保存，并在已登录管理员的请求详情中直接显示原值；重复参数完整展示。历史脱敏记录无法恢复原值，界面会说明原因。密码、Cookie、Authorization 和其他敏感字段继续按原规则脱敏。

加密依赖当前 `APP_SECRET`，备份和迁移时必须保留；日志仍保留七天。每次返回明文前记录 `request_log.token_view` 审计，只保存请求编号。

更新前的数据库快照、源码与构建资源位于服务器 `/opt/linkscope-release/log-token-20260914/`；旧应用镜像为 `linkscope-lu81:before-log-token-20260914`。回退应用时保留新增列及现有生产数据，不直接恢复数据库快照覆盖后续业务记录。

验证：独立 PostgreSQL 上的后端测试、静态检查、前端测试和构建通过。公网 HTTPS 详情接口、未登录拒绝、列表不暴露原值、数据库加密及浏览器明文展示均通过。使用专用不存在的路径和假 token 验证，没有创建短链接、访问统计或 Meta 回传；相关测试日志和查看记录已清理。

## 2026-09-15 短链接广告统计

短链接列表的「统计」进入独立 Vue 页面 `/admin/links/:id/stats`，调用登录保护的 `POST /api/v1/links/:id/stats`（初版 GET 已在下述更新中替换）。展示广告/来源、访问、独立访客、手动咨询、自动跳转，支持日期、时区、来源值筛选以及全量排序、每页 50 组分页。按用户要求不展示花费和咨询成本；无需 Meta 读取授权，也不调用 Meta 网络接口。

数据口径：仅统计当前短链接中 `event_type=landing`、`classification=normal` 的访问。按进入网站的日期筛选，咨询沿用同一次访问各自去重；总体 UV 从原始访问中的非空 Cookie 标识重新计算，不能相加每个广告的 UV。Meta 账户使用访问时的快照分组，修改链接绑定不会改写历史归属。

优先使用已记录的 `ad_id`；只有 `utm_content` 的旧数据单列为未确认的 UTM 来源，不直接当作广告 ID，缺少两者时显示未识别来源。名称优先使用账户匹配的已同步 Meta 广告名称，其次是与广告 ID 一致的访问参数名称。不会自动修改历史数据或公开凭证。

备份目录：`/opt/linkscope-release/link-stats-20260915/`，包括数据库快照、原源码与原构建资源、发布包和构建日志。旧镜像：`linkscope-lu81:before-link-stats-20260915`。本次没有数据库结构迁移，仅重建应用容器，保留数据库、生产环境文件及证书。

验证：独立 PostgreSQL 上新增接口测试覆盖 Cookie 去重、跨广告 UV、机器人排除、跨链接和跨账户隔离、历史 UTM 单列、日期边界、分页、非法输入及登录保护；后端测试和静态检查、前端 30 项测试与生产构建通过。线上「测试-1」按上海时区 2026-09-08—2026-09-14 查询返回 63 次访问、59 位访客、手动 1 次、自动 60 次，与原数据总览和独立数据库查询一致。线上验证只读取现有访问数据，没有制造咨询或回传测试事件。

## 2026-09-15 统计接口改用 POST

当前接口为 `POST /api/v1/links/:id/stats`，前端统一通过 JSON 请求体提交筛选条件，旧 GET 入口不再返回统计。页面地址中的筛选参数仍用于刷新与历史导航，接口只读取 JSON，不合并 URL 查询参数。原统计口径、Cookie 去重及返回结构保持不变。

请求需带后台登录 Cookie，以及 `Content-Type: application/json`、`X-Requested-With: XMLHttpRequest`。示例：

```json
{
  "start": "2026-09-09",
  "end": "2026-09-15",
  "tz": "Asia/Shanghai",
  "ad_id": "",
  "page": 1,
  "sort": "visits",
  "order": "desc"
}
```

`page` 必须为数字。省略日期、时区时沿用最近七天和系统时区；省略页码、排序时使用第 1 页和访问数倒序。请求体必须为单个 JSON 对象；空请求体、null、字段类型错误、无效日期和非法排序返回 400，未登录返回 401，缺少请求校验头返回 403。

后端测试与静态检查、前端 30 项测试和生产构建通过。发布备份为 `/opt/linkscope-release/stats-post-20260915/`，回滚镜像为 `linkscope-lu81:before-stats-post-20260915`。只更新应用源码及镜像，不修改生产数据、环境文件或证书。

线上验收：已登录 POST 返回 200、未登录 POST 返回 401、非法页码返回 400、原 GET 返回 404。按上海时区 2026-09-09—2026-09-15 查询「测试-1」仍为访问 63、访客 59、手动 1、自动 60，与数据总览一致；浏览器统计页面已正常加载新接口结果。

## 2026-09-15 落地页 Meta 参数链路

入口 `GET /:code` 继续直接记录 URL 中的 Meta 参数，是广告归因的权威数据。落地页随后发起的 `/view`、手动 `/contact` 和倒计时 `/contact` 会镜像同一组 `X-Meta-*` 请求头，方便在请求日志中串联访问和咨询。中文名称按 UTF-8 百分号编码传输，日志详情自动还原；`fbcli` 作为常见误写兼容并规范为 `fbclid`。自定义请求头可以由客户端伪造，因此不参与归因覆盖。

短链接广告统计增加 `meta_visits` 和 `meta_unique_visitors`。前者统计入口携带至少一个已解析 Meta 白名单参数的正常落地页访问，后者在同一范围内按本站访客 Cookie 去重。`visits` 和 `unique_visitors` 继续表示全部正常落地页访问；`/view` 与 `/contact` 只更新原访问记录，不会增加访问次数。只有 Meta 参数但缺少 `ad_id`、`utm_content` 的流量单列显示。

浏览器咨询改为 Fetch：服务端验证签名访问票据并更新原访问后返回 WhatsApp 目标，页面再执行跳转；原生表单仍作为失败备用。验收链接使用真实浏览器和两组访客 Cookie 得到总访问 3、Meta 参数访问 3、全部去重访客 2、Meta 去重访客 2、手动咨询 1、自动跳转 1。`/view`、手动 `/contact`、自动 `/contact` 的日志均验证了广告系列、广告组、广告名称、广告 ID 与 fbclid 请求头。

发布备份位于 `/opt/linkscope-release/landing-meta-attribution-20260915/`，回滚镜像为 `linkscope-lu81:before-landing-meta-attribution-20260915`。本次没有数据库迁移，只重建应用容器；生产数据库、环境变量、Meta 凭证和 SSL 证书均保留。

## 2026-09-15 真实广告点击口径

短链接广告统计现统一采用真实广告点击口径：入口必须同时携带已展开且非空的 `fbclid` 和 `ad_id`，事件类型为落地页访问，并被系统识别为正常访客。Facebook 预览机器人、预取、可疑访问、普通帖子点击、只有 UTM 参数、只有 `fbclid` 或缺少广告 ID 的记录均不进入该页面的汇总和广告表现表。

页面只展示真实广告点击、按本站 Cookie 去重的真实广告访客、手动咨询和自动跳转；已移除 Meta 参数访问、Meta 去重访客及非广告来源行。手动咨询和自动跳转也只汇总到对应的真实广告点击。新入口参数写入前清理首尾空格，查询时同时兼容清理历史记录中的空格，因此同一广告不会因 Meta 网址参数格式产生多个分组。

本次已发布到 `meta.lu81.com`，没有数据库迁移，也没有创建测试访问或 Meta 回传。线上「测试-1」按上海时区 2026-09-09—2026-09-15 返回真实广告点击 104、去重访客 79、手动咨询 5、自动跳转 0，并只形成 1 个广告 ID 分组；独立数据库查询得到相同结果。相同范围内的 463 条机器人入口和不完整广告参数记录均未进入该统计。

公网健康检查、未登录 401、登录后统计接口 200、新页面静态资源及长期缓存均已验证。数据库和原应用文件保存在 `/opt/linkscope-release/real-ad-stats-before-20260915-1800/`，原镜像标记为 `linkscope-lu81:before-real-ad-stats-20260915`。

## 2026-09-15 统计日期快捷按钮

短链接广告统计的日期筛选新增「今天」和「近 3 天」两个可见按钮。「近 3 天」包含今天，两种快捷范围均按照页面当前选择的统计时区计算；点击按钮后立即更新路由筛选并查询，当前范围匹配时按钮显示选中状态。日期选择器、广告 ID、排序和分页功能保持原有行为。

前端 30 项测试、格式检查和生产构建通过，公网健康检查及新版带哈希静态资源返回 200。此次只重建无状态应用容器，没有修改数据库、Meta 配置、证书或统计口径。回退文件位于 `/opt/linkscope-release/date-presets-before-20260915-1815/`，回退镜像为 `linkscope-lu81:before-date-presets-20260915`。
