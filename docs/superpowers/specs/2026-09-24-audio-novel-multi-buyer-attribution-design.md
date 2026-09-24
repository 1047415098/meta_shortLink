# 语音小说多投手短链、统计与广告回传设计

**日期：** 2026-09-24  
**状态：** 用户已批准，进入实施计划阶段  
**适用项目：** `whatsapp-analytics` 的语音小说 H5、Go 后端与 Vue 运营后台

## 1. 目标

在现有语音小说内容、Podcast 播放、普通短链、Meta 和 TikTok 配置基础上，为每部语音小说增加独立的多投手投放能力。

系统需要做到：

- 一部语音小说只维护一份内容，可以创建任意多条投放短链。
- 每条短链代表一个投手或一次投放，访问、匿名访客、页面停留、实际播放和广告事件互相隔离。
- 访问 `/audio-novel/{code}` 后直接进入该链接绑定的语音小说详情和播放器，不先经过公共首页。
- 每条链接只能选择 Meta 或 TikTok 其中一个平台，并绑定对应 Pixel。
- `PageView`、`StartListening` 和达到播放门槛后的 `ViewContent` 按平台回传。
- 播放完成只作为内部分析指标，不额外发送给广告平台。
- 旧语音短链、公共语音首页、文字小说、普通短链接和 WhatsApp 行为继续兼容。

成功标准是：多个投手可以推广同一部语音小说并分别查看真实播放漏斗；浏览器 Pixel 与服务端回传可以使用相同事件 ID 去重；运营后台能清楚区分内部行为、平台 API 接收状态和最终广告归因。

## 2. 非目标

- 不增加投手账号、角色或数据权限隔离。
- 不做跨短链的投手汇总报表。
- 不允许一条链接同时选择 Meta 和 TikTok；双平台投放需要分别创建两条链接。
- 不重构 Meta 和 TikTok 为复杂的通用 Provider 框架。
- 不新增支付、登录、会员或音频章节锁。
- 不为旧语音短链补算历史播放时长或完成率。
- 不把 Meta/TikTok API 接收成功表述为广告已经归因。

## 3. 已确认业务决策

1. 新式语音投放链接绑定一部具体语音小说。
2. 用户打开投放链接后直接进入绑定内容的详情和播放器。
3. 达标回传按音频实际播放时长计算，不按页面可见时长计算。
4. 锁屏或切到后台后音频仍在播放时继续累计；暂停、结束、缓冲和拖动期间不累计。
5. 快进或拖动进度条不能直接增加实际播放秒数。
6. 每条链接的达标播放时长可配置，默认 10 秒，允许 1 至 3600 秒；0 表示关闭达标事件。
7. 保留 `StartListening`，用于区分页面访问和真正开始收听。
8. Meta 与 TikTok 的达标事件都使用标准事件 `ViewContent`。
9. 自然播放完成仅用于后台完成率统计，不发送广告事件。
10. 归因方式固定为动态归因，不在语音投放表单展示渠道、广告 ID、广告系列 ID 或广告组 ID。

## 4. 架构边界

### 4.1 复用能力

继续复用以下成熟能力：

- `short_links` 的全局唯一短码、启停状态和首次正常访问锁定规则。
- `click_events` 的匿名访客、访问分类、设备、地区、来源和广告参数快照。
- Meta 账户、Pixel、凭证和事件队列。
- TikTok 凭证、Pixel、动态宏、加密访问上下文、事件队列和发送 Worker。
- 现有访问票据、同源校验、Cookie 策略、日志脱敏和重试窗口。

本项目只扩展语音小说场景，不为复用而改造成新的广告平台框架。文字小说和语音小说保留各自清晰的业务处理代码。

### 4.2 语音小说职责

`audionovel` 模块负责：

- 语音投放链接的创建、编辑、删除和统计。
- 绑定内容可用性校验。
- `StartListening`、累计播放和播放完成接口。
- Meta/TikTok 语音事件的幂等业务状态。

`tracking` 模块继续负责首次入口访问、广告参数解析和冻结。`audio-novel` Vue 工程负责播放器状态采集和浏览器 Pixel 事件。广告发送仍由现有 Meta/TikTok 模块负责。

## 5. 数据设计

### 5.1 语音投放链接

`short_links` 增加可空字段：

- `audio_novel_id bigint REFERENCES audio_novels(id)`

产品绑定约束调整为：

- `product_type='novel'`：`novel_id` 必填，`audio_novel_id` 为空。
- `product_type='audio_novel'`：`audio_novel_id` 必填，`novel_id` 为空。
- 其他兼容类型：两者均为空。

