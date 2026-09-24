# TikTok 小说网页事件与广告归因设计

**日期：** 2026-09-24  
**状态：** 已完成业务设计确认，等待书面规格复查  
**适用项目：** `whatsapp-analytics` 的小说 H5、Go 后端与 Vue 运营后台

## 1. 目标

在现有小说投放短链、匿名访客、可见停留时长和 Meta 回传能力之外，增加一套独立的 TikTok 网页事件与广告归因能力。

系统需要做到：

- 每条小说投放链接只选择一个广告平台：Meta 或 TikTok。
- TikTok Pixel、Events API Access Token 集中管理，链接只选择已配置的 TikTok Pixel。
- TikTok 链接立即加载 Pixel，并通过 Pixel 与 Events API 双通道回传关键阅读事件。
- 广告参数使用动态归因，首次访问时冻结，运营人员不手填广告系列、广告组或广告 ID。
- 用户达到链接配置的前台可见阅读时长后，才产生主转化事件。
- 不影响现有普通短链接、语音小说、Meta 投放链接、多语言小说内容和内部统计。

成功标准是：运营人员可以为同一本小说创建多条独立 TikTok 投放短链；每条短链独立统计访问、开始阅读、达标阅读和停留时长；TikTok Events Manager 能收到可去重的浏览器与服务端事件；系统能清楚区分“本系统已保存”“TikTok API 已接收”和“TikTok 最终归因”。

## 2. 非目标

- 不增加投手账号、数据权限隔离或跨链接投手汇总。
- 不把 Meta 和 TikTok 重构成通用广告 Provider 框架。
- 不支持同一条小说链接同时启用 Meta 与 TikTok。
- 不接入 TikTok Marketing API 拉取广告名称、消耗或平台归因结果。
- 不实现支付、注册、邮箱或手机号匹配；小说站仍是免费匿名阅读站点。
- 本阶段不新增 Cookie 同意弹窗。TikTok 链接进入后立即加载 Pixel，与当前 Meta 行为保持一致；合规同意管理应作为独立项目处理。

## 3. 已确认业务决策

1. 小说投放链接的平台为 `meta` 或 `tiktok`，二选一。
2. TikTok 凭证和 Pixel 在运营后台集中管理。
3. TikTok 采用动态归因，不显示手动渠道和广告 ID 输入项。
4. TikTok Pixel 在 TikTok 链接打开后立即加载。
5. 主转化定义为：用户前台可见阅读达到该链接配置的停留阈值，默认 10 秒。
6. 主转化使用 TikTok 标准事件 `ViewContent`；`PageView` 和 `StartReading` 只用于漏斗观察。
7. Pixel 与 Events API 对相同业务事件使用相同事件名和 `event_id`。
8. Access Token 只在服务端使用，不能进入 H5 启动数据或前端日志。

## 4. 架构边界

### 4.1 保留现有模块

现有 `meta` 模块、小说入口 `/novel/{code}`、签名访问票据、`click_events` 访问记录、可见时间上报和小说统计继续作为现有能力使用。Meta 代码不迁移到通用框架，避免扩大回归范围。

### 4.2 新增 TikTok 模块

后端新增独立 `backend/internal/modules/tiktok` 模块，职责包括：

- TikTok 凭证加密保存和生命周期管理。
- TikTok Pixel CRUD、凭证关联、启用状态和测试事件码管理。
- TikTok 事件持久化、幂等、查询和人工重试。
- Events API HTTP 客户端。
- 使用数据库租约领取事件的发送 Worker。

实现形式参考现有 `backend/internal/modules/meta` 的可靠队列和加密方式，但只复用成熟模式，不强制共享复杂抽象。

### 4.3 链接平台选择

小说投放链接增加 `ad_platform`：

- `meta`：必须绑定有效 `meta_pixel_id`，`tiktok_pixel_id` 必须为空。
- `tiktok`：必须绑定有效 `tiktok_pixel_id`，`meta_pixel_id` 必须为空。

