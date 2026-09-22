<script setup>
import { inject, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";
import WhatsAppAction from "../components/WhatsAppAction.vue";
import { fetchAudioNovelStory, formatAudioSize } from "../lib/api.js";
import { DEFAULT_SIZE, readSavedSize, saveReadingSize } from "../lib/reading.js";

const bootstrap = inject("bootstrap");
const route = useRoute();
const story = ref(null), related = reactive([]), loading = ref(true), error = ref("");
const size = ref(DEFAULT_SIZE);
const immersive = ref(false);

function setSize(next) { size.value = saveReadingSize(next); }
function setImmersive(value) {
  immersive.value = value;
  document.body.classList.toggle("audio-novel-focus", value);
}
function onKeydown(event) { if (event.key === "Escape") setImmersive(false); }
async function load(){loading.value=true;error.value="";story.value=null;related.splice(0);try{const data=await fetchAudioNovelStory(route.params.code,route.params.slug);story.value=data.story;related.push(...data.related)}catch(cause){error.value=cause.message}finally{loading.value=false}}
watch(() => [route.params.code,route.params.slug], load, { immediate: true });
onMounted(() => { size.value = readSavedSize(); window.addEventListener("keydown", onKeydown); });
onBeforeUnmount(() => { setImmersive(false); window.removeEventListener("keydown", onKeydown); });
</script>

<template>
  <main class="page-stage" :class="{ immersive }">
    <section v-if="loading" class="paper empty-state"><p>Opening the story…</p></section>
    <section v-else-if="story" class="paper story-paper">
      <div class="reading-toolbar" aria-label="Reading controls">
        <button type="button" aria-label="Decrease text size" @click="setSize(size - 2)">A−</button>
        <button type="button" aria-label="Reset text size" @click="setSize(DEFAULT_SIZE)">A</button>
        <button type="button" aria-label="Increase text size" @click="setSize(size + 2)">A+</button>
        <button type="button" @click="setImmersive(!immersive)">{{ immersive ? "Exit focus" : "Focus mode" }}</button>
      </div>
      <header class="story-heading">
        <p class="eyebrow ink">{{ story.category }}</p>
        <h1>{{ story.title }}</h1>
        <p class="byline dark">{{ story.published_at }}</p>
      </header>
      <!-- 文章有关联 MP3 时直接提供原生播放器，同时保留独立 Podcast 页面。 -->
      <section v-if="story.audio_path" class="story-audio" aria-label="Story audio">
        <p class="eyebrow ink">Listen to this story</p>
        <audio :src="story.audio_path" controls preload="metadata"></audio>
        <div class="story-audio-meta">
          <span>{{ story.audio_duration }} · {{ formatAudioSize(story.audio_size_bytes) }}</span>
          <div class="audio-actions">
            <RouterLink class="read-more" :to="{ name: 'audio-detail', params: { code: bootstrap.link.code, slug: story.slug } }">Podcast page</RouterLink>
            <a class="read-more" :href="story.audio_path" download>Download MP3</a>
          </div>
        </div>
      </section>
      <!-- body_html 只来自后端统一安全渲染器。 -->
      <article class="story-body" :style="{ fontSize: `${size}px` }" v-html="story.body_html"></article>
      <WhatsAppAction />
      <section v-if="related.length" class="related">
        <h2>More from the Archive</h2>
        <div class="related-grid">
          <RouterLink v-for="item in related" :key="item.slug" :to="{ name: 'story', params: { code: bootstrap.link.code, slug: item.slug } }">
            <span>{{ item.category }}</span><strong>{{ item.title }}</strong>
          </RouterLink>
        </div>
      </section>
    </section>
    <section v-else class="paper empty-state">
      <p class="eyebrow ink">Lost page</p><h1>{{ error || 'This story is not in the Archive.' }}</h1>
      <button v-if="error" type="button" @click="load">Try again</button>
      <RouterLink class="read-more" :to="{ name: 'stories', params: { code: bootstrap.link.code } }">Return to all stories</RouterLink>
    </section>
  </main>
</template>
