import { createApp } from "vue";
import "@fortawesome/fontawesome-free/css/all.min.css";
import "./styles.css";
import App from "./App.vue";
import { readBootstrap } from "./bootstrap.js";
import { createNovelRouter } from "./router/index.js";
const bootstrap = readBootstrap(), router = createNovelRouter();
// 服务端注入的短码是公开数据请求的唯一可信入口。
router.beforeEach((to) => bootstrap.link?.code && to.params.code !== bootstrap.link.code ? { name:"home", params:{ code:bootstrap.link.code } } : true);
createApp(App).provide("bootstrap", bootstrap).use(router).mount("#app");
