<script setup>
import { inject, onBeforeUnmount, onMounted, provide, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import SiteHeader from "./components/SiteHeader.vue";
import UnavailableView from "./views/UnavailableView.vue";
import { confirmAudioNovelView, reportAudioNovelVisibleTime } from "./lib/api.js";
import { installMetaPixel, trackMetaAudioEvent } from "./lib/meta.js";
import { audioEntryLocation } from "./lib/routes.js";
import { installTikTokPixel, trackTikTokAudioEvent } from "./lib/tiktok.js";
import { createCampaignVisibleTracker, createVisibleTimeTracker, formatVisibleTime, reportTimeSpent, trackMetaTimeSpent } from "./lib/timeSpent.js";

const bootstrap = inject("bootstrap");
const route = useRoute();
const router = useRouter();
const visibleTime = ref("00:00");
let cleanupVisibleTimer;

function trackConfirmedAudioEvents(events = [], content = null) {
  // 浏览器事件只消费服务端确认过的确定性 ID；SDK 异常由各自适配器隔离。
  for (const event of events) {
    if (bootstrap.ad_platform === "meta") trackMetaAudioEvent({ name: event.name, eventId: event.event_id, content });
    if (bootstrap.ad_platform === "tiktok") trackTikTokAudioEvent({ name: event.name, eventId: event.event_id, content });
  }
}
provide("trackConfirmedAudioEvents", trackConfirmedAudioEvents);

onMounted(() => {
  if (!bootstrap.link?.code || bootstrap.error) return;
  if (bootstrap.ad_platform === "meta" && installMetaPixel({ pixelId: bootstrap.meta_browser_pixel_id })) {
    // Campaign PageView waits for the server to validate the frozen dynamic
    // attribution snapshot; legacy archive links keep their original flow.
    if (!bootstrap.playback_ticket) trackMetaAudioEvent({ name: "PageView", eventId: bootstrap.meta_pageview_event_id });
  } else if (bootstrap.ad_platform === "tiktok" && bootstrap.tiktok_enabled && installTikTokPixel({ pixelCode: bootstrap.tiktok_pixel_code })) {
    trackTikTokAudioEvent({ name: "PageView", eventId: bootstrap.tiktok_pageview_event_id });
  }

  if (bootstrap.ticket) {
    // 服务端确认一次入口浏览；相同事件 ID 会被当前文档的 Set 自动去重。
    void confirmAudioNovelView({ code: bootstrap.link.code, ticket: bootstrap.ticket })
      .then((result) => trackConfirmedAudioEvents(result?.confirmed_events))
      .catch(() => {});
  }

  if (bootstrap.playback_ticket) {
    // Campaign page visibility and audio playback are measured independently.
    // Hiding the page pauses this counter while the audio tracker may continue in the background.
    const tracker = createCampaignVisibleTracker({
      report: (seconds, { useBeacon }) => reportAudioNovelVisibleTime({
        code: bootstrap.link.code,
        ticket: bootstrap.playback_ticket,
        visibleSeconds: seconds,
        useBeacon,
      }),
    });
    cleanupVisibleTimer = tracker.cleanup;
  } else {
    // 旧语音站链接继续沿用页面可见时长；新投放链接只按真实播放时长达标。
    cleanupVisibleTimer = createVisibleTimeTracker({
      threshold: Number(bootstrap.link.time_spent_threshold || 0),
      onTick: (seconds) => { visibleTime.value = formatVisibleTime(seconds); },
      onThreshold: () => {
        trackMetaTimeSpent({ eventId: bootstrap.meta_time_spent_event_id });
        void reportTimeSpent({ code: bootstrap.link.code, ticket: bootstrap.ticket });
      },
    });
  }

  const entryLocation = audioEntryLocation(bootstrap, route);
  if (entryLocation) {
    // 保留广告动态参数，且只做 SPA replace，不产生第二次入口 GET。
    void router.replace(entryLocation);
  }
});

onBeforeUnmount(() => cleanupVisibleTimer?.());
</script>

<template>
  <div class="site-shell" :class="{ 'is-home': route.name === 'home' }">
    <SiteHeader v-if="!bootstrap.error" />
    <div v-if="!bootstrap.error && !bootstrap.playback_ticket" class="stay-timer" role="timer" aria-label="Visible reading time">
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
