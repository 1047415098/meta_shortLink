import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { readBootstrap } from "../src/bootstrap.js";
import { entryRouteForBootstrap, shouldReportStartReading } from "../src/lib/entry.js";

test("initial entry waits for router readiness before the app starts", async () => {
  const main=await readFile(new URL("../src/main.js",import.meta.url),"utf8");
  // App 的挂载回调会触发入口跳转，必须在 router.isReady 之后执行。
  assert.match(main,/router\.isReady\(\)\.then\(\(\) => app\.mount\("#app"\)\)/);
});

test("countdown entry keeps the home page in browser history", async () => {
  const app=await readFile(new URL("../src/App.vue",import.meta.url),"utf8");
  // 免费小说唯一入口使用 push，让章节返回操作回到书架。
  assert.match(app,/if\(isNovelStartupEntryPath\(bootstrap\.link\)\)void router\.push\(entryRoute\)/);
  assert.match(app,/else void router\.replace\(entryRoute\)/);
});

test("bootstrap keeps entry chapter optional for legacy links", () => {
  const documentRef={ getElementById:()=>({ textContent:JSON.stringify({ link:{ code:"legacy",entry_story_slug:"wife" } }) }) };
  const bootstrap=readBootstrap(documentRef);
  assert.equal(bootstrap.link.entry_story_slug,"wife");
  assert.equal(bootstrap.link.entry_chapter_number,null);
  // Missing theme data from older links must retain the established countdown first screen.
  assert.equal(bootstrap.link.startup_theme,"countdown");
});

test("bootstrap ignores historical cover-wall theme values", () => {
  const documentRef={ getElementById:()=>({ textContent:JSON.stringify({ link:{ code:"campaign",startup_theme:"cover_wall",startup_tail_seconds:12 } }) }) };
  const bootstrap=readBootstrap(documentRef);
  assert.equal(bootstrap.link.startup_theme,"countdown");
  assert.equal(bootstrap.link.startup_tail_seconds,12);
});

test("bound entry chapter opens the reader directly from campaign home", () => {
  const route=entryRouteForBootstrap(
    { code:"buyer-a",entry_story_slug:"wife",entry_chapter_number:3 },
    { name:"home",query:{ lang:"ja",utm_source:"tiktok" } },
  );
  assert.deepEqual(route,{
    name:"reader",
    params:{ code:"buyer-a",slug:"wife",chapter:3 },
    query:{ lang:"ja",utm_source:"tiktok" },
  });
});

test("legacy story binding still opens the introduction page", () => {
  assert.deepEqual(
    entryRouteForBootstrap(
      { code:"legacy",entry_story_slug:"wife",entry_chapter_number:null },
      { name:"home",query:{ lang:"en" } },
    ),
    { name:"story",params:{ code:"legacy",slug:"wife" },query:{ lang:"en" } },
  );
  assert.equal(entryRouteForBootstrap({ code:"legacy",entry_story_slug:"wife",entry_chapter_number:2 },{ name:"reader",query:{} }),null);
});

test("StartReading follows the bound chapter and legacy links use the first readable chapter", () => {
  assert.equal(shouldReportStartReading({ entryChapterNumber:3,currentChapterNumber:3,firstReadableChapterNumber:1 }),true);
  assert.equal(shouldReportStartReading({ entryChapterNumber:3,currentChapterNumber:1,firstReadableChapterNumber:1 }),false);
  assert.equal(shouldReportStartReading({ entryChapterNumber:null,currentChapterNumber:3,firstReadableChapterNumber:3 }),true);
  assert.equal(shouldReportStartReading({ entryChapterNumber:null,currentChapterNumber:1,firstReadableChapterNumber:3 }),false);
});
