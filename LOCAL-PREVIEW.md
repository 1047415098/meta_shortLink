# 当前访问入口

后台：https://sputter-untimely-pardon.ngrok-free.dev

账号：`admin`

密码：`admin`

同一套 Gin 服务运行在 `http://localhost:8080`，并托管三个独立前端：

- 管理后台：`http://localhost:8080/admin/audio-novels`
- 短链接：`http://localhost:8080/hello`
- 语音小说：首页 `http://localhost:8080/audio-novel/hello`，列表 `http://localhost:8080/audio-novel/hello/stories`

需要前端热更新时执行 `./scripts/dev-local.sh`：后台为 5173，短链接落地页为 5174，语音小说为 `http://localhost:5175/audio-novel/hello`。公网 HTTPS 地址仅用于已有会话配置，不替代本地开发入口。
