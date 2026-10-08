import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { STARTUP_DEFAULT_TAIL_MS, canFinishStartupLoader, fetchStartupExcerpt, installStartupLoader, markNovelStartupReady, startupCoverPath, startupExcerpt, startupProgressAt, startupStoryRoute, startupTailMs } from "../src/lib/startupLoader.js";

test("startup loader accepts persisted local covers and the narrowly trusted historical CDN", () => {
  assert.equal(startupCoverPath({ startup_cover_path: "/novel-uploads/0123456789abcdef0123456789abcdef.webp" }), "/novel-uploads/0123456789abcdef0123456789abcdef.webp");
  assert.equal(startupCoverPath({ startup_cover_path: "https://cdn.overseas-new-media.com/xiaoyao-writer/prod/content/cover/EJJjSSQy89w9JEe7tWjCoXwWPmbcL6KQ.jpg" }), "https://cdn.overseas-new-media.com/xiaoyao-writer/prod/content/cover/EJJjSSQy89w9JEe7tWjCoXwWPmbcL6KQ.jpg");
  assert.equal(startupCoverPath({ startup_cover_path: "https://example.com/cover.webp" }), "");
  assert.equal(startupCoverPath({ startup_cover_path: "https://cdn.overseas-new-media.com.evil.example/xiaoyao-writer/prod/content/cover/cover.webp" }), "");
  assert.equal(startupCoverPath({ startup_cover_path: "/novel-uploads/../private.webp" }), "");
});

test("startup loader normalizes only a short plain-text chapter excerpt", () => {
  assert.equal(startupExcerpt({ startup_excerpt: "  First\nchapter.  " }), "First chapter.");
  assert.equal(startupExcerpt({ startup_excerpt: 123 }), "");
  assert.equal(Array.from(startupExcerpt({ startup_excerpt: "文".repeat(1901) })).length, 1901);
});

test("startup loader recognizes only direct novel detail routes", () => {
  assert.deepEqual(startupStoryRoute("/novel/hello/stories/divorced-my-ex-husband-wants-me-back"), { code:"hello", slug:"divorced-my-ex-husband-wants-me-back" });
  assert.deepEqual(startupStoryRoute("/novel/hello/stories/a%20story/chapters/2"), { code:"hello", slug:"a story" });
  assert.equal(startupStoryRoute("/novel/hello/search"), null);
  assert.equal(startupStoryRoute("/outside/hello/stories/a"), null);
});

test("startup loader fetches a short first-chapter preview when the server did not inject one", async () => {
  const calls=[];
  const request=async (url, options) => {
    calls.push({ url, headers:options.headers });
    if (url.endsWith("/stories/divorced-my-ex-husband-wants-me-back")) return { ok:true, json:async()=>({ chapters:[{ chapter_number:2 }] }) };
    return { ok:true, json:async()=>({ chapter:{ body_html:"<p>First <strong>chapter</strong> &amp; opening.</p>" } }) };
  };
  const excerpt=await fetchStartupExcerpt("/novel/hello/stories/divorced-my-ex-husband-wants-me-back", { locale:"ja", request });
  assert.equal(excerpt, "First chapter & opening.");
  assert.deepEqual(calls, [
    { url:"/novel-api/hello/stories/divorced-my-ex-husband-wants-me-back", headers:{ Accept:"application/json", "X-Novel-Language":"ja" } },
    { url:"/novel-api/hello/stories/divorced-my-ex-husband-wants-me-back/chapters/2", headers:{ Accept:"application/json", "X-Novel-Language":"ja" } },
  ]);
});

test("startup loader paints the development preview behind its progress card", async () => {
  const loader={ classList:{ values:new Set(), add(value){this.values.add(value);} }, setAttribute(){}, remove(){} };
  const fill={ style:{} },excerptLayer={ textContent:"" };
  const documentRef={ getElementById:(id)=>({ "novel-startup-loader":loader, "novel-startup-loader-fill":fill, "novel-startup-loader-text":excerptLayer }[id]) };
  const windowRef={
    __novelStartupStartedAt:1,
    location:{ pathname:"/novel/hello/stories/divorced-my-ex-husband-wants-me-back" },
    fetch:async (url)=>url.endsWith("/stories/divorced-my-ex-husband-wants-me-back")
      ? { ok:true, json:async()=>({ chapters:[{ chapter_number:1 }] }) }
      : { ok:true, json:async()=>({ chapter:{ body_html:"<p>Visible first chapter.</p>" } }) },
    requestAnimationFrame:()=>1,cancelAnimationFrame(){},setTimeout:()=>1,addEventListener(){},removeEventListener(){},dispatchEvent(){},
  };
  installStartupLoader({ documentRef, windowRef, now:()=>0 });
  // 两次 API 解析和一次 then 回调均在微任务队列中完成。
  await new Promise((resolve)=>setImmediate(resolve));
  assert.equal(excerptLayer.textContent, "Visible first chapter.");
  assert.equal(loader.classList.values.has("has-excerpt"), true);
});

