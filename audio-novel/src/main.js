import { createApp } from "vue";
import App from "./App.vue";
import { readBootstrap } from "./bootstrap.js";
import { createAudioNovelRouter } from "./router/index.js";
import "./styles.css";

const bootstrap = readBootstrap();
const router = createAudioNovelRouter();

// 服务端启动数据是 code 与访问票据的唯一可信来源。
router.beforeEach((to) => {
  if (bootstrap.link?.code && to.params.code !== bootstrap.link.code) {
    return { name: "home", params: { code: bootstrap.link.code } };
  }
  return true;
});

createApp(App).provide("bootstrap", bootstrap).use(router).mount("#app");
