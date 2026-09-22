import test from "node:test";
import assert from "node:assert/strict";
import {
  listNovels,
  createNovel,
  listChapters,
  createChapter,
  updateChapter,
  novelPayload,
  chapterPayload,
} from "../src/api/novels.js";
import viteConfig from "../vite.config.js";

test("novel clients whitelist payloads and use chapter endpoints", async (t) => {
  const calls = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    return new Response(JSON.stringify({ items: [] }), { status: 200, headers: { "Content-Type": "application/json" } });
  };
  t.after(() => { globalThis.fetch = originalFetch; });
  assert.deepEqual(novelPayload({ title:"Story", author:"Nine", sort_order:3, ignored:true }).ignored, undefined);
  assert.equal(novelPayload({ cover_path:"/novel-uploads/cover.png" }).cover_path, "/novel-uploads/cover.png");
  assert.deepEqual(chapterPayload({ chapter_number:2, title:"Two", body_markdown:"Body", enabled:true, ignored:true }).ignored, undefined);
  await listNovels({ q:"glass" });
  await createNovel({ title:"Story" });
  await listChapters(7);
  await createChapter(7, { chapter_number:1 });
  await updateChapter(7, 3, { chapter_number:2 });
  assert.deepEqual(calls.map(({url})=>url), [
    "/api/v1/novels?q=glass",
    "/api/v1/novels",
    "/api/v1/novels/7/chapters",
    "/api/v1/novels/7/chapters",
    "/api/v1/novels/7/chapters/3",
  ]);
});

test("admin development server proxies persisted novel covers", () => {
  assert.equal(viteConfig.server.proxy["/novel-uploads"], "http://127.0.0.1:8080");
});