语音投放链接还必须满足广告平台约束：

- `ad_platform='meta'`：Meta Pixel 和所属账户必填，TikTok Pixel 为空。
- `ad_platform='tiktok'`：TikTok Pixel 必填，Meta Pixel 和账户为空。

链接第一次产生正常 `GET` 访问后，冻结 `audio_novel_id`、`ad_platform` 和 Pixel。无访问记录时允许修改；有访问记录后只能修改名称、启停状态和播放门槛等不会改变历史归因含义的字段。无历史访问时允许删除；有历史访问时只能停用。

### 5.2 音频总时长

`audio_novels` 增加：

- `audio_duration_seconds integer NOT NULL DEFAULT 0`

新上传 MP3 时，运营后台在读取本地媒体元数据后同时提交格式化时长和整数秒数。服务端校验其范围和一致性。迁移尝试从现有 `audio_duration` 的 `MM:SS` 或 `HH:MM:SS` 文本回填整数秒数；无法识别时保留 0。

总时长为 0 的旧音频仍可播放、统计开始播放和累计播放时长，但不计算播放完成。

### 5.3 访问播放状态

`click_events` 增加：

- `audio_novel_id`
- `playback_seconds`：真实处于播放状态的累计墙钟秒数。
- `media_consumed_seconds`：按合法播放速率折算的累计媒体消费秒数，仅用于辅助完成判断。
- `playback_updated_at`
- `audio_started_at`
- `audio_qualified_at`
- `audio_completed_at`

首次入口访问冻结 `audio_novel_id`、平台、Pixel、门槛和广告参数。SPA 内切换路由或查看其他内容不能覆盖该快照，也不会新增入口访问。

### 5.4 TikTok 事件关联

`tiktok_events` 增加可空的 `audio_novel_id`，语音事件使用它关联内容。现有文字小说事件继续使用 `novel_id`。数据库约束保证一个事件不会同时绑定文字小说和语音小说。

语音事件 ID 使用稳定前缀：

- PageView：`audio_{visit_id}_view`
- StartListening：`audio_{visit_id}_start`
- ViewContent：`audio_{visit_id}_qualified`

Meta 与 TikTok 的浏览器/服务端双通道对同一业务事件复用同一事件 ID。

## 6. 路由与入口访问

新增语音投放链接仍使用现有入口：

```text
/audio-novel/{code}
```

处理流程：

1. 后端验证短码、链接状态、产品类型、绑定内容、MP3 和内容启用状态。
2. 正常 `GET` 首次访问冻结链接绑定和广告配置。
3. 创建一条 `surface='audio_novel'` 的入口访问记录。
4. 启动数据返回绑定的语音小说 slug、签名票据、平台公开配置和播放门槛。
5. Vue Router 在同一文档内 `replace` 到 `/audio-novel/{code}/audio/{slug}`。
6. 页面直接显示绑定内容和播放器，不产生第二次入口访问。

绑定内容被停用、软删除或移除 MP3 时，投放链接返回不可用状态；重新启用内容或上传 MP3 后自动恢复。旧 `legacy` 短码继续打开公共语音首页，不强制绑定内容。

## 7. 播放计时规则

### 7.1 客户端计时

播放器不能使用 `currentTime` 的跳变量直接作为播放时长。H5 使用单调时钟维护累计状态：

- `playing`：开始或恢复计时。
- `pause`、`waiting`、`stalled`、`seeking`、`ended`：结算本段并暂停计时。
- `seeked`：仅当媒体仍应继续播放时重新开始计时。
- 页面隐藏、锁屏或切到后台不会自动暂停；只要音频仍在播放就继续累计。
- 拖动进度条只改变播放位置，不增加累计播放秒数。
- 播放速率限制在合理范围内参与 `media_consumed_seconds` 计算，但达标门槛始终使用真实墙钟播放秒数。

每 10 秒上报一次累计值，在 `pagehide` 时使用 `sendBeacon` 补报。网络失败后保留最后一次服务端确认值，在同一会话下一轮重试。

### 7.2 服务端校验

服务端只接受：

- 正确短码、正确 `audio_novel` surface 和有效签名票据。
- 当前访问冻结的同一语音小说。
- 单调递增的累计秒数。
- 不超过服务端观察到的访问存活时间和 24 小时单会话上限的值。
- `media_consumed_seconds` 不超过播放秒数乘以允许的最大播放速率。

重复、倒退、超长、跨短码、过期和篡改请求不会推进业务状态，也不会产生广告事件。

### 7.3 播放完成

