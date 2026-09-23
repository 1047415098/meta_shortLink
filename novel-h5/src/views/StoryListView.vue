<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from "vue"; import { useRoute } from "vue-router"; import { useI18n } from "vue-i18n"; import AppHeader from "../components/AppHeader.vue"; import BottomNav from "../components/BottomNav.vue"; import StoryCard from "../components/StoryCard.vue"; import { fetchNovelList } from "../lib/api.js"; import { createRequestGate } from "../lib/request.js";
const route=useRoute(),items=ref([]),page=ref(1),pages=ref(1),loading=ref(false),error=ref("");
const { locale, t } = useI18n({ useScope:"global" });
const requestGate=createRequestGate();
async function load(){if(loading.value||page.value>pages.value)return;const version=requestGate.next(),requestedPage=page.value,selectedLocale=locale.value;loading.value=true;error.value="";try{const data=await fetchNovelList(route.params.code,{page:requestedPage,pageSize:20,locale:selectedLocale});if(!requestGate.isCurrent(version))return;items.value.push(...data.items.filter(next=>!items.value.some(item=>item.id===next.id)));pages.value=data.pages||1;page.value=requestedPage+1;}catch(e){if(requestGate.isCurrent(version))error.value=e.message;}finally{if(requestGate.isCurrent(version))loading.value=false;}}
function reset(){requestGate.cancel();loading.value=false;items.value=[];page.value=1;pages.value=1;load();}
onMounted(load);
watch(locale,reset);
onBeforeUnmount(()=>requestGate.cancel());
</script>
<template><div class="page with-nav"><AppHeader back /><section class="section list-page"><div class="page-heading"><p>{{ t('browseShelf') }}</p><h1>{{ t('allStories') }}</h1></div><div v-if="error" class="state-block compact-state"><p class="error-text">{{ error }}</p><button class="secondary-button" @click="load">{{ t('retry') }}</button></div><div class="story-grid"><StoryCard v-for="story in items" :key="story.id" :story="story" /></div><button v-if="!error&&page<=pages" class="load-more" :disabled="loading" @click="load">{{ loading?t('loading'):t('loadMore') }}</button><div v-else-if="!error&&!items.length&&!loading" class="state-block"><p>{{ t('noStories') }}</p></div></section><BottomNav /></div></template>
