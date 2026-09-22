import test from "node:test";
import assert from "node:assert/strict";
import { fetchNovelChapter, fetchNovelHome, fetchNovelList, fetchNovelStory } from "../src/lib/api.js";

function response(data, ok = true, status = 200) { return { ok, status, json: async () => data }; }
test("novel API keeps code and encodes search and pagination", async () => {
  const urls = [], request = async (url) => { urls.push(url); return response(url.includes("/home") ? { featured:null, ranking:[], items:[] } : url.includes("chapters") ? { chapter:{}, previous:null, next:null } : url.endsWith("/a%20story") ? { story:{}, chapters:[], related:[] } : { items:[], page:2, pages:2 }); };
  await fetchNovelHome("Ab_C", request); await fetchNovelList("Ab_C", { q:"red moon", page:2, pageSize:12 }, request); await fetchNovelStory("Ab_C", "a story", request); await fetchNovelChapter("Ab_C", "a story", 3, request);
  assert.deepEqual(urls, ["/novel-api/Ab_C/home", "/novel-api/Ab_C/stories?q=red+moon&page=2&page_size=12", "/novel-api/Ab_C/stories/a%20story", "/novel-api/Ab_C/stories/a%20story/chapters/3"]);
});
test("novel API rejects incomplete responses and maps unavailable links", async () => {
  await assert.rejects(() => fetchNovelHome("x", async () => response({})), /incomplete/i);
  await assert.rejects(() => fetchNovelHome("x", async () => response({}, false, 410)), /unavailable/i);
});
