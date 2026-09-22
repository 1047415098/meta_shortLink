<script setup>
import { inject, onBeforeUnmount, onMounted } from "vue";
import { useRouter } from "vue-router";
import UnavailableView from "./views/UnavailableView.vue";
import { installMetaPixel, trackMetaTimeSpent } from "./lib/meta.js";
import { createVisibleTimeTracker, reportReadingTime, reportTimeSpent } from "./lib/timeSpent.js";
const bootstrap = inject("bootstrap");
const router=useRouter();
let cleanupTimer,visibleSeconds=0,lastReported=0;
function reportReading(useBeacon=false){
  if(visibleSeconds<=lastReported||!bootstrap.ticket)return;
  const seconds=visibleSeconds;lastReported=seconds;
  void reportReadingTime({code:bootstrap.link.code,ticket:bootstrap.ticket,seconds,useBeacon});
}
function onVisibilityChange(){if(document.visibilityState==="hidden")reportReading();}
function onPageHide(){reportReading(true);}
onMounted(() => {
  installMetaPixel({ pixelId:bootstrap.meta_browser_pixel_id, eventId:bootstrap.meta_pageview_event_id });
  if (!bootstrap.link?.code) return;
  // 整个 SPA 文档只确认一次浏览，并连续累计可见停留时间。
  if (bootstrap.ticket) fetch(`/novel/${encodeURIComponent(bootstrap.link.code)}/view`, { method:"POST", headers:{ "Content-Type":"application/x-www-form-urlencoded" }, body:new URLSearchParams({ ticket:bootstrap.ticket }), keepalive:true }).catch(()=>{});
  cleanupTimer = createVisibleTimeTracker({ threshold:Number(bootstrap.link.time_spent_threshold || 0), onTick:(seconds)=>{visibleSeconds=seconds;if(seconds>=lastReported+10)reportReading();}, onThreshold:() => { trackMetaTimeSpent(bootstrap.meta_time_spent_event_id); void reportTimeSpent({ code:bootstrap.link.code, ticket:bootstrap.ticket }); } });
  document.addEventListener("visibilitychange",onVisibilityChange);
  window.addEventListener("pagehide",onPageHide);
  // A bound campaign opens its story directly while keeping this document and visit intact.
  if(bootstrap.link.entry_story_slug&&router.currentRoute.value.name==="home")void router.replace({name:"story",params:{code:bootstrap.link.code,slug:bootstrap.link.entry_story_slug}});
});
onBeforeUnmount(() => {reportReading(true);cleanupTimer?.();document.removeEventListener("visibilitychange",onVisibilityChange);window.removeEventListener("pagehide",onPageHide);});
</script>
<template><main class="app-shell"><UnavailableView v-if="bootstrap.error" :error="bootstrap.error" /><RouterView v-else /></main></template>
