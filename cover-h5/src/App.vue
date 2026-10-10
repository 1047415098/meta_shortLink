<script setup>
import { inject, onBeforeUnmount, onMounted } from "vue";
import PhotoWallStartup from "./components/PhotoWallStartup.vue";
import { installMetaPixel, trackMetaTimeSpent } from "./lib/meta.js";
import { installTikTokPixel, trackTikTokEvent } from "./lib/tiktok.js";
import { createVisibleTimeTracker, postForm } from "./lib/tracking.js";

const bootstrap = inject("bootstrap");
let cleanup;
let visibleSeconds = 0;
let lastReported = 0;
let reportPending = false;
let thresholdReporting = false;
let thresholdAttempts = 0;

async function reportVisible(beacon = false) {
  if (!bootstrap.ticket || visibleSeconds <= lastReported || (!beacon && reportPending)) return;
  const seconds = visibleSeconds;
  reportPending = !beacon;
  const result = await postForm(`/cover/${encodeURIComponent(bootstrap.link.code)}/visible-time`, {
    ticket: bootstrap.ticket,
    seconds: String(seconds),
  }, { beacon });
  reportPending = false;
  if (result.ok && !beacon) lastReported = Math.max(lastReported, Number(result.data?.visible_seconds) || seconds);
}

async function reportThreshold() {
  if (!bootstrap.ticket || thresholdReporting || thresholdAttempts >= 3) return;
  thresholdReporting = true;
  thresholdAttempts += 1;
  const result = await postForm(`/cover/${encodeURIComponent(bootstrap.link.code)}/time-spent`, { ticket: bootstrap.ticket });
  thresholdReporting = false;
  if (result.ok) {
    // Browser events fire only after the server confirms that the configured visible-time threshold was reached.
    if (bootstrap.ad_platform === "meta") trackMetaTimeSpent(bootstrap.meta_time_spent_event_id);
    if (result.data?.tiktok_event) trackTikTokEvent({ name: result.data.tiktok_event.name, eventId: result.data.tiktok_event.event_id });
    return;
  }
  // Browser and server clocks can cross the exact second differently; retry only that narrow threshold race.
  if (result.status === 409 && thresholdAttempts < 3) window.setTimeout(reportThreshold, 1000);
}

function pagehide() { void reportVisible(true); }

onMounted(async () => {
  if (bootstrap.error || !bootstrap.link?.code) return;
  if (bootstrap.ad_platform === "meta") installMetaPixel({ pixelId: bootstrap.meta_browser_pixel_id, eventId: bootstrap.meta_pageview_event_id });
  if (bootstrap.ad_platform === "tiktok" && bootstrap.tiktok_enabled) {
    installTikTokPixel({ pixelCode: bootstrap.tiktok_pixel_code });
  }
  // The signed confirmation makes browser and server PageView delivery idempotent.
  const view = await postForm(`/cover/${encodeURIComponent(bootstrap.link.code)}/view`, { ticket: bootstrap.ticket });
  // TikTok browser PageView is emitted only after the signed visit confirmation succeeds.
  if (view.data?.tiktok_event) trackTikTokEvent({ name: view.data.tiktok_event.name, eventId: view.data.tiktok_event.event_id });
  cleanup = createVisibleTimeTracker({
    threshold: bootstrap.link.time_spent_threshold,
    onTick(seconds) {
      visibleSeconds = seconds;
      if (seconds >= lastReported + 10) void reportVisible();
    },
    onThreshold: reportThreshold,
  });
  window.addEventListener("pagehide", pagehide);
});

onBeforeUnmount(() => {
  void reportVisible(true);
  cleanup?.();
  window.removeEventListener("pagehide", pagehide);
});
</script>

<template>
  <main class="app-shell">
    <section v-if="bootstrap.error" class="cover-unavailable">
      <h1>{{ bootstrap.error.status }}</h1>
      <p>{{ bootstrap.error.message }}</p>
    </section>
    <PhotoWallStartup v-else />
  </main>
</template>