test("startup loader reuses the server-injected excerpt without a duplicate request", async () => {
  const loader={ classList:{ values:new Set(), add(value){this.values.add(value);} }, setAttribute(){}, remove(){} };
  const fill={ style:{} },excerptLayer={ textContent:"" };
  const documentRef={ getElementById:(id)=>({
    "novel-startup-loader":loader,
    "novel-startup-loader-fill":fill,
    "novel-startup-loader-text":excerptLayer,
    "novel-h5-data":{ textContent:JSON.stringify({ startup_excerpt:"Server preview." }) },
  }[id]) };
  let requests=0;
  const windowRef={ __novelStartupStartedAt:1,location:{ pathname:"/novel/hello/stories/story" },fetch:async()=>{requests+=1;return { ok:false };},requestAnimationFrame:()=>1,cancelAnimationFrame(){},setTimeout:()=>1,addEventListener(){},removeEventListener(){},dispatchEvent(){} };
  installStartupLoader({ documentRef, windowRef, now:()=>0 });
  await new Promise((resolve)=>setImmediate(resolve));
  assert.equal(requests, 0);
  assert.equal(excerptLayer.textContent, "Server preview.");
});

test("startup loader keeps the first three seconds fixed and reads each link's final-ten-percent duration", () => {
  assert.equal(startupTailMs({ link:{ startup_tail_seconds:12 } }), 12000);
  assert.equal(startupTailMs({ link:{ startup_tail_seconds:0 } }), STARTUP_DEFAULT_TAIL_MS);
  assert.equal(startupTailMs({}), STARTUP_DEFAULT_TAIL_MS);
  assert.equal(startupProgressAt(0), 0);
  assert.equal(startupProgressAt(1500), 45);
  assert.equal(startupProgressAt(3000), 90);
  assert.equal(startupProgressAt(5500), 95);
  assert.equal(startupProgressAt(8000), 100);
  // A two-second tail proves the campaign setting changes only the final visual segment.
  assert.equal(startupProgressAt(4000, 2000), 95);
  assert.equal(canFinishStartupLoader(7999, true), false);
  assert.equal(canFinishStartupLoader(8000, false), false);
  assert.equal(canFinishStartupLoader(8000, true), true);
});

test("startup loader closes when content becomes ready after the configured default minimum", () => {
  let clock = 8000;
  let scheduledFrame;
  const listeners = new Map();
  const loader = { classList:{ values:new Set(), add(value){this.values.add(value);} }, setAttribute(){}, remove(){this.removed=true;} };
  const fill = { style:{} };
  const documentRef = { getElementById:(id) => ({ "novel-startup-loader":loader, "novel-startup-loader-fill":fill }[id]) };
  const windowRef = {
    __novelStartupStartedAt:1,
    requestAnimationFrame:(callback) => { scheduledFrame=callback; return 1; },
    cancelAnimationFrame(){},
    setTimeout:(callback) => { callback(); return 1; },
    addEventListener:(name,callback) => listeners.set(name,callback),
    removeEventListener:()=>{},
    dispatchEvent:(event) => listeners.get(event.type)?.(),
  };

  installStartupLoader({ documentRef, windowRef, now:()=>clock });
  clock = 8001;
  scheduledFrame();
  assert.equal(fill.style.width, "100%");
  markNovelStartupReady(windowRef);
  assert.equal(loader.classList.values.has("is-leaving"), true);
  assert.equal(loader.removed, true);
});

test("every initial novel route notifies the one-time loader after its first render", async () => {
  const [index, app, home, search, list, story, reader] = await Promise.all([
    readFile(new URL("../index.html", import.meta.url), "utf8"),
    readFile(new URL("../src/App.vue", import.meta.url), "utf8"),
    readFile(new URL("../src/views/HomeView.vue", import.meta.url), "utf8"),
    readFile(new URL("../src/views/SearchView.vue", import.meta.url), "utf8"),
    readFile(new URL("../src/views/StoryListView.vue", import.meta.url), "utf8"),
    readFile(new URL("../src/views/StoryView.vue", import.meta.url), "utf8"),
    readFile(new URL("../src/views/ReaderView.vue", import.meta.url), "utf8"),
  ]);

  assert.match(index, /id="novel-startup-loader"/);
  // 标签页图标与页面头部使用同一品牌资源，ICO 不可用时回退 PNG。
  assert.match(index, /rel="icon" type="image\/x-icon" href="\/src\/assets\/logo\.ico"/);
  assert.match(index, /rel="icon" type="image\/png" href="\/src\/assets\/logo\.png"/);
  assert.match(index, /id="novel-startup-loader-cover"/);
  assert.match(index, /id="novel-startup-loader-text"/);
  assert.match(index, /novel-startup-loader__reader/);
  assert.match(index, /min-height:100dvh/);
  // 正文可作加载背景，但居中的原始反馈卡必须持续存在，避免首屏交互样式回退。
  assert.match(index, /place-items:center/);
  assert.match(index, /width:min\(320px,100%\)/);
  assert.match(index, /height:9px/);
  // The visual progress bar is intentionally unlabeled; aria-valuenow remains for accessibility.
  assert.doesNotMatch(index, /novel-startup-loader-progress/);
  assert.match(index, /src="\/src\/startup\.js"/);
  assert.match(app, /let expectedInitialRouteName/);
  assert.match(app, /function markInitialViewReady\(routeName\)/);
  assert.match(app, /expectedInitialRouteName=entryRoute\?\.name\|\|""/);
  for (const [source, routeName] of [[home,"home"],[search,"search"],[list,"stories"],[story,"story"],[reader,"reader"]]) {
    assert.match(source, new RegExp(`markInitialViewReady\\("${routeName}"\\)`));
  }
});
