import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { installMetaPixel } from "../src/lib/meta.js";
import { createVisibleTimeTracker, reportReadingTime, reportTimeSpent } from "../src/lib/timeSpent.js";

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
  assert.equal(await reportReadingTime({code:"wife-a",ticket:"signed",seconds:37,request:async(url,options)=>{calls.push([url,options]);return {ok:true};}}),true);
  assert.equal(calls[0][0],"/novel/wife-a/reading-time");
  assert.match(String(calls[0][1].body),/seconds=37/);
  assert.equal(calls[0][1].keepalive,true);
});

test("reader shows an explicit empty state when no chapters are enabled", async () => {
  const source=await readFile(new URL("../src/views/StoryView.vue",import.meta.url),"utf8");
  assert.match(source,/This story has no readable chapters yet/);
  assert.match(source,/v-if="chapters.length" id="reading"/);
});

test("reader header keeps search and contents in the same flex action row", async () => {
  // 回归保护：目录按钮必须进入通用头部布局，不能再绝对定位到搜索按钮上方。
  const header=await readFile(new URL("../src/components/AppHeader.vue",import.meta.url),"utf8");
  const story=await readFile(new URL("../src/views/StoryView.vue",import.meta.url),"utf8");
  assert.match(header,/<slot\s*\/>/);
  assert.match(story,/<AppHeader back><button[^>]+class="menu-button"/);
});

test("active carousel indicator remains a circle", async () => {
  // 读取最终规则并比较宽高，避免激活状态再次被拉成长条。
  const css=await readFile(new URL("../src/styles.css",import.meta.url),"utf8");
  const base=css.match(/\.dots button\s*\{([^}]+)\}/)?.[1]||"";
  const active=css.match(/\.dots button\.active\s*\{([^}]+)\}/)?.[1]||"";
  const value=(rule,name)=>rule.match(new RegExp(`${name}\\s*:\\s*([^;]+)`))?.[1].trim();
  assert.equal(value(active,"width")||value(base,"width"),value(active,"height")||value(base,"height"));
});