客户端只在原生播放器触发 `ended` 后请求完成接口。服务端同时要求：

- 音频总秒数大于 0。
- 累计媒体消费秒数达到音频总时长的 90%。
- 当前访问已经记录开始播放。

播放完成按访问幂等记录一次。它是运营分析指标，不进入 Meta/TikTok 事件队列。

## 8. 广告事件契约

### 8.1 PageView

- 页面成功打开后只确认一次。
- Meta 沿用现有浏览器 Pixel 与服务端 CAPI 去重链路。
- TikTok 沿用现有小说站策略，由浏览器 Pixel 发送 PageView，不额外制造重复的 Events API PageView。
- 页面浏览不代表开始播放。

### 8.2 StartListening

- 原生播放器第一次触发真实 `playing` 后生成。
- 事件名为自定义事件 `StartListening`。
- 浏览器 Pixel 与 Meta CAPI/TikTok Events API 使用同一事件 ID。
- 服务端先幂等保存业务状态和队列事件，再把确定性事件 ID 返回 H5；H5 收到确认后发送浏览器事件。

### 8.3 ViewContent

- `playback_seconds` 达到链接冻结的门槛时生成一次。
- Meta 与 TikTok 都使用标准事件 `ViewContent`。
- 门槛默认为 10 秒；0 表示关闭此事件。
- 达标状态更新和服务端事件入队在同一事务完成。
- H5 只发送服务端确认过的浏览器事件，并复用服务端事件 ID。

事件可以携带 `content_id=audio_novel:{id}`、内容名称和内容类型，但不发送虚构金额、币种、邮箱、手机号或购买字段。

## 9. 动态归因

语音投放链接固定使用动态归因。运营后台不展示手动渠道、广告 ID、广告系列 ID 和广告组 ID。

Meta 投放地址继续使用现有规范化动态参数。TikTok 模板使用：

```text
/audio-novel/{code}?utm_source=tiktok&utm_medium=paid_social&campaign_id=__CAMPAIGN_ID__&adgroup_id=__AID__&creative_id=__CID__&ad_id_v2=__ADID_V2__&placement=__PLACEMENT__
```

`ttclid` 由真实 TikTok 广告点击附加，系统不能伪造。未展开宏、空值和超长值不保存。首次正常访问冻结归因参数；播放、暂停、后台播放和 SPA 导航都不能覆盖它们。

## 10. 后端接口

新增运营后台接口：

- `GET /api/v1/audio-novel-links`
- `POST /api/v1/audio-novel-links`
- `PATCH /api/v1/audio-novel-links/:id`
- `DELETE /api/v1/audio-novel-links/:id`
- `POST /api/v1/audio-novel-links/:id/stats`

新增公开行为接口：

- `POST /audio-novel/:code/start-listening`
- `POST /audio-novel/:code/playback-time`
- `POST /audio-novel/:code/complete`

现有 `/audio-novel/:code/view`、`/contact` 和 `/time-spent` 保留，供旧入口和现有 WhatsApp/页面停留行为使用。新式语音投放链接的主转化以实际播放门槛为准，不再用页面停留替代播放达标。

## 11. 运营后台

“语音小说管理”列表增加“投放链接”操作。新增路由：

- `/admin/audio-novels/:id/links`
- `/admin/audio-novel-links/:id/stats`

创建或编辑表单只展示：

- 链接名称
- 自定义短码，留空自动生成
- 绑定语音小说
- Meta / TikTok 二选一
- 对应 Pixel
- 达标播放时长，默认 10 秒
- 启用状态

链接列表展示短码、平台、Pixel、播放门槛、状态、访问量和首次访问时间，并提供复制普通链接、复制 TikTok 动态模板、查看统计、编辑、启停和删除操作。

## 12. 统计

语音投放链接统计仅包含当前链接的正常 `GET` 入口访问，提供：

- 访问次数和匿名独立访客数。
- 已采集页面平均/总可见时长。
- 开始播放人数和次数。
- 已采集平均/总实际播放时长。
- 达标播放人数、次数和达标率。
- 播放完成人数、次数和完成率。
- Meta/TikTok 待发送、已接收和失败事件数。

明细包含：访问时间、匿名访客标识、地区、设备、浏览器、来源、广告参数、页面可见时长、实际播放时长、开始/达标/完成状态和广告事件状态。

支持日期、报表时区、Meta 广告 ID、TikTok Campaign/Ad Group/Creative/Ad ID v2 和事件状态筛选。TikTok 点击标识只展示脱敏值。

统计页面必须区分：