现有小说链接迁移后默认保持 `meta`。一条链接产生首条正常访问后，平台、小说和 Pixel 一并锁定；更换平台必须新建链接。无访问记录的链接允许修改。

## 5. 数据设计

### 5.1 TikTok 凭证

`tiktok_connections` 保存：名称、加密 Access Token、启用状态、创建和更新时间。凭证被 Pixel 引用或产生事件后不能物理删除，只能停用。

### 5.2 TikTok Pixel

`tiktok_pixels` 保存：名称、Pixel Code、关联凭证、可选测试事件码、启用状态、最后测试时间、最后测试结果、创建和更新时间。Pixel Code 全局唯一。Pixel 被链接或事件引用后不能物理删除，只能停用。

可发送性测试必须在 Pixel 页面执行，因为只有 Access Token 与 Pixel Code 组合后才能验证完整的 Events API 配置。凭证页面不提供会造成误导的独立“测试成功”状态。

测试事件码只用于联调和上线验收；正式投放前必须清空，避免正式访问进入测试事件。

### 5.3 小说链接

小说投放链接增加 `ad_platform` 和 `tiktok_pixel_id`。数据库和服务层共同保证 Meta/TikTok 二选一约束，不能只依赖前端表单。

### 5.4 访问归因快照

TikTok 访问需要在 `click_events` 中冻结：

- `ad_platform`
- TikTok Pixel 引用和 Pixel Code 快照
- `ttclid`
- `_ttp`
- `campaign_id`
- `adgroup_id`
- `creative_id`
- `ad_id_v2`
- `placement`
- `utm_source`、`utm_medium`
- 落地页、来源页、真实客户端 IP 和 User-Agent
- `tiktok_start_reading_at`
- `tiktok_view_content_at`

`ttclid` 在最初 `GET /novel/{code}` 时从 URL 获取。`_ttp` 由 H5 在 Pixel 加载后读取第一方 Cookie，通过已签名的 `/view` 或后续 `/reading-time` 请求补充；只允许在原值为空时补充，不能覆盖已冻结值。

广告 ID 使用字符串保存，避免大整数精度问题。原始 `ttclid` 和 `_ttp` 不做哈希；后台展示时脱敏。未展开的 `__...__`、`{{...}}` 宏和空白值不保存为真实归因字段。

### 5.5 TikTok 事件队列

`tiktok_events` 保存：访问、链接、小说、Pixel、事件名、事件 ID、原始事件时间、加密请求数据、状态、发送次数、下次发送时间、租约信息、HTTP 状态、TikTok 业务码、请求 ID、简化错误信息和时间戳。

唯一约束为：

```text
Pixel Code + event_name + event_id
```

状态至少包括 `pending`、`sending`、`accepted`、`retry` 和 `failed`。`accepted` 只表示 TikTok API 接收，不表示 TikTok 已将事件归因给某条广告。

## 6. 动态归因

运营后台为 TikTok 链接生成投放地址模板：

```text
/novel/{code}?utm_source=tiktok&utm_medium=paid_social&campaign_id=__CAMPAIGN_ID__&adgroup_id=__AID__&creative_id=__CID__&ad_id_v2=__ADID_V2__&placement=__PLACEMENT__
```

`ttclid` 由 TikTok 实际广告点击附加，系统不能伪造。`__CID__` 按原始素材标识保存，不擅自当作所有投放类型下统一的广告 ID。Smart+ 才可能提供 `__ADID_V2__`；缺失时保持为空。

首次正常小说访问冻结投放链接、小说、Pixel 和归因参数。后续章节切换、语言切换、SPA 路由变化或浏览其他小说都继续归属于最初入口访问，不创建新的入口访问记录，也不覆盖归因。

本系统统计的是内部入口来源，不能还原 TikTok 的浏览归因、跨设备匹配或模型归因。TikTok 最终归因只能在 TikTok Ads Manager 中查看。

## 7. 事件契约

### 7.1 PageView

