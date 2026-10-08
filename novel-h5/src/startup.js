import { installStartupLoader } from "./lib/startupLoader.js";

// 首屏脚本独立于 Vue 挂载，以便在路由组件请求数据期间持续显示加载进度。
installStartupLoader();
