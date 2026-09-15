# 运营后台

Vue 3 + JavaScript + Vue Router + Element Plus。页面位于 `src/views/`，独立路由位于 `src/router/index.js`；每个页面按 template、script setup、style scoped 排列。

- `layouts/`：跨页面布局。
- `components/`：跨页面复用的标题、筛选、趋势图；不创建页面专用 components 目录。
- `api/`：按业务分组封装后端请求。
- `stores/`：共享登录用户、部署设置。
- `composables/useReport.js`：报表查询、分页、路由筛选同步与请求结果竞态处理。
- `utils/`：无界面依赖的纯函数；`styles/` 只放全局基础样式。

页面独立懒加载，筛选报表和日志使用 URL 查询参数，刷新保留条件。后端需要允许 `/admin/*` 返回后台入口文件。

```sh
npm ci
npm run dev
npm test
npm run build
npm run format:check
```

开发模式 API 代理到 `http://127.0.0.1:8080`。生产目录为 `dist/`，静态资源路径为 `/admin-assets/`，构建自动生成 gzip/Brotli 文件。落地页代码在同级 `../landing/`，不依赖本工程。
