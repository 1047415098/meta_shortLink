# ngrok 公网访问

2026-09-10 已将现有域名切换到本项目 `http://127.0.0.1:8080`，后台和用户落地页共用此域名。

- 后台：https://sputter-untimely-pardon.ngrok-free.dev/admin/overview
- 落地页示例：https://sputter-untimely-pardon.ngrok-free.dev/f943cd06
- 本地服务：http://localhost:8080

需要本机 Docker 和 ngrok 持续运行。免费入口首次可能显示 Visit Site 提示页。链接数据、访客 Cookie 配置及统计口径保持原设置。

## 当前机器启动方式

已替换原来指向 8000 的 ngrok 登录启动项，新的启动项为：

`~/Library/LaunchAgents/com.linkscope.ngrok.plist`

登录时自动启动，并在进程退出后自动重启。它读取本项目的 `deploy/ngrok.yml`。原 8000 项目服务没有停止，但不再通过该 ngrok 域名提供访问。旧启动配置备份在 `backups/ngrok-8000-launchagent-20260910.plist`，没有自动提交到 GitHub。

重启当前隧道：

```sh
launchctl kickstart -k "gui/$(id -u)/com.linkscope.ngrok"
```

日志：`/tmp/linkscope-ngrok.log`。

## 手动启动（其他机器）

先配置本机 ngrok 账号，确认该域名没有被其他在线隧道占用，再在项目根目录执行：

```sh
ngrok start linkscope --config "$HOME/Library/Application Support/ngrok/ngrok.yml" --config ./deploy/ngrok.yml
```

不要同时启动上述手动命令与本机登录启动项，二者会争用同一个域名。
