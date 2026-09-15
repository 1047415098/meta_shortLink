# Meta 咨询事件回传

本系统当前只提供 Facebook / Meta 广告来源识别与 Pixel CAPI 事件回传，不读取 Meta 广告报表，不需要 `ads_read` 或 `ads_management` 权限，也不读取 WhatsApp 消息。

后台入口：

- `/admin/meta/connections`：管理广告账户分组；Graph API 版本由程序统一维护。
- `/admin/meta/pixels`：在账户下管理多个 Pixel、CAPI Token、事件规则和测试事件。
- `/admin/meta/credentials`：查看 Pixel 凭证状态、加密密钥轮换和审计记录。
- `/admin/meta/source`：生成广告网址参数并诊断来源。
- `/admin/meta/events`：查看 CAPI 回传状态、失败原因、重试和 Meta 回执。

## 1. 配置账户和 Pixel

先在「Meta 连接」添加账户名称和广告账户 ID。账户 ID 只填写数字，不带 `act_`。一个账户可以添加多个 Pixel。

然后在「Meta Pixel」添加：

- 所属账户。
- Pixel 名称和 Pixel ID。
- CAPI Token。
- PageView、手动咨询 `Contact` 和自动跳转 `WhatsAppAutoRedirect` 的独立开关。

短链接只需要选择具体 Pixel，系统自动保存该 Pixel 所属账户。多个短链接可以共用一个 Pixel，同一短链接也可以通过每次访问携带的广告参数区分不同广告。

Token 仅在服务端加密保存，后台不会返回明文，也不维护人工到期日。Pixel ID 和所属账户保存后不可更改，需要调整时请新建 Pixel，避免改写历史事件归属。凭证状态以实际回传结果为准。

## 2. 验证 CAPI

1. 保存 Pixel ID 和对应 CAPI Token。
2. 在目标 Pixel 点击「测试事件」，填写事件管理工具显示的 `test_event_code`。
3. 在「Meta 事件记录」确认任务状态、`events_received` 和 `fbtrace_id`。
4. 在 Meta 事件管理工具中核对同一个 Pixel、事件名和发送时间。

服务端发送到：

```text
POST https://graph.facebook.com/{api-version}/{pixel-id}/events
Authorization: Bearer {capi-token}
```

测试事件在请求体顶层携带 `test_event_code`。Meta 返回 `events_received=1` 表示接口接收了本次事件；广告归因结果仍以 Meta 平台最终处理为准。

## 3. 广告链接参数

建议在 Meta 广告层级的「网址参数」中填写：

```text
campaign_id={{campaign.id}}&campaign_name={{campaign.name}}&adset_id={{adset.id}}&adset_name={{adset.name}}&ad_id={{ad.id}}&ad_name={{ad.name}}&utm_campaign={{campaign.name}}&placement={{placement}}&site_source_name={{site_source_name}}
```

Meta 在真实广告点击时通常追加 `fbclid`。系统将同时满足以下条件的入口认定为真实广告点击：

- 有有效 `fbclid`。
- 有已展开的有效 `ad_id`。
- 访问没有被识别为机器人或可疑流量。
- 来源参数之间没有冲突。

未展开的 `{{...}}` 不作为有效 ID。名称仅用于展示，`campaign_id`、`adset_id` 和 `ad_id` 用于关联。同一短链接挂到多条广告时，广告表现按每次真实点击携带的 `ad_id` 分开统计。

不要把 Meta API Token 或 CAPI Token 放入广告网址。来源诊断会拒绝包含 token、secret、password 等凭证参数的链接。

## 4. 回传与统计口径

每次入口访问生成签名票据，并冻结当时的账户、Pixel、广告参数和事件规则。之后修改短链接或 Pixel 不会改变已经发生的访问归属。

| 站内行为 | 站内统计 | CAPI 事件 |
| --- | --- | --- |
| 真实广告落地页访问 | 计入真实广告访问与 Cookie 去重访客 | Pixel 开启 PageView 时回传 `PageView` |
| 用户手动点击咨询 | 同一次访问最多计一次 | Pixel 开启手动咨询时回传标准事件 `Contact` |
| 倒计时自动跳转 | 同一次访问最多计一次并单列 | Pixel 开启自动跳转时回传 `WhatsAppAutoRedirect` |
| 非真实广告流量 | 仍可进入普通站内统计 | 记录为跳过，不发送到 Meta |

CAPI 事件异步发送，不阻塞用户跳转。相同访问和动作使用稳定事件 ID，浏览器重复提交不会重复创建业务事件；网络失败自动重试并保持原事件时间和事件 ID。

系统只读取本站收到的 `_fbc`、`_fbp`、`fbclid`、IP 和 User-Agent，用于 Meta 匹配。系统无法读取 Facebook 或 WhatsApp 域名的登录 Cookie，也不会读取聊天内容。

## 5. 数据安全和运维

- CAPI Token 使用 `APP_SECRET` 或已配置的独立 Meta 加密密钥加密。
- 回传成功后清除加密的事件匹配载荷；结束事件按 `RETENTION_DAYS` 清理。
- 「Meta 凭证」的重加密只处理 Pixel Token 和仍待发送的事件载荷。
- 历史数据库迁移中保留的旧报表表和字段不再由程序读写，避免升级时破坏旧数据。
- CSV 花费导入仍是独立功能，只用于站内成本分析。

本地验证：

```sh
TEST_DATABASE_URL='postgres://.../app_test?sslmode=disable' go test -race ./...
```

测试数据库必须可独立销毁，不能指向线上数据库。前端还应执行 `npm test` 和 `npm run build`。
