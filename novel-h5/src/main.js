import { createApp } from "vue";
import "@fortawesome/fontawesome-free/css/all.min.css";
import "./styles.css";
import App from "./App.vue";
import { readBootstrap } from "./bootstrap.js";
import { createNovelRouter } from "./router/index.js";
import { createNovelI18n } from "./lib/i18n.js";
const bootstrap = readBootstrap(), router = createNovelRouter();
// 服务端注入的短码是公开数据请求的唯一可信入口。
router.beforeEach((to) => bootstrap.link?.code && to.params.code !== bootstrap.link.code ? { name:"home", params:{ code:bootstrap.link.code }, query:to.query } : true);
const localeController=createNovelI18n({bootstrap});
// Vue I18n 管界面文案，控制器只负责可用语言与 Cookie 持久化；接口语言由请求头传递。
createApp(App).provide("bootstrap", bootstrap).provide("localeController",localeController).use(router).use(localeController.plugin).mount("#app");