- TikTok 链接打开后立即加载官方 Pixel 基础代码并产生浏览器 `PageView`。
- 后端创建现有小说访问记录，作为内部访问量、UV 和停留统计事实。
- `PageView` 不作为本项目主转化，不额外制造一条可能与 Pixel 自动 PageView 重复的 Events API 事件。

### 7.2 StartReading

- 用户点击 `Start Reading` 且成功进入第一章时触发一次。
- 事件名为自定义事件 `StartReading`。
- 浏览器 Pixel 与 Events API 双路发送，使用同一个确定性事件 ID：`novel_{visit_id}_start`。
- 自定义事件用于漏斗报告和受众，不作为主优化事件。

### 7.3 ViewContent

- 用户前台可见阅读时间达到链接冻结的阈值后触发一次，默认 10 秒。
- 使用 TikTok 标准事件 `ViewContent` 作为主转化，以支持网页广告优化。
- 确定性事件 ID 为 `novel_{visit_id}_qualified`。
- 后端先校验累计秒数单调增长、服务器观察到的会话时长、访问 surface、签名票据和两小时上限。
- 达标后，在同一数据库事务中设置 `tiktok_view_content_at` 并插入 `tiktok_events`。
- H5 收到服务端“达标且事件已保存”的响应后，再调用 Pixel 发送同一事件 ID。
- 重复、倒退、提前或伪造超长上报不能生成第二条事件。

`StartReading` 和 `ViewContent` 的事件数据可以携带小说内容标识，例如 `contents: [{content_id: "novel:<id>", quantity: 1}]`，但不发送虚构金额、币种、邮箱、手机号或购买信息。正式请求字段必须通过 TikTok Payload Helper 验证。

## 8. H5 行为

小说启动数据按平台互斥返回公开配置：

- Meta 链接只返回并加载 Meta Pixel 配置。
- TikTok 链接只返回 `pixel_code`、事件 ID、链接阈值和必要开关。
- 普通非投放页面不加载广告 Pixel。

新增简单的 `novel-h5/src/lib/tiktok.js`，只负责：

- 按 TikTok 官方基础代码初始化一次 `ttq`。
- 防止 SPA 路由变化重复加载脚本。
- 在 SDK 未就绪时短暂排队事件；初始化完成后按原事件 ID 发送。
- SDK 加载或调用异常时安静失败，不影响页面和阅读流程。
- 读取 `_ttp` 后通过现有签名业务请求补充服务端访问记录。

现有可见时间算法保持：页面可见时累计，进入后台暂停，章节和语言切换不重置。TikTok 只消费经过服务端确认的达标结果，不另写第二套计时器。

H5 的 Content Security Policy 仅加入 TikTok 官方 Pixel 脚本、图片和请求域名，并保留现有 Meta 白名单；不增加无关的宽泛通配规则。

## 9. 后端接口

新增运营后台接口：

- `GET/POST /api/v1/tiktok-connections`
- `PATCH/DELETE /api/v1/tiktok-connections/:id`
- `GET/POST /api/v1/tiktok-pixels`
- `PATCH/DELETE /api/v1/tiktok-pixels/:id`
- `POST /api/v1/tiktok-pixels/:id/test`
- `GET /api/v1/tiktok-events`
- `POST /api/v1/tiktok-events/:id/retry`

扩展现有小说链接接口以接收 `ad_platform` 和相应 Pixel ID。扩展现有小说 `/view`、章节进入或阅读时间流程完成 `_ttp` 补充、`StartReading` 幂等标记和达标 `ViewContent` 入队。所有公共写请求继续使用访问票据、同源检查、surface 校验、字段长度限制和限流。

## 10. Events API Worker

Worker 使用现有数据库租约和 fencing token 模式领取事件，避免多实例重复发送。第一期每次发送一条事件，便于精确记录响应状态。

发送规则：

