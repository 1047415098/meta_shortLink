<script setup>
import { inject, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { fetchAudioNovelAudioList, formatAudioSize } from "../lib/api.js";

const bootstrap = inject("bootstrap");
const route = useRoute();
const router = useRouter();
const result = reactive({ items: [], page: 1, pages: 0, total: 0 });
const loading = ref(true);
const error = ref("");

async function load() {
  loading.value = true;
  error.value = "";
  try {
    Object.assign(result, await fetchAudioNovelAudioList(route.params.code, route.query.page || 1, 6));
  } catch (cause) {
    error.value = cause.message;
  } finally {
    loading.value = false;
  }
}
watch(() => [route.params.code, route.query.page], load, { immediate: true });

function go(page) {
  router.push({ name: "audio-list", params: { code: bootstrap.link.code }, query: page > 1 ? { page } : {} });
}
</script>

<template>
  <main class="page-stage">
    <section class="paper list-paper">
      <header class="page-heading">
        <p class="eyebrow ink">Listen to the Archive</p>
        <h1>Audio Fiction</h1>
        <p>Complete narrated stories from strange kingdoms, waking forests, and worlds beyond the mapped road.</p>
      </header>
      <div v-if="loading" class="content-status">Opening the audio archive…</div>
      <div v-else-if="error" class="content-status"><p>{{ error }}</p><button type="button" @click="load">Try again</button></div>
      <div v-else-if="result.items.length" class="audio-list">
        <article v-for="item in result.items" :key="item.slug" class="audio-card">
          <p class="story-meta">{{ item.published_at }} · {{ item.audio_duration }} · {{ formatAudioSize(item.audio_size_bytes) }}</p>
          <h2>{{ item.title }}</h2>
          <p>{{ item.excerpt }}</p>
          <audio :src="item.audio_path" controls preload="metadata"></audio>
          <div class="audio-actions">
            <a class="read-more" :href="item.audio_path" download>Download</a>
            <RouterLink class="read-more" :to="{ name: 'audio-detail', params: { code: bootstrap.link.code, slug: item.slug } }">More →</RouterLink>
          </div>
        </article>
      </div>
      <div v-else class="content-status">No audio fiction has been published yet.</div>
      <nav v-if="result.pages > 1" class="pagination" aria-label="Audio fiction pages">
        <button type="button" :disabled="result.page <= 1" @click="go(result.page - 1)">Previous</button>
        <span>{{ result.page }} / {{ result.pages }}</span>
        <button type="button" :disabled="result.page >= result.pages" @click="go(result.page + 1)">Next</button>
      </nav>
    </section>
  </main>
</template>
