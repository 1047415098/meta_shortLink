import test from "node:test";
import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { installMetaPixel } from "../src/lib/meta.js";
import { createVisibleTimeTracker, reportReadingTime, reportStartReading, reportTimeSpent } from "../src/lib/timeSpent.js";

test("novel Meta PageView is installed once per event", () => {
  const scope={},dataset={},head={ appendChild(){} },documentRef={ documentElement:{ dataset },head,createElement:()=>({}) };
  assert.equal(installMetaPixel({ pixelId:"123456",eventId:"event-1",scope,documentRef }),true);
  installMetaPixel({ pixelId:"123456",eventId:"event-1",scope,documentRef });
  assert.equal(dataset.novelMetaPageView,"event-1");
  assert.equal(scope.fbq.queue.filter((item)=>item[0]==="track").length,1);
});

test("novel TimeSpent uses its own signed surface endpoint once", async () => {
  const calls=[],state={};
  assert.equal(await reportTimeSpent({ code:"Ab_C",ticket:"signed",state,request:async(url,options)=>{calls.push([url,options]);return {ok:true};} }),true);
  assert.equal(await reportTimeSpent({ code:"Ab_C",ticket:"signed",state,request:async()=>{} }),false);
  assert.equal(calls[0][0],"/novel/Ab_C/time-spent");
});

test("visible tracker emits cumulative foreground seconds", () => {
  let clock=0,interval,visibilityState="visible";const listeners={};const seconds=[];
  const documentRef={get visibilityState(){return visibilityState;},addEventListener:(name,fn)=>listeners[name]=fn,removeEventListener(){}};
  const cleanup=createVisibleTimeTracker({documentRef,now:()=>clock,schedule:(fn)=>{interval=fn;return 1;},cancel:()=>{},onTick:(value)=>seconds.push(value)});
  clock=12000;interval();visibilityState="hidden";listeners.visibilitychange();clock=30000;interval();
  assert.equal(seconds.at(-1),12);
  cleanup();
});

test("reading time reports cumulative seconds with keepalive", async () => {
  const calls=[];
  const result=await reportReadingTime({code:"wife-a",ticket:"signed",seconds:37,ttp:"cookie-1",request:async(url,options)=>{calls.push([url,options]);return {ok:true,json:async()=>({visible_seconds:35,tiktok_event:{name:"ViewContent",event_id:"event-1"}})};}});
  assert.deepEqual(result,{ok:true,visibleSeconds:35,tiktokEvent:{name:"ViewContent",event_id:"event-1"}});
  assert.equal(calls[0][0],"/novel/wife-a/reading-time");
  assert.match(String(calls[0][1].body),/seconds=37/);
  assert.match(String(calls[0][1].body),/_ttp=cookie-1/);
  assert.equal(calls[0][1].keepalive,true);
});

test("start reading posts chapter one and returns only a confirmed browser event", async () => {
  const calls=[];
  const result=await reportStartReading({code:"wife-a",ticket:"signed",ttp:"cookie-1",request:async(url,options)=>{calls.push([url,options]);return {ok:true,json:async()=>({ok:true,tiktok_event:{name:"StartReading",event_id:"event-start"}})};}});
  assert.deepEqual(result,{ok:true,tiktokEvent:{name:"StartReading",event_id:"event-start"}});
  assert.equal(calls[0][0],"/novel/wife-a/start-reading");
  assert.match(String(calls[0][1].body),/chapter=1/);
  assert.match(String(calls[0][1].body),/_ttp=cookie-1/);
});

test("story introduction shows an explicit empty state when no chapters are enabled", async () => {
  const source=await readFile(new URL("../src/views/StoryView.vue",import.meta.url),"utf8");
  assert.match(source,/t\("noReadableChapters"\)/);
  assert.match(source,/name:\s*"reader"/);
});

test("story introduction and chapter reader have separate page responsibilities", async () => {
  const header=await readFile(new URL("../src/components/AppHeader.vue",import.meta.url),"utf8");
  const story=await readFile(new URL("../src/views/StoryView.vue",import.meta.url),"utf8");
  const readerURL=new URL("../src/views/ReaderView.vue",import.meta.url);
  assert.equal(existsSync(readerURL),true,"ReaderView.vue should exist");
  const reader=await readFile(readerURL,"utf8");
  assert.match(header,/<slot\s*\/>/);
  assert.match(story,/<AppHeader back show-language :show-search="false"><button[^>]+class="menu-button"/);
  assert.doesNotMatch(story,/fetchNovelChapter/);
  assert.match(reader,/<AppHeader back :show-search="false"><button[^>]+class="menu-button"/);
  assert.match(reader,/fetchNovelChapter/);
});

test("language menu is limited to home and story introduction", async () => {
  const sources=await Promise.all(["HomeView.vue","StoryView.vue","SearchView.vue","StoryListView.vue"].map((name)=>readFile(new URL(`../src/views/${name}`,import.meta.url),"utf8")));
  assert.match(sources[0],/<AppHeader show-language\s*\/>/);
  assert.match(sources[1],/<AppHeader back show-language/);
  assert.doesNotMatch(sources[2],/show-language/);
  assert.doesNotMatch(sources[3],/show-language/);
});

test("chapter navigation keeps previous left and continue reading right", async () => {
  const css=await readFile(new URL("../src/styles.css",import.meta.url),"utf8");
  const actions=css.match(/\.chapter-actions\s*\{([^}]+)\}/)?.[1]||"";
  assert.match(actions,/justify-content:\s*space-between/);
  assert.match(css,/\.chapter-actions \.primary-button\s*\{[^}]*margin-left:\s*auto/);
});

test("reader renders chapter content before restoring saved scroll position", async () => {
  // 先移除加载态再等待 DOM 更新，保存的深层阅读位置才不会被短页面截断为顶部。
  const reader=await readFile(new URL("../src/views/ReaderView.vue",import.meta.url),"utf8");
  const renderIndex=reader.indexOf("loading.value=false;",reader.indexOf("story.value=storyData.story"));
  const restoreIndex=reader.indexOf("await nextTick()");
  assert.notEqual(renderIndex,-1);
  assert.ok(renderIndex<restoreIndex);
});

test("active carousel indicator remains a circle", async () => {
  // 读取最终规则并比较宽高，避免激活状态再次被拉成长条。
  const css=await readFile(new URL("../src/styles.css",import.meta.url),"utf8");
  const base=css.match(/\.dots button\s*\{([^}]+)\}/)?.[1]||"";
  const active=css.match(/\.dots button\.active\s*\{([^}]+)\}/)?.[1]||"";
  const value=(rule,name)=>rule.match(new RegExp(`${name}\\s*:\\s*([^;]+)`))?.[1].trim();
  assert.equal(value(active,"width")||value(base,"width"),value(active,"height")||value(base,"height"));
});
