<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import AppHeader from "../components/AppHeader.vue";
import BottomNav from "../components/BottomNav.vue";
import StoryCard from "../components/StoryCard.vue";
import { fetchNovelHome } from "../lib/api.js";
import { bannerStories, homeSections } from "../lib/content.js";
import { createRequestGate } from "../lib/request.js";

const route=useRoute(),loading=ref(true),error=ref(""),data=reactive(homeSections());
const { locale, t } = useI18n({ useScope:"global" });
const bannerTrack=ref(),activeBanner=ref(0),bannerItems=computed(()=>bannerStories(data.featured,data.items));
const requestGate=createRequestGate();
let autoTimer,scrollTimer;

function goToBanner(index,behavior="smooth"){
  const total=bannerItems.value.length;if(!total)return;
  const target=(index+total)%total;activeBanner.value=target;
  bannerTrack.value?.scrollTo({left:target*bannerTrack.value.clientWidth,behavior});
}
function startAuto(){clearInterval(autoTimer);if(bannerItems.value.length>1)autoTimer=setInterval(()=>goToBanner(activeBanner.value+1),4500);}
function restartAuto(){startAuto();}
function syncBanner(){clearTimeout(scrollTimer);scrollTimer=setTimeout(()=>{const width=bannerTrack.value?.clientWidth||1;activeBanner.value=Math.round((bannerTrack.value?.scrollLeft||0)/width);},80);}
async function load(){const version=requestGate.next(),selectedLocale=locale.value;loading.value=true;error.value="";try{const result=await fetchNovelHome(route.params.code,undefined,selectedLocale);if(!requestGate.isCurrent(version))return;Object.assign(data,homeSections(result));activeBanner.value=0;startAuto();}catch(e){if(requestGate.isCurrent(version))error.value=e.message;}finally{if(requestGate.isCurrent(version))loading.value=false;}}
onMounted(load);
watch(locale,load);
onBeforeUnmount(()=>{requestGate.cancel();clearInterval(autoTimer);clearTimeout(scrollTimer);});
</script>

<template>
  <div class="page with-nav">
    <AppHeader show-language />
    <div v-if="loading" class="state-block"><i class="fa-solid fa-spinner fa-spin" /><p>{{ t('openingShelf') }}</p></div>
    <div v-else-if="error" class="state-block"><i class="fa-solid fa-triangle-exclamation" /><p>{{ error }}</p><button class="secondary-button" @click="load">{{ t('retry') }}</button></div>
    <template v-else-if="data.items.length">
      <section v-if="bannerItems.length" class="featured" :aria-label="t('featuredStories')" @pointerdown="restartAuto">
        <div ref="bannerTrack" class="banner-track" @scroll.passive="syncBanner">
          <article v-for="story in bannerItems" :key="story.id || story.slug" class="banner-slide">
            <div class="featured-cover"><img v-if="story.cover_path" :src="story.cover_path" :alt="t('cover',{title:story.title})" /><i v-else class="fa-solid fa-book-open" /></div>
            <h1>{{ story.title }}</h1>
            <RouterLink class="primary-button" :to="{ name:'story', params:{ code:route.params.code, slug:story.slug } }">{{ t('startReading') }}</RouterLink>
          </article>
        </div>
        <template v-if="bannerItems.length>1">
          <button class="carousel-arrow previous" :aria-label="t('previousFeatured')" @click="goToBanner(activeBanner-1);restartAuto()"><i class="fa-solid fa-chevron-left" /></button>
          <button class="carousel-arrow next" :aria-label="t('nextFeatured')" @click="goToBanner(activeBanner+1);restartAuto()"><i class="fa-solid fa-chevron-right" /></button>
          <div class="dots" :aria-label="t('chooseFeatured')"><button v-for="(_,index) in bannerItems" :key="index" :class="{ active:index===activeBanner }" :aria-label="t('showFeatured',{number:index+1})" @click="goToBanner(index);restartAuto()" /></div>
        </template>
      </section>
      <section v-if="data.ranking.length" class="section"><h2>{{ t('ranking') }}</h2><div class="ranking-grid"><StoryCard v-for="story in data.ranking" :key="story.id" :story="story" compact /></div></section>
      <section class="section"><div class="section-title"><h2>{{ t('popularStories') }}</h2><RouterLink :to="{ name:'stories', params:{ code:route.params.code } }">{{ t('more') }} <i class="fa-solid fa-chevron-right" /></RouterLink></div><div class="story-grid"><StoryCard v-for="story in data.items" :key="story.id" :story="story" /></div></section>
    </template>
    <div v-else class="state-block"><i class="fa-solid fa-book-open" /><p>{{ t('noStories') }}</p></div>
    <BottomNav />
  </div>
</template>
