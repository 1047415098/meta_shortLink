import { createApp } from "vue";
import { createRouter, createWebHistory } from "vue-router";
import App from "./App.vue";
import LandingView from "./views/LandingView.vue";
import UnavailableView from "./views/UnavailableView.vue";
import { landingData } from "./bootstrap.js";
import "./styles.css";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/:code",
      component:
        landingData.error || !landingData.link ? UnavailableView : LandingView,
    },
    { path: "/:pathMatch(.*)*", component: UnavailableView },
  ],
});
createApp(App).use(router).mount("#app");