- “已保存”：事件已进入本系统数据库。
- “平台已接收”：Meta/TikTok API 返回成功。
- “广告是否归因”：本系统未知，必须到对应广告平台查看。

## 13. 异常和降级

- Pixel SDK 加载失败不影响播放、下载、正文和内部统计。
- Events API/CAPI 失败不阻塞播放器，由现有可靠队列重试。
- Pixel 或凭证停用后继续采集内部播放统计，但暂停广告回传并在后台提示配置异常。
- TikTok 总开关关闭时不加载 TikTok Pixel、不发送 TikTok 服务端事件；Meta 链路不受影响。
- 播放上报失败时客户端在当前会话内有界重试，不创建新的入口访问。
- 内容不可用时返回明确的 404 或 410 页面，不创建错误的播放事件。
- 并发、重复和 Beacon 补报通过数据库条件更新保持幂等。

## 14. 迁移与上线

1. 增量增加 `audio_novel_id`、音频整数时长、播放状态字段和索引。
2. 回填可解析的旧音频时长，不改写旧访问与广告事件。
3. 部署后端接口和事件扩展，保持 TikTok 总开关现状。
4. 部署运营后台和语音 H5。
5. 为一部测试音频分别创建 Meta 和 TikTok 测试短链。
6. 使用 Meta Test Events、TikTok Test Events、Pixel Helper 和后台统计完成验收。
7. 清理测试事件码后再创建正式投放链接并小流量观察。

迁移和部署不得删除旧语音内容、MP3、访问记录或普通短链。

## 15. 测试计划

### 15.1 数据和链接 API

- 旧短链、普通短链和文字小说链接迁移后继续可用。
- `audio_novel` 必须绑定有效语音小说，并保证 Meta/TikTok 二选一。
- 自动短码和自定义短码保持全局唯一。
- 首次正常访问后拒绝更换内容、平台和 Pixel。
- 有访问记录时拒绝删除；无访问时允许删除。
- 内容停用、删除、移除音频和恢复后的可用性正确。

### 15.2 入口和归因

- 多个短码绑定同一语音小说时写入不同 `link_id`。
- 根地址只记录一次入口访问并直接打开绑定播放器。
- SPA 导航不产生第二次入口访问，也不覆盖归因。
- Meta/TikTok 动态宏、未展开宏、`ttclid` 和 `_ttp` 处理正确。

### 15.3 播放状态

- `playing`、暂停、恢复、缓冲、拖动、结束和重复播放。
- 后台或锁屏继续播放时累计，媒体暂停时停止。
- 周期上报和 `pagehide` Beacon 补报。
- 倒退、重复、错误票据、跨短码和伪造超长值。
- 播放门槛 0、默认 10 秒和边界 3600 秒。
- 播放完成要求自然结束和 90% 媒体消费量。

### 15.4 广告事件

- Meta 链接只加载 Meta，TikTok 链接只加载 TikTok。
- PageView、StartListening 和 ViewContent 的触发时机正确。
- 浏览器与服务端事件名和事件 ID 一致。
- StartListening 和 ViewContent 每个访问最多一次。
- 凭证停用、Pixel 停用、超时、限流、重试和人工重试。

### 15.5 统计和前端

- 访问、UV、页面时长、播放时长、达标率和完成率计算正确。
- 历史未采集数据明确显示“未采集”，不反推时长。
- 日期、时区、广告参数、事件状态和分页筛选正确。
- 后台桌面与移动端链接管理、复制和统计正常。
- iPhone、Android、Facebook/Instagram/TikTok 内置浏览器和桌面浏览器完成真机播放验证。

### 15.6 回归

- 旧语音公共首页、列表、正文、Podcast 页面和下载功能。
- WhatsApp 手动咨询、旧 PageView 和旧页面 TimeSpent。
- 普通短链接、文字小说、多语言小说和现有 Meta/TikTok 管理页面。

## 16. 验收标准

- 同一部语音小说可创建多条独立投放链接。
- 新链接直接打开绑定的播放器且只记录一次入口访问。
- 页面访问、开始播放、达标播放和播放完成可以按短链隔离统计。
- 后台或锁屏播放继续累计，暂停、缓冲和拖动不伪造时长。
- Meta/TikTok 每条链接严格二选一，并绑定有效 Pixel。
- PageView、StartListening 和 ViewContent 按确认规则回传并正确去重。
- 完成率只进入内部统计，不额外发送广告事件。
- 广告发送异常不影响播放，后台能查看接收、重试和失败状态。
- 旧语音入口、普通短链接、文字小说和 WhatsApp 链路通过回归测试。
