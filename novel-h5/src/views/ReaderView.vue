<script setup>
import { inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import AppHeader from "../components/AppHeader.vue";
import ChapterDrawer from "../components/ChapterDrawer.vue";
import { fetchNovelChapter, fetchNovelStory } from "../lib/api.js";
import { readProgress, saveProgress } from "../lib/progress.js";
import { createRequestGate } from "../lib/request.js";
import { reportStartReading } from "../lib/timeSpent.js";
import { readTikTokTTP, trackTikTokEvent } from "../lib/tiktok.js";

const route=useRoute(),router=useRouter(),story=ref(),chapters=ref([]),chapter=ref(),previous=ref(),next=ref(),loading=ref(true),error=ref(""),drawer=ref(false);
const bootstrap=inject("bootstrap");
const { locale, t } = useI18n({ useScope:"global" });
const requestGate=createRequestGate();
let scrollTimer;

function readerRoute(number){
  // 阅读页只使用路径中的章节号，并保留当前语言等其他查询参数。
  const query={...route.query};delete query.chapter;
  return { name:"reader", params:{ code:route.params.code, slug:route.params.slug, chapter:number }, query };
}
function select(number){drawer.value=false;void router.push(readerRoute(number));}
function saveScroll(){
  if(!chapter.value)return;
  clearTimeout(scrollTimer);
  scrollTimer=setTimeout(()=>saveProgress(route.params.slug,chapter.value.chapter_number,window.scrollY),160);
}
async function load(){
  const version=requestGate.next(),selectedLocale=locale.value,chapterNumber=Number(route.params.chapter)||1;
  loading.value=true;error.value="";drawer.value=false;
  try{
    // 目录和正文并行加载，让直达章节链接也能完整显示章节导航。
    const [storyData,chapterData]=await Promise.all([
      fetchNovelStory(route.params.code,route.params.slug,undefined,selectedLocale),
      fetchNovelChapter(route.params.code,route.params.slug,chapterNumber,undefined,selectedLocale),
    ]);
    if(!requestGate.isCurrent(version))return;
    story.value=storyData.story;chapters.value=storyData.chapters;
    chapter.value=chapterData.chapter;previous.value=chapterData.previous;next.value=chapterData.next;
    const saved=readProgress(route.params.slug,storyData.chapters.map((item)=>item.chapter_number));
    const scrollY=saved?.chapterNumber===chapterData.chapter.chapter_number?saved.scrollY:0;
    // 先渲染完整正文，再恢复滚动位置，避免加载占位页高度不足导致位置被截断。
    loading.value=false;
    await nextTick();
    if(!requestGate.isCurrent(version))return;
    window.scrollTo({ top:scrollY, behavior:"auto" });
    saveProgress(route.params.slug,chapterData.chapter.chapter_number,scrollY);
    if(chapterData.chapter.chapter_number===1&&bootstrap.ad_platform==="tiktok"&&bootstrap.tiktok_enabled&&bootstrap.ticket){
      const result=await reportStartReading({code:bootstrap.link.code,ticket:bootstrap.ticket,ttp:readTikTokTTP()});
      if(requestGate.isCurrent(version)&&result.tiktokEvent)trackTikTokEvent({name:result.tiktokEvent.name,eventId:result.tiktokEvent.event_id});
    }
  }catch(e){
    if(requestGate.isCurrent(version)){error.value=e.message;story.value=undefined;chapter.value=undefined;}
  }finally{
    if(requestGate.isCurrent(version))loading.value=false;
  }
}
onMounted(()=>{window.addEventListener("scroll",saveScroll,{ passive:true });void load();});
onBeforeUnmount(()=>{requestGate.cancel();clearTimeout(scrollTimer);window.removeEventListener("scroll",saveScroll);});
watch([()=>route.params.slug,()=>route.params.chapter,locale],load);
</script>

<template>
  <div class="reader-page reader-detail-page">
    <!-- 阅读页只保留返回和目录，减少长文阅读时的头部干扰。 -->
    <AppHeader back :show-search="false"><button v-if="chapters.length" class="menu-button" :aria-label="t('openContents')" @click="drawer=true"><i class="fa-solid fa-bars" /></button></AppHeader>
    <div v-if="loading" class="state-page"><i class="fa-solid fa-spinner fa-spin" /><p>{{ t('openingStory') }}</p></div>
    <div v-else-if="error&&!chapter" class="state-page"><i class="fa-solid fa-triangle-exclamation" /><p>{{ error }}</p></div>
    <article v-else class="reader-content">
      <section class="chapter-content" aria-live="polite">
        <header class="reader-chapter-heading">
          <span>{{ t('chapter',{number:chapter.chapter_number}) }}</span>
          <h1>{{ chapter.title || story.title }}</h1>
          <p>{{ story.title }}</p>
        </header>
        <div class="prose" v-html="chapter.body_html" />
        <div class="chapter-actions">
          <RouterLink v-if="previous" class="secondary-button" :to="readerRoute(previous.chapter_number)"><i class="fa-solid fa-chevron-left" /> {{ t('previous') }}</RouterLink>
          <RouterLink v-if="next" class="primary-button" :to="readerRoute(next.chapter_number)">{{ t('continueReading') }} <i class="fa-solid fa-chevron-right" /></RouterLink>
          <span v-else class="finished"><i class="fa-solid fa-circle-check" /> {{ t('reachedEnd') }}</span>
        </div>
      </section>
    </article>
    <ChapterDrawer :open="drawer" :chapters="chapters" :current="chapter?.chapter_number" @close="drawer=false" @select="select" />
  </div>
</template>
