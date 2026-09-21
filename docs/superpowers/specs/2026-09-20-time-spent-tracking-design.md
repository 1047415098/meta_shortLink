# 小说内容与 TimeSpent 设计

## 目标

- 小说继续使用独立 `novels` 数据中心和 `/admin/novels` CRUD，不与短链接展示字段混存。
- 同一条 `short_links.code` 同时作为短链接入口和 `/novel/:code` 小说入口的业务配置。
- 每个 code 可配置 TimeSpent 阈值；`0` 表示关闭事件，`5–3600` 表示达到对应的前台可见秒数后上报。
- 短链接落地页和小说站全部页面始终展示本次访问的可见停留时间。

## 数据与事件边界

`short_links.time_spent_threshold` 保存当前配置。创建访问记录时把阈值冻结到 `click_events.time_spent_threshold`，避免运营修改配置后影响已打开的页面。达到阈值后，服务端写入 `time_spent_reported_at`；同一访问最多写入一次。

浏览器只累计页面处于可见状态的时间，标签页隐藏时暂停。服务端不信任客户端传入的秒数，而是校验该访问确实已存在至少阈值时长。通过校验后生成自定义 Meta 事件 `TimeSpent`，浏览器 Pixel 和 CAPI 共用 `wa_<visit>_time_spent` 事件 ID 去重。

## 路由

- 短链接：`POST /:code/time-spent`
- 小说站：`POST /novel/:code/time-spent`

两条路由复用已有签名 ticket、同源校验和访问 surface 校验。重复请求返回成功，提前请求返回冲突状态且不产生事件。

## 前台体验

两个独立 Vue 3 应用各自包含一个简洁的固定计时器。小说 SPA 内部切换首页、列表和详情时计时连续，不重复创建访问或事件；整页刷新则是新的访问。