- 固定调用 TikTok 官方 Events API Web 事件端点。
- 请求头使用服务端解密得到的 `Access-Token`。
- 请求包含 `event_source=web`、`event_source_id=Pixel Code`、事件名、事件时间、事件 ID、真实 `ttclid`、`ttp`、客户端 IP、User-Agent 和页面 URL。
- 没有 `ttclid` 时仍可发送合法自然事件，但系统不能声明已广告归因。
- HTTP 成功且 TikTok 业务码成功才标记 `accepted`。
- 网络超时、限流、HTTP 5xx 和可恢复业务错误进入指数退避，并遵守 `Retry-After`。
- 参数错误、无效 Pixel、凭证失效等不可恢复错误标记 `failed`。
- 重试始终保留原 `event_id` 和事件时间。
- 自动重试最长 24 小时，避免跨过 TikTok 48 小时去重窗口后再次发送造成重复风险。
- HTTP 客户端设置超时、响应体大小限制，并禁止跨域重定向，防止 Access Token 泄露。

凭证、完整请求数据、`ttclid`、`ttp`、IP 和 User-Agent 不进入普通日志。事件请求数据沿用项目加密机制保存，后台只展示必要的脱敏诊断信息。

## 11. 运营后台

新增并列菜单：

- TikTok 管理
  - TikTok Pixel
  - TikTok 凭证
  - TikTok 事件记录

小说投放链接表单只展示：链接名称、自定义短码、绑定小说、广告平台、对应 Pixel 和合格阅读时长。归因方式固定为动态归因；渠道、广告 ID、广告系列 ID 和广告组 ID 不显示。

链接保存后提供“复制普通短链”和“复制 TikTok 投放地址模板”。普通短链用于测试或自然访问；投放模板包含官方动态宏。

TikTok 事件记录显示事件名、链接、小说、Pixel、事件时间、发送状态、重试次数、TikTok 请求 ID 和简化错误。只有仍在安全重试时间内的失败事件允许人工重试。

## 12. 统计

小说链接统计保留现有访问次数、匿名访客数、平均可见停留时长、总可见停留时长和访问明细，并增加：

- 开始阅读人数和次数。
- 达标阅读人数和次数。
- 达标阅读率。
- TikTok API 待发送、已接收和失败事件数。

访问明细增加广告平台、Pixel、Campaign ID、Ad Group ID、Creative ID、脱敏 `ttclid`、开始阅读状态、达标状态和服务端事件状态。支持日期、时区、广告参数和事件状态筛选。

统计页必须使用以下文案边界：

- “已保存”：事件已可靠写入本系统数据库。
- “TikTok 已接收”：Events API 返回成功。
- “TikTok 是否归因”：本系统未知，请到 TikTok Ads Manager 查看。

## 13. 异常和降级

- Pixel SDK 加载失败不影响小说页面、章节和内部统计。
- Events API 失败不阻塞用户阅读，由数据库队列重试。
- TikTok Pixel 或凭证停用后，已有链接继续展示小说和采集内部统计，但暂停 TikTok 回传并在后台显示配置异常。
- TikTok 总开关关闭时不加载 Pixel、不启动 Worker；Meta 链路不受影响。
- H5 发送失败后可以在同一次页面会话内有界重试，但必须复用原事件对象和事件 ID。
- 服务端重复请求返回幂等成功，不新建第二条事件。

## 14. 迁移策略

迁移全部为增量操作：

1. 新建 TikTok 凭证、Pixel 和事件表。
2. 为小说链接和访问记录增加 TikTok 字段、索引和条件约束。
3. 将已有小说投放链接标记为 `meta`，保留其 Meta Pixel 和历史数据。
4. 部署后端管理接口和 Worker，但默认关闭 TikTok 总开关。
5. 部署运营后台和 H5。
6. 配置测试凭证、Pixel 和测试事件码，完成联调后再打开正式开关。

迁移不得删除或改写既有 Meta 事件、访问归因和小说内容。

## 15. 测试计划

### 15.1 数据与接口

