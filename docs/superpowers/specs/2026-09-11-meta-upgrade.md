# Meta 广告管理升级

用户确认本次只接入 Facebook / Meta。沿用 Vue 3 + JavaScript + Element Plus、Go/Gin、PostgreSQL，同一 Go 服务。不得提交 Git。

## 业务与兼容
- 保留 meta_connections 作为广告账户和 Insights 同步单位，一个账户同步一次。新增 meta_pixels 管理账户下多个 Pixel。默认一个短链接选择一个 Pixel；没有指定时沿用账户默认 Pixel。已有连接、链接、待发事件迁移保留原归属。
- 账户保留旧 pixel_id/capi_token 字段作为旧 API 兼容默认目标；新的页面分别管理账户与 Pixel。账户 ID 不可修改；Pixel 外部编号和所属账户创建后不可修改。
- 回传规则属于 Pixel：pageview_enabled 默认 false；manual_enabled 默认 true；manual_event_name 可选 WhatsAppConsultClick 或 Contact。自动跳转不回传、不冒充咨询。没有注册、订单来源时不生成注册或购买事件。
- 访问时固化目标 Pixel、规则和来源。落地页挂载后的 signed-ticket POST /:code/view 产生 PageView；咨询 POST /:code/contact 产生手动事件。每访问/动作最多一次，默认 Pixel 老咨询 event_id 保持兼容。规则变更不改写旧访问；账户/目标停用立即暂停发送。
- 不加载浏览器 Pixel SDK，不读取第三方站点登录 Cookie。仅发送现有有效匹配数据。新目标默认停用；浏览事件默认关闭。每条短链接的咨询和自动统计保持原口径。
- 统一显式 campaign_id/adset_id/ad_id 与名称、版位、来源字段；兼容旧 utm_content 仅显式开启。增加参数生成、检查和来源诊断；参数不能携带访问 Token，不以 URL 参数确认账户归属。
- 凭证分读取/回传，写入加密、返回只给状态；可设置人工记录的到期时间，标注并非平台保证。更换凭证使先前验证失效；测试接收成功更新对应凭证状态，错误必须脱敏。提供审计记录与密钥轮换能力。
- 现有 Insights 报表只按账户汇总，绝不按 Pixel 重复累加花费。新 PageView 不改变站内访问口径；回传记录可按事件/Pixel 筛选。

## 接口约定
现有 /api/v1/meta/connections 兼容。允许账户创建时 pixel_id 留空；capi_enabled=true 需存在可用 Pixel。新增：
- GET /meta/pixels?connection_id= -> Pixel[]
- POST /meta/pixels；PATCH /meta/pixels/:id
Pixel {id,connection_id,name,pixel_id,enabled,pageview_enabled,manual_enabled,manual_event_name,has_capi_token,token_expires_at,credential_status,validated_at,last_error,updated_at}；写入 capi_token/clear_capi_token；新增默认 enabled=false, pageview_enabled=false,manual_enabled=true,manual_event_name=WhatsAppConsultClick。内部 ID 与外部 pixel_id 区分。
- POST /meta/pixels/:id/test-event {test_event_code,event_name} -> event；event_name 仅 PageView/WhatsAppConsultClick/Contact。测试不进入站内统计。
- GET /meta/credentials -> {items:[{kind,connection_id,pixel_record_id,name,account_id,pixel_id,configured,status,expires_at,validated_at,last_error}],encryption_key_id}
- GET /meta/audit -> {items:[{id,actor,action,detail,created_at}]} 最近 100 条，脱敏。
- POST /meta/credentials/rewrap -> {updated,encryption_key_id}；仅服务端已配置新活动密钥时重加密。
- POST /meta/source/inspect {url} -> {valid,parameters,issues:[string],source,campaign_id,adset_id,ad_id}，仅解析，不访问网址，不返回 Token。
- links API 新增 meta_pixel_id(number|null)，空值选择账户默认目标，跨账户组合拒绝。
- events 返回新增 pixel_id,pixel_record_id；筛选 event_name,pixel_record_id。
- connections 返回 read_token_expires_at,read_credential_status,read_token_updated_at；写入 read_token_expires_at(string|null)。兼容旧回传字段。

## 验收
使用独立可销毁的 PostgreSQL 测试库；模拟 Graph 网络边界验证协议。验证迁移保留、跨账户绑定拒绝、目标停用、访问规则快照、浏览/咨询/自动分流、稳定重试、凭证脱敏和轮换、Insights 花费不重复。构建并测试两个 Vue 应用；浏览器核对配置、短链接选择、事件列表和来源工具。真实 Meta 接收需用户提供授权凭证，不把模拟测试冒充真实接入。
