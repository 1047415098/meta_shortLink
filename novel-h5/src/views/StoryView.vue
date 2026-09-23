<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import AppHeader from "../components/AppHeader.vue";
import ChapterDrawer from "../components/ChapterDrawer.vue";
import StoryCard from "../components/StoryCard.vue";
import { fetchNovelStory } from "../lib/api.js";
import { createRequestGate } from "../lib/request.js";

const route=useRoute(),router=useRouter(),story=ref(),chapters=ref([]),related=ref([]),loading=ref(true),error=ref(""),drawer=ref(false);
const { locale, t } = useI18n({ useScope:"global" });
const requestGate=createRequestGate();

function readerRoute(number){
  // 清理旧版 query 章节号，避免新路径中同时存在两个章节编号。
  const query={...route.query};delete query.chapter;
  return { name:"reader", params:{ code:route.params.code, slug:route.params.slug, chapter:number }, query };
}
function start(){
  // 简介页始终从第一条可读章节进入，不受旧的阅读进度影响。
  const first=chapters.value[0]?.chapter_number;
  if(first)void router.push(readerRoute(first));
}
function select(number){drawer.value=false;void router.push(readerRoute(number));}
async function load(){
  const version=requestGate.next(),selectedLocale=locale.value;
  loading.value=true;error.value="";drawer.value=false;
  try{
    const data=await fetchNovelStory(route.params.code,route.params.slug,undefined,selectedLocale);
    if(!requestGate.isCurrent(version))return;
    story.value=data.story;chapters.value=data.chapters;related.value=data.related;
  }catch(e){if(requestGate.isCurrent(version)){error.value=e.message;story.value=undefined;}}
  finally{if(requestGate.isCurrent(version))loading.value=false;}
}
onMounted(load);
onBeforeUnmount(()=>requestGate.cancel());
watch([()=>route.params.slug,locale],load);
</script>

<template>
  <div class="reader-page">
    <div v-if="loading" class="state-page"><i class="fa-solid fa-spinner fa-spin" /><p>{{ t('openingStory') }}</p></div>
    <div v-else-if="error&&!story" class="state-page"><i class="fa-solid fa-triangle-exclamation" /><p>{{ error }}</p></div>
    <template v-else>
      <section class="story-hero" :style="story.cover_path?{'--cover':`url(${story.cover_path})`}:{}">
        <div class="hero-overlay">
          <!-- 简介页保留语言与目录，搜索入口仅留在首页和列表导航中。 -->
          <AppHeader back show-language :show-search="false"><button v-if="chapters.length" class="menu-button" :aria-label="t('openContents')" @click="drawer=true"><i class="fa-solid fa-bars" /></button></AppHeader>
          <div class="hero-content"><div class="hero-cover"><img v-if="story.cover_path" :src="story.cover_path" :alt="t('cover',{title:story.title})" /><i v-else class="fa-solid fa-book-open" /></div><div><h1>{{ story.title }}</h1><p>{{ story.author || t('anonymous') }}</p><small>{{ t('chapterCount',{count:chapters.length}) }}</small></div></div>
        </div>
      </section>
      <article class="story-body">
        <p class="story-excerpt">{{ story.excerpt }}</p>
        <button v-if="chapters.length" class="primary-button" @click="start">{{ t('startReading') }}</button>
        <div v-else class="state-block compact-state"><i class="fa-solid fa-book-open" /><p>{{ t("noReadableChapters") }}</p></div>
        <section v-if="related.length" class="related"><h2>{{ t('mayAlsoLike') }}</h2><div class="story-grid"><StoryCard v-for="item in related" :key="item.id" :story="item" /></div></section>
      </article>
      <ChapterDrawer :open="drawer" :chapters="chapters" @close="drawer=false" @select="select" />
    </template>
  </div>
</template>
