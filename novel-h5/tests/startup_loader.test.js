import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { canFinishStartupLoader, installStartupLoader, markNovelStartupReady, startupCoverPath, startupProgressAt } from "../src/lib/startupLoader.js";

test("startup loader accepts persisted local covers and the narrowly trusted historical CDN", () => {
  assert.equal(startupCoverPath({ startup_cover_path: "/novel-uploads/0123456789abcdef0123456789abcdef.webp" }), "/novel-uploads/0123456789abcdef0123456789abcdef.webp");
  assert.equal(startupCoverPath({ startup_cover_path: "https://cdn.overseas-new-media.com/xiaoyao-writer/prod/content/cover/EJJjSSQy89w9JEe7tWjCoXwWPmbcL6KQ.jpg" }), "https://cdn.overseas-new-media.com/xiaoyao-writer/prod/content/cover/EJJjSSQy89w9JEe7tWjCoXwWPmbcL6KQ.jpg");
  assert.equal(startupCoverPath({ startup_cover_path: "https://example.com/cover.webp" }), "");
  assert.equal(startupCoverPath({ startup_cover_path: "https://cdn.overseas-new-media.com.evil.example/xiaoyao-writer/prod/content/cover/cover.webp" }), "");
  assert.equal(startupCoverPath({ startup_cover_path: "/novel-uploads/../private.webp" }), "");
});

test("startup loader reaches 90 percent in three seconds and finishes over the next seven", () => {
  assert.equal(startupProgressAt(0, false), 0);
  assert.equal(startupProgressAt(1500, false), 45);
  assert.equal(startupProgressAt(3000, false), 90);
  assert.equal(startupProgressAt(6500, false), 95);
  assert.equal(startupProgressAt(10000, false), 100);
  assert.equal(startupProgressAt(10000, true), 100);
  assert.equal(canFinishStartupLoader(9999, true), false);
  assert.equal(canFinishStartupLoader(10000, false), false);
  assert.equal(canFinishStartupLoader(10000, true), true);
});

test("startup loader closes when content becomes ready after the ten-second minimum", () => {
  let clock = 10000;
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
  clock = 10001;
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
  assert.match(index, /id="novel-startup-loader-cover"/);
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
