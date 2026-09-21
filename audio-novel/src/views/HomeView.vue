<script setup>
import { computed, inject, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { fetchAudioNovelHome } from "../lib/api.js";
import WhatsAppAction from "../components/WhatsAppAction.vue";
const bootstrap = inject("bootstrap");
const route = useRoute(), featuredStory = ref(null), loading = ref(true), error = ref("");
const heroStyle = computed(() => featuredStory.value?.cover_path ? { backgroundImage: `url(${featuredStory.value.cover_path})` } : {});
async function load(){loading.value=true;error.value="";try{featuredStory.value=(await fetchAudioNovelHome(route.params.code)).featured}catch(cause){error.value=cause.message}finally{loading.value=false}}
watch(() => route.params.code, load, { immediate: true });
</script>

<template>
  <main class="hero" :class="{ 'has-cover': featuredStory?.cover_path }" :style="heroStyle">
    <div class="hero-scrim"></div>
    <section class="hero-copy">
      <p class="eyebrow">Issue No. 01 · Autumn 2026</p>
      <h1>Stories beyond the last mapped road.</h1>
      <p class="hero-intro">Original fantasy fiction for readers who still believe a doorway can open anywhere.</p>
      <div v-if="loading" class="featured-status">Opening the archive…</div>
      <div v-else-if="error" class="featured-status"><p>{{ error }}</p><button type="button" @click="load">Try again</button></div>
      <template v-else-if="featuredStory"><div class="featured-rule"></div>
      <p class="story-kicker">Featured story</p><h2>{{ featuredStory.title }}</h2>
      <div class="hero-actions">
        <RouterLink class="primary-link" :to="{ name: 'story', params: { code: bootstrap.link.code, slug: featuredStory.slug } }">Read the story</RouterLink>
        <RouterLink class="text-link" :to="{ name: 'stories', params: { code: bootstrap.link.code } }">Browse the issue →</RouterLink>
      </div></template><div v-else class="featured-status"><p>No stories have been published yet.</p><RouterLink class="text-link" :to="{ name: 'stories', params: { code: bootstrap.link.code } }">Browse the archive →</RouterLink></div>
      <WhatsAppAction />
    </section>
    <p class="hero-credit">The Lantern Archive</p>
  </main>
</template>
