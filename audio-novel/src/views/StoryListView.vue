<script setup>
import { inject, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { fetchAudioNovelList } from "../lib/api.js";

const bootstrap = inject("bootstrap");
const route = useRoute();
const router = useRouter();
const result = reactive({ items: [], page: 1, pages: 0, total: 0 }), loading = ref(true), error = ref("");
async function load(){loading.value=true;error.value="";try{Object.assign(result,await fetchAudioNovelList(route.params.code,route.query.page||1,6))}catch(cause){error.value=cause.message}finally{loading.value=false}}
watch(() => [route.params.code,route.query.page], load, { immediate: true });

function go(page) {
  router.push({ name: "stories", params: { code: bootstrap.link.code }, query: page > 1 ? { page } : {} });
}
</script>

<template>
  <main class="page-stage">
    <section class="paper list-paper">
      <header class="page-heading">
        <p class="eyebrow ink">The current issue</p>
        <h1>Stories from the Archive</h1>
        <p>Journeys through haunted cities, waking forests, and kingdoms where memory has weight.</p>
      </header>
      <div v-if="loading" class="content-status">Opening the story index…</div>
      <div v-else-if="error" class="content-status"><p>{{ error }}</p><button type="button" @click="load">Try again</button></div>
      <div v-else-if="result.items.length" class="story-list">
        <article v-for="story in result.items" :key="story.slug" class="story-card">
          <div class="story-thumb" :class="{ fallback: !story.cover_path }" :style="story.cover_path ? { backgroundImage: `url(${story.cover_path})` } : {}"></div>
          <div>
            <p class="story-meta">{{ story.category }} · {{ story.published_at }}</p>
            <h2><RouterLink :to="{ name: 'story', params: { code: bootstrap.link.code, slug: story.slug } }">{{ story.title }}</RouterLink></h2>
            <p>{{ story.excerpt }}</p>
            <RouterLink class="read-more" :to="{ name: 'story', params: { code: bootstrap.link.code, slug: story.slug } }">Read story →</RouterLink>
          </div>
        </article>
      </div>
      <div v-else class="content-status">No stories have been published yet.</div>
      <nav v-if="result.pages > 1" class="pagination" aria-label="Story pages">
        <button type="button" :disabled="result.page <= 1" @click="go(result.page - 1)">Previous</button>
        <span>{{ result.page }} / {{ result.pages }}</span>
        <button type="button" :disabled="result.page >= result.pages" @click="go(result.page + 1)">Next</button>
      </nav>
    </section>
  </main>
</template>
