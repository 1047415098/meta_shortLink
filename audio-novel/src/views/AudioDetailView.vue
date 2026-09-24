<script setup>
import { inject, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { onBeforeRouteUpdate, useRoute } from "vue-router";
import WhatsAppAction from "../components/WhatsAppAction.vue";
import {
  completeAudioNovelPlayback,
  fetchAudioNovelAudio,
  formatAudioSize,
  reportAudioNovelPlayback,
  startAudioNovelPlayback,
} from "../lib/api.js";
import { createPlaybackTracker } from "../lib/playback.js";

const bootstrap = inject("bootstrap");
const trackConfirmedAudioEvents = inject("trackConfirmedAudioEvents", () => {});
const route = useRoute();
const audio = ref(null);
const audioElement = ref(null);
const loading = ref(true);
const error = ref("");
const unavailable = ref(false);
const mediaError = ref(false);
let loadVersion = 0;
let reportTimer;
let startConfirmed = false;
let startRequest = null;

function trackingEnabled() {
  return Boolean(bootstrap.playback_ticket && bootstrap.entry_audio_slug && route.params.slug === bootstrap.entry_audio_slug);
}

function eventContent() {
  return audio.value ? { id: audio.value.id, title: audio.value.title } : null;
}

const playbackTracker = createPlaybackTracker({
  currentPlaybackRate: () => audioElement.value?.playbackRate || 1,
  report: async (payload, { useBeacon }) => {
    if (!trackingEnabled()) return null;
    const result = await reportAudioNovelPlayback({
      code: bootstrap.link.code,
      ticket: bootstrap.playback_ticket,
      playbackSeconds: payload.playback_seconds,
      mediaConsumedSeconds: payload.media_consumed_seconds,
      useBeacon,
    });
    if (!useBeacon) trackConfirmedAudioEvents(result?.confirmed_events, eventContent());
    return result;
  },
});

async function ensureStarted() {
  if (!trackingEnabled() || startConfirmed) return;
  if (startRequest) return startRequest;
  startRequest = startAudioNovelPlayback({ code: bootstrap.link.code, ticket: bootstrap.playback_ticket })
    .then((result) => {
      startConfirmed = Boolean(result?.started);
      trackConfirmedAudioEvents(result?.confirmed_events, eventContent());
      return result;
    })
    .catch(() => null)
    .finally(() => { startRequest = null; });
  return startRequest;
}

function onPlaying() {
  if (!trackingEnabled()) return;
  playbackTracker.playing();
  void ensureStarted();
}
function onPause() { if (trackingEnabled()) playbackTracker.pause(); }
function onWaiting() { if (trackingEnabled()) playbackTracker.waiting(); }
function onStalled() { if (trackingEnabled()) playbackTracker.stalled(); }
function onRateChange() { if (trackingEnabled()) playbackTracker.rateChanged(); }
function onSeeking() { if (trackingEnabled()) playbackTracker.seeking(); }
function onSeeked() {
  if (trackingEnabled()) playbackTracker.seeked({ paused: audioElement.value?.paused, ended: audioElement.value?.ended });
}
async function onEnded() {
  if (!trackingEnabled()) return;
  playbackTracker.ended();
  await ensureStarted();
  const payload = playbackTracker.snapshot();
  try {
    const result = await completeAudioNovelPlayback({
      code: bootstrap.link.code,
      ticket: bootstrap.playback_ticket,
      playbackSeconds: payload.playback_seconds,
      mediaConsumedSeconds: payload.media_consumed_seconds,
    });
    trackConfirmedAudioEvents(result?.confirmed_events, eventContent());
  } catch {
    // 完成统计失败不影响用户重播、下载或继续浏览正文。
  }
}

async function reportProgress() {
  if (!trackingEnabled()) return;
  // The interval also flushes a segment that ended on pause/waiting; the
  // tracker itself skips requests when no unconfirmed progress exists.
  await ensureStarted();
  await playbackTracker.flush();
}

function onPageHide() {
  if (trackingEnabled()) void playbackTracker.flush({ useBeacon: true });
}

async function load() {
  const version = ++loadVersion;
  playbackTracker.pause();
  loading.value = true;
  error.value = "";
  unavailable.value = false;
  mediaError.value = false;
  audio.value = null;
  try {
    const result = await fetchAudioNovelAudio(route.params.code, route.params.slug);
    // 快速切换条目时忽略过期响应，避免旧音频覆盖当前播放器。
    if (version !== loadVersion) return;
    audio.value = result.audio;
  } catch (cause) {
    if (version !== loadVersion) return;
    unavailable.value = cause.status === 404;
    error.value = unavailable.value ? "This Podcast is unavailable." : cause.message;
  } finally {
    if (version === loadVersion) loading.value = false;
  }
}
watch(() => [route.params.code, route.params.slug], load, { immediate: true });
onBeforeRouteUpdate(() => {
  // 离开绑定音频前补报最后一段，避免把它误记到随后浏览的其它音频。
  onPageHide();
  playbackTracker.pause();
});
onMounted(() => {
  reportTimer = setInterval(reportProgress, 10_000);
  window.addEventListener("pagehide", onPageHide);
});
onBeforeUnmount(() => {
  clearInterval(reportTimer);
  window.removeEventListener("pagehide", onPageHide);
  onPageHide();
  playbackTracker.pause();
});
</script>

<template>
  <main class="page-stage">
    <section v-if="loading" class="paper empty-state"><p>Opening the Podcast…</p></section>
    <article v-else-if="audio" class="paper audio-detail">
      <header class="story-heading">
        <p class="eyebrow ink">Podcast</p>
        <h1>{{ audio.title }}</h1>
        <p class="byline dark">{{ audio.published_at }} · {{ audio.audio_duration }} · {{ formatAudioSize(audio.audio_size_bytes) }}</p>
      </header>
      <audio
        ref="audioElement"
        :src="audio.audio_path"
        controls
        preload="metadata"
        @playing="onPlaying"
        @pause="onPause"
        @waiting="onWaiting"
        @stalled="onStalled"
        @ratechange="onRateChange"
        @seeking="onSeeking"
        @seeked="onSeeked"
        @ended="onEnded"
        @error="mediaError = true"
      ></audio>
      <p v-if="mediaError" class="action-error">The audio could not be played. You can retry or download the MP3.</p>
      <p class="audio-description">{{ audio.excerpt }}</p>
      <div class="audio-actions detail-actions">
        <a class="primary-link" :href="audio.audio_path" download>Download MP3</a>
        <RouterLink class="read-more" :to="{ name: 'story', params: { code: bootstrap.link.code, slug: audio.slug } }">Read Story</RouterLink>
      </div>
      <WhatsAppAction />
    </article>
    <section v-else class="paper empty-state">
      <p class="eyebrow ink">Podcast</p>
      <h1>{{ error || "This Podcast is unavailable." }}</h1>
      <button v-if="error && !unavailable" type="button" @click="load">Try again</button>
      <RouterLink class="read-more" :to="{ name: 'story', params: { code: bootstrap.link.code, slug: route.params.slug } }">Read Story</RouterLink>
    </section>
  </main>
</template>
