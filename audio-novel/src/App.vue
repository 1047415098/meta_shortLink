<script setup>
import { inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import SiteHeader from "./components/SiteHeader.vue";
import UnavailableView from "./views/UnavailableView.vue";
import { installMetaPixel } from "./lib/meta.js";
import { createVisibleTimeTracker, formatVisibleTime, reportTimeSpent, trackMetaTimeSpent } from "./lib/timeSpent.js";

const bootstrap = inject("bootstrap");
const route = useRoute();
const visibleTime = ref("00:00");
let cleanupVisibleTimer;

onMounted(() => {
  installMetaPixel({ pixelId: bootstrap.meta_browser_pixel_id, eventId: bootstrap.meta_pageview_event_id });
	if (!bootstrap.link?.code) return;
	// One document visit spans every Vue route, so list/detail navigation keeps one continuous timer.
	cleanupVisibleTimer = createVisibleTimeTracker({
	  threshold: Number(bootstrap.link.time_spent_threshold || 0),
	  onTick: (seconds) => { visibleTime.value = formatVisibleTime(seconds); },
	  onThreshold: () => {
		trackMetaTimeSpent({ eventId: bootstrap.meta_time_spent_event_id });
		void reportTimeSpent({ code: bootstrap.link.code, ticket: bootstrap.ticket });
	  },
	});
  if (!bootstrap.ticket) return;
  const body = new URLSearchParams({ ticket: bootstrap.ticket });
  // 每次服务端文档只确认一次 PageView；SPA 内部换页不重复上报。
  fetch(`/audio-novel/${encodeURIComponent(bootstrap.link.code)}/view`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
    keepalive: true
  }).catch(() => {});
});

onBeforeUnmount(() => cleanupVisibleTimer?.());
</script>

<template>
  <div class="site-shell" :class="{ 'is-home': route.name === 'home' }">
    <SiteHeader v-if="!bootstrap.error" />
	<div v-if="!bootstrap.error" class="stay-timer" role="timer" aria-label="Visible reading time">
	  <span aria-hidden="true"></span>
	  Reading time {{ visibleTime }}
	</div>
    <UnavailableView v-if="bootstrap.error" :error="bootstrap.error" />
    <RouterView v-else />
  </div>
</template>

<style scoped>
.stay-timer {
  position: fixed;
  z-index: 30;
  right: 18px;
  bottom: 20px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 9px 13px;
  border: 1px solid rgba(70, 54, 31, 0.18);
  border-radius: 999px;
  color: #59482f;
  background: rgba(250, 247, 239, 0.94);
  box-shadow: 0 8px 24px rgba(49, 38, 24, 0.14);
  font: 600 12px/1.2 Georgia, serif;
  font-variant-numeric: tabular-nums;
  backdrop-filter: blur(8px);
}
.stay-timer span {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #9c6b32;
}
</style>
