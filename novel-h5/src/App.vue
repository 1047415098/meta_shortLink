<script setup>
import { inject, onBeforeUnmount, onMounted, provide, ref } from "vue";
import { useRouter } from "vue-router";
import UnavailableView from "./views/UnavailableView.vue";
import { installMetaPixel, trackMetaTimeSpent } from "./lib/meta.js";
import { installTikTokPixel, readTikTokTTP, trackTikTokEvent } from "./lib/tiktok.js";
import { createVisibleTimeTracker, reportReadingTime, reportTimeSpent } from "./lib/timeSpent.js";
import { entryRouteForBootstrap } from "./lib/entry.js";
import { markNovelStartupReady } from "./lib/startupLoader.js";
const bootstrap = inject("bootstrap");
const router=useRouter();
// 临时关闭 18+ 确认与 10 秒倒计时：短链访问直接进入小说；AgeGateView 组件保留，后续可恢复。
const ageGatePassed = ref(true);
let cleanupTimer,visibleSeconds=0,lastReported=0,readingInFlight=false;
let experienceStarted=false;
let initialViewReady=false;
let expectedInitialRouteName="";
function markInitialViewReady(routeName){
  // 短链若直达小说或章节，首页的旧请求不能抢先关闭加载层。
  if(expectedInitialRouteName&&routeName!==expectedInitialRouteName)return;
  if(initialViewReady)return;
  initialViewReady=true;
  // 首次数据已渲染后才允许首屏进度层消失，避免出现白屏闪烁。
  markNovelStartupReady();
}
provide("markInitialViewReady",markInitialViewReady);
async function reportReading(useBeacon=false){
  if(visibleSeconds<=lastReported||!bootstrap.ticket||(!useBeacon&&readingInFlight))return;
  const seconds=visibleSeconds;
  if(!useBeacon)readingInFlight=true;
  // A reader may enter a later chapter directly, so visible-time reports also
  // supplement the first-party TikTok Cookie when StartReading did not run.
  const result=await reportReadingTime({code:bootstrap.link.code,ticket:bootstrap.ticket,seconds,ttp:bootstrap.ad_platform==="tiktok"?readTikTokTTP():"",useBeacon});
  if(!useBeacon)readingInFlight=false;
  if(result.ok&&!useBeacon){
    // Advance only to the server-clipped duration so a failed request retries later.
    lastReported=Math.max(lastReported,result.visibleSeconds);
    if(result.tiktokEvent)trackTikTokEvent({name:result.tiktokEvent.name,eventId:result.tiktokEvent.event_id});
  }
}
function onVisibilityChange(){if(document.visibilityState==="hidden")reportReading();}
function onPageHide(){reportReading(true);}
function startNovelExperience(){
  if(experienceStarted||!bootstrap.link?.code)return;
  experienceStarted=true;
  // 年龄门槛暂时关闭后，短链进入即开始累计真实前台可见阅读时长。
  cleanupTimer = createVisibleTimeTracker({ threshold:Number(bootstrap.link.time_spent_threshold || 0), onTick:(seconds)=>{visibleSeconds=seconds;if(seconds>=lastReported+10)void reportReading();}, onThreshold:() => {
    if(bootstrap.ad_platform === "meta"){trackMetaTimeSpent(bootstrap.meta_time_spent_event_id);void reportTimeSpent({ code:bootstrap.link.code, ticket:bootstrap.ticket });}
    if(bootstrap.ad_platform === "tiktok")void reportReading();
  } });
  document.addEventListener("visibilitychange",onVisibilityChange);
  window.addEventListener("pagehide",onPageHide);
  // 同一文档内进入绑定小说或章节，避免重新请求入口并重复统计访问。
  const entryRoute=entryRouteForBootstrap(bootstrap.link,router.currentRoute.value);
  expectedInitialRouteName=entryRoute?.name||"";
  if(entryRoute)void router.replace(entryRoute);
}
onMounted(() => {
  if(bootstrap.error)markInitialViewReady("unavailable");
  if(bootstrap.ad_platform === "meta")installMetaPixel({ pixelId:bootstrap.meta_browser_pixel_id, eventId:bootstrap.meta_pageview_event_id });
  if(bootstrap.ad_platform === "tiktok"&&bootstrap.tiktok_enabled)installTikTokPixel({pixelCode:bootstrap.tiktok_pixel_code});
  if (!bootstrap.link?.code) return;
  // 保留入口访问确认和平台 PageView；内容停留统计会在短链进入后直接启动。
  if (bootstrap.ticket) fetch(`/novel/${encodeURIComponent(bootstrap.link.code)}/view`, { method:"POST", headers:{ "Content-Type":"application/x-www-form-urlencoded" }, body:new URLSearchParams({ ticket:bootstrap.ticket }), keepalive:true }).catch(()=>{});
  if(ageGatePassed.value)startNovelExperience();
});
onBeforeUnmount(() => {reportReading(true);cleanupTimer?.();document.removeEventListener("visibilitychange",onVisibilityChange);window.removeEventListener("pagehide",onPageHide);});
</script>
<template>
  <main class="app-shell">
    <UnavailableView v-if="bootstrap.error" :error="bootstrap.error" />
    <RouterView v-else />
  </main>
</template>
