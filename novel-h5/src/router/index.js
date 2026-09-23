import { createRouter, createWebHistory } from "vue-router";
export function createNovelRouter() {
  return createRouter({ history:createWebHistory(), scrollBehavior:(to, from, saved) => saved || (to.name === from.name ? undefined : { top:0 }), routes:[
    { path:"/novel/:code", name:"home", component:() => import("../views/HomeView.vue") },
    { path:"/novel/:code/search", name:"search", component:() => import("../views/SearchView.vue") },
    { path:"/novel/:code/stories", name:"stories", component:() => import("../views/StoryListView.vue") },
    { path:"/novel/:code/stories/:slug", name:"story", component:() => import("../views/StoryView.vue") },
    { path:"/novel/:code/stories/:slug/chapters/:chapter", name:"reader", component:() => import("../views/ReaderView.vue") },
    { path:"/:pathMatch(.*)*", redirect:(to) => `/novel/${to.params.code || "hello"}` }
  ] });
}