- 旧 Meta 小说链接迁移后继续可用。
- Meta/TikTok 二选一约束和对应 Pixel 必填。
- 有访问后拒绝更换平台、小说或 Pixel。
- 凭证加密、Pixel Code 唯一性、停用和引用删除规则。
- API 鉴权、校验、分页、筛选和脱敏。

### 15.2 归因

- 正确解析 TikTok 动态宏。
- 丢弃未展开宏、空白和超长值。
- 首次访问冻结，章节、语言和 SPA 导航不覆盖。
- `_ttp` 只补充空值，不能跨访问覆盖。
- 无 `ttclid` 的自然访问正常记录但不标记广告归因。

### 15.3 事件与时间

- `StartReading` 只在成功进入第一章后生成一次。
- 页面前台累计、后台暂停、章节切换连续。
- 达到阈值才生成 `ViewContent`。
- 提前、倒退、重复、错误票据和伪造超长时长不生成重复事件。
- Pixel 与 Events API 使用一致事件名和事件 ID。

### 15.4 Worker

- TikTok 成功响应、HTTP 200 业务失败、4xx、429、5xx、超时和无效 JSON。
- 租约超时、多 Worker 竞争、进程重启和 fencing token。
- 指数退避、`Retry-After`、24 小时截止和人工重试限制。
- 日志不包含 Access Token 或完整匹配数据。

### 15.5 前端与回归

- Meta 链接只加载 Meta Pixel，TikTok 链接只加载 TikTok Pixel。
- TikTok SDK 只初始化一次，加载失败不影响阅读。
- 首页、简介页、章节页、目录、语言切换和移动端布局正常。
- 普通短链接、语音小说、Meta 回传和现有统计回归通过。

## 16. 上线验收

1. 在 TikTok Events Manager 创建 Web 数据源并选择 Pixel + Events API。
2. 在运营后台录入测试凭证、Pixel Code 和 Test Event Code。
3. 创建一条专用 TikTok 测试小说短链，并带动态宏示例参数访问。
4. 使用 TikTok Pixel Helper 验证 Pixel 加载、PageView、StartReading 和 ViewContent。
5. 使用 TikTok Test Events 与 Payload Helper 验证服务端字段和响应。
6. 核对 Pixel 与 Events API 的事件名和事件 ID，并确认 TikTok 去重诊断正常。
7. 验证后台内部访问、阅读、达标、事件状态和广告参数统计。
8. 清空 Test Event Code，创建正式链接并进行小流量投放。
9. 观察至少一个完整归因窗口，再对比内部来源统计和 TikTok Ads Manager 报告；差异按两套归因口径解释，不能强行对齐。

## 17. 验收标准

- 运营人员可以集中管理 TikTok 凭证和 Pixel。
- 小说链接可以选择 Meta 或 TikTok，不能同时选择。
- TikTok 投放链接能自动生成动态宏地址。
- TikTok 链接立即加载 Pixel，但 SDK 或 API 异常不影响阅读。
- 用户未达到阈值时不发送主转化；达到阈值后只产生一次 `ViewContent`。
- Pixel 与 Events API 事件正确去重。
- 多条链接绑定同一本小说时，访问、停留和 TikTok 事件按链接隔离。
- 后台可追踪事件保存、发送、重试和失败状态，不把 API 接收误称为广告归因。
- 现有 Meta、短链接、语音小说和多语言小说功能通过回归测试。

## 18. 官方参考

- [TikTok Events API](https://ads.tiktok.com/resources/help/article/events-api?redirected=1)
- [TikTok 事件去重](https://ads.tiktok.com/help/article/event-deduplication?lang=en)
- [TikTok 标准事件与参数](https://ads.tiktok.com/help/article/standard-events-parameters?lang=zh)
- [TikTok Pixel Cookie](https://ads.tiktok.com/help/article/using-cookies-with-tiktok-pixel)
- [TikTok 动态 UTM 宏](https://ads.tiktok.com/resources/help/article/track-offsite-web-events-with-utm-parameters?lang=zh)
- [TikTok Payload Helper](https://business-api.tiktok.com/payload_helper/)
