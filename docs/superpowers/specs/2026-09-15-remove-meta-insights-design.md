# 移除 Meta Insights 功能设计

## 目标

当前业务只需要短链接广告来源统计和 Meta CAPI 咨询事件回传。系统将移除需要 `ads_read` 或 `ads_management` 权限的 Insights 报告读取能力，使运营人员无需配置广告读取 Token，也不会再看到无法使用的同步入口。

## 保留范围

- Meta 账户继续保存名称、广告账户 ID 和 Graph API 版本，用于归类多个 Pixel、生成 CAPI 请求和加密 Pixel Token。
- Meta Pixel 继续保存 Pixel ID、CAPI Token、启用状态、PageView 开关、手动咨询事件名和自动跳转事件规则。
- 短链接继续绑定 Pixel，并自动保存其所属账户。
- CAPI 继续只处理满足真实广告点击条件的访问；`WhatsAppConsultClick` 与 `WhatsAppAutoRedirect` 保持独立事件。
- 广告来源诊断、短链接广告统计、访问参数和回传事件记录继续保留。
- CSV 广告花费导入继续作为独立功能保留。
- `meta_ad_entities` 表继续保留，因为短链接广告统计会使用其中已有的广告名称；没有缓存名称时继续使用落地页捕获的 `ad_name` 参数。

## 删除范围

### 运营后台

- 删除“Meta 广告报告”菜单、路由和 `MetaReportsView.vue`。
- Meta 账户页面删除广告读取 Token、读取凭证到期日、验证读取、Insights 自动同步、自动回补天数、转化动作类型、报告日期口径、账户时区、币种和最近同步信息。
- Meta 凭证页面只展示 Pixel CAPI 凭证，不再展示账户读取凭证。
- CSV 导入统计页面删除引导用户前往 Meta 广告报告的说明。
- 前端 Meta API 模块删除验证读取、同步、同步任务、重试和报告请求函数。
- 前端工具模块删除读取凭证及 Insights 表单状态、验证和日期范围辅助逻辑；与短链接日期筛选共用的逻辑不删除。

### 后端

- 不再注册账户读取验证、Insights 同步、同步任务、重试和广告报告路由。
- 删除 Insights HTTP 处理器、报告查询、同步调度、同步 Worker 和对应测试代码。
- Meta 后台生命周期只启动 CAPI 事件 Worker。
- Meta 账户保存接口只接受并返回名称、广告账户 ID 和 API 版本；不再接受读取 Token 或 Insights 配置。
- Meta 凭证接口和凭证重加密任务只处理 Pixel CAPI Token 与待发送事件，不再处理账户读取 Token。
- 数据保留任务不再清理同步任务，因为系统不会再创建同步任务。

## 数据库兼容策略

现有迁移文件保持不变，已有 `meta_insights_daily`、`meta_sync_jobs`、`meta_sync_coverage` 和账户读取字段不删除。系统停止读取和写入这些数据。这样可以避免本地已有数据被破坏，并允许以后通过新版本恢复或单独导出历史信息。

新部署仍会按历史迁移创建这些兼容字段和表，但运行代码不会注册相关接口或启动相关任务。当前阶段不新增 `DROP TABLE` 或 `DROP COLUMN` 迁移。

## 接口与页面行为

1. 管理员创建 Meta 账户，只填写连接名称、广告账户 ID 和 API 版本。
2. 管理员在 Meta Pixel 页面选择所属账户，填写 Pixel ID 和对应 CAPI Token。
3. 管理员在短链接中选择 Pixel，系统自动确定账户。
4. 真实广告点击产生符合规则的 PageView、手动咨询或自动跳转事件，并由 CAPI Worker 发送至 `/{pixel-id}/events`。
5. 未配置 Insights 权限不会产生错误提示，也不会影响 CAPI 流程。

## 错误处理与安全

- Pixel Token 继续仅在服务端加密保存，列表和日志不返回明文。
- Pixel 缺少 Token、已停用、已过期或被 Meta 判定无效时，短链接不能选择该 Pixel。
- 已停用的 Insights 路径返回 404，不保留可被误调用的后台接口。
- 历史读取 Token 不再显示、不再使用，也不在重加密任务中读取明文。

## 验证

- 前端测试确认账户表单不再生成 Insights 或读取凭证字段，Pixel 与短链接选择仍正常。
- 后端接口测试确认 Insights 路径全部不可用，账户接口不会接受或返回读取凭证字段。
- 生命周期测试确认只启动并停止 CAPI Worker。
- CAPI 集成测试确认账户没有广告读取权限时，真实广告点击仍能发送到所选 Pixel。
- 短链接广告统计测试确认移除 Insights 后，广告 ID、广告名称参数、访问、独立访客、手动咨询和自动跳转仍能查询。
- 运行前端测试、格式检查、生产构建、后端完整测试和 `go vet ./...`。
