import { createApp } from "vue";
import "@fortawesome/fontawesome-free/css/all.min.css";
import "./styles.css";
import App from "./App.vue";
import { readBootstrap } from "./bootstrap.js";
import { createCoverI18n } from "./lib/i18n.js";

const bootstrap = readBootstrap();
const i18n = createCoverI18n(bootstrap);
// The cover frontend is deliberately independent from the novel router and content APIs.
createApp(App).provide("bootstrap", bootstrap).use(i18n).mount("#app");
