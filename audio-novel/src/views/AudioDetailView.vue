<script setup>
import { inject, ref, watch } from "vue";
import { useRoute } from "vue-router";
import WhatsAppAction from "../components/WhatsAppAction.vue";
import { fetchAudioNovelAudio, formatAudioSize } from "../lib/api.js";

const bootstrap = inject("bootstrap");
const route = useRoute();
const audio = ref(null);
const loading = ref(true);
const error = ref("");
const unavailable = ref(false);
const mediaError = ref(false);

async function load() {
  loading.value = true;
  error.value = "";
  unavailable.value = false;
  mediaError.value = false;
  audio.value = null;
  try {
    audio.value = (await fetchAudioNovelAudio(route.params.code, route.params.slug)).audio;
  } catch (cause) {
    unavailable.value = cause.status === 404;
    error.value = unavailable.value ? "This Podcast is unavailable." : cause.message;
  } finally {
    loading.value = false;
  }
}
watch(() => [route.params.code, route.params.slug], load, { immediate: true });
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
      <audio :src="audio.audio_path" controls preload="metadata" @error="mediaError = true"></audio>
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
