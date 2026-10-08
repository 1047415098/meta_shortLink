import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { canFinishStartupLoader, installStartupLoader, markNovelStartupReady, startupProgressAt } from "../src/lib/startupLoader.js";

test("startup loader reaches 90 percent in five seconds and reserves the final step for readiness", () => {
  assert.equal(startupProgressAt(0, false), 0);
  assert.equal(startupProgressAt(2500, false), 45);
  assert.equal(startupProgressAt(5000, false), 90);
  assert.equal(startupProgressAt(6500, false), 95);
  assert.equal(startupProgressAt(8000, false), 99);
  assert.equal(startupProgressAt(8000, true), 100);
  assert.equal(canFinishStartupLoader(7999, true), false);
  assert.equal(canFinishStartupLoader(8000, false), false);
  assert.equal(canFinishStartupLoader(8000, true), true);
});

test("startup loader closes when content becomes ready after the eight-second minimum", () => {
  let clock = 8000;
  let scheduledFrame;
  const listeners = new Map();
  const loader = { classList:{ values:new Set(), add(value){this.values.add(value);} }, setAttribute(){}, remove(){this.removed=true;} };
  const fill = { style:{} };
  const label = { textContent:"" };
  const documentRef = { getElementById:(id) => ({ "novel-startup-loader":loader, "novel-startup-loader-fill":fill, "novel-startup-loader-progress":label }[id]) };
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
  assert.equal(label.textContent, "99%");
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
  assert.match(index, /src="\/src\/startup\.js"/);
  assert.match(app, /let expectedInitialRouteName/);
  assert.match(app, /function markInitialViewReady\(routeName\)/);
  assert.match(app, /expectedInitialRouteName=entryRoute\?\.name\|\|""/);
  for (const [source, routeName] of [[home,"home"],[search,"search"],[list,"stories"],[story,"story"],[reader,"reader"]]) {
    assert.match(source, new RegExp(`markInitialViewReady\\("${routeName}"\\)`));
  }
});
