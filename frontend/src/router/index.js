import { createRouter, createWebHistory } from "vue-router";
import { ensureSession, user } from "../stores/auth";
import { setUnauthorizedHandler } from "../api/http";

import { loadSettings } from "../stores/settings";
const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", redirect: "/admin/overview" },
    {
      path: "/admin/login",
      name: "login",
      component: () => import("../views/LoginView.vue"),
      meta: { title: "登录" },
    },
    {
      path: "/admin",
      component: () => import("../layouts/AdminLayout.vue"),
      children: [
        { path: "", redirect: "/admin/overview" },
        {
          path: "overview",
          name: "overview",
          component: () => import("../views/DashboardView.vue"),
          meta: { title: "数据总览", requiresAuth: true },
        },
        {
          path: "links",
          name: "links",
          component: () => import("../views/LinkListView.vue"),
          meta: { title: "短链接管理", requiresAuth: true },
        },
        {
          // Keep a dedicated, refreshable page for each short link's ad statistics.
          path: "links/:id/stats",
          name: "link-stats",
          component: () => import("../views/LinkStatsView.vue"),
          meta: {
            title: "短链接广告统计",
            requiresAuth: true,
            activeMenu: "links",
          },
        },
        {
          path: "visits",
          name: "visits",
          component: () => import("../views/VisitListView.vue"),
          meta: { title: "访问明细", requiresAuth: true },
        },
        {
          path: "ads",
          name: "ads",
          component: () => import("../views/AdAnalyticsView.vue"),
          meta: { title: "导入统计", requiresAuth: true },
        },
        {
          path: "meta/connections",
          name: "meta-connections",
          component: () => import("../views/MetaConnectionsView.vue"),
          meta: { title: "Meta 连接", requiresAuth: true },
        },
        {
          path: "meta/events",
          name: "meta-events",
          component: () => import("../views/MetaEventsView.vue"),
          meta: { title: "Meta 事件记录", requiresAuth: true },
        },
        {
          path: "meta/pixels",
          name: "meta-pixels",
          component: () => import("../views/MetaPixelsView.vue"),
          meta: { title: "Meta Pixel", requiresAuth: true },
        },
        {
          path: "meta/credentials",
          name: "meta-credentials",
          component: () => import("../views/MetaCredentialsView.vue"),
          meta: { title: "Meta 凭证", requiresAuth: true },
        },
        {
          path: "meta/source",
          name: "meta-source",
          component: () => import("../views/MetaSourceView.vue"),
          meta: { title: "Meta 来源诊断", requiresAuth: true },
        },
        {
          path: "logs",
          name: "logs",
          component: () => import("../views/RequestLogView.vue"),
          meta: { title: "日志管理", requiresAuth: true },
        },
        {
          path: "settings",
          name: "settings",
          component: () => import("../views/SettingsView.vue"),
          meta: { title: "系统设置", requiresAuth: true },
        },
        {
          path: ":pathMatch(.*)*",
          component: () => import("../views/NotFoundView.vue"),
          meta: { title: "页面不存在", requiresAuth: true },
        },
      ],
    },
    {
      path: "/:pathMatch(.*)*",
      component: () => import("../views/NotFoundView.vue"),
    },
  ],
});
router.beforeEach(async (to) => {
  if (to.meta.requiresAuth) {
    try {
      if (!(await ensureSession()))
        return { name: "login", query: { redirect: to.fullPath } };
      await loadSettings();
    } catch {
      return {
        name: "login",
        query: { redirect: to.fullPath, reason: "network" },
      };
    }
  }
  document.title = (to.meta.title || "LinkScope") + " · LinkScope";
});
setUnauthorizedHandler(() => {
  user.value = null;
  if (router.currentRoute.value.name !== "login")
    router.replace({
      name: "login",
      query: { redirect: router.currentRoute.value.fullPath },
    });
});
export default router;
