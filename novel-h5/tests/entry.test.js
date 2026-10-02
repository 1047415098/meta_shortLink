import test from "node:test";
import assert from "node:assert/strict";
import { readBootstrap } from "../src/bootstrap.js";
import { entryRouteForBootstrap, shouldReportStartReading } from "../src/lib/entry.js";

test("bootstrap keeps entry chapter optional for legacy links", () => {
  const documentRef={ getElementById:()=>({ textContent:JSON.stringify({ link:{ code:"legacy",entry_story_slug:"wife" } }) }) };
  const bootstrap=readBootstrap(documentRef);
  assert.equal(bootstrap.link.entry_story_slug,"wife");
  assert.equal(bootstrap.link.entry_chapter_number,null);
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
