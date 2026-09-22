import { createRouter, createWebHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import StoryListView from "../views/StoryListView.vue";
import StoryDetailView from "../views/StoryDetailView.vue";
import AudioListView from "../views/AudioListView.vue";
import AudioDetailView from "../views/AudioDetailView.vue";

export function createAudioNovelRouter() {
  return createRouter({
    history: createWebHistory(),
    routes: [
      { path: "/audio-novel/:code", name: "home", component: HomeView },
      { path: "/audio-novel/:code/audio", name: "audio-list", component: AudioListView },
      { path: "/audio-novel/:code/audio/:slug", name: "audio-detail", component: AudioDetailView },
      { path: "/audio-novel/:code/stories", name: "stories", component: StoryListView },
      { path: "/audio-novel/:code/stories/:slug", name: "story", component: StoryDetailView }
    ],
    scrollBehavior: () => ({ top: 0 })
  });
}
