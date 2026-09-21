import test from "node:test";
import assert from "node:assert/strict";
import { fetchAudioNovelHome, fetchAudioNovelList, fetchAudioNovelStory } from "../src/lib/api.js";

function response(body, ok = true, status = 200) { return { ok, status, json: async () => body }; }

test("audio novel API safely encodes code, slug and paging", async () => {
  const urls = [];
  const request = async (url) => { urls.push(url); return response(url.includes("/home") ? { featured: null } : url.includes("stories?") ? { items: [], page: 2, pages: 0, total: 0 } : { story: {}, related: [] }); };
  await fetchAudioNovelHome("hello world", request);
  await fetchAudioNovelList("hello world", 2, 6, request);
  await fetchAudioNovelStory("hello world", "glass/orchard", request);
  assert.deepEqual(urls, ["/audio-novel-api/hello%20world/home", "/audio-novel-api/hello%20world/stories?page=2&page_size=6", "/audio-novel-api/hello%20world/stories/glass%2Forchard"]);
});

test("audio novel API exposes readable failures", async () => {
  await assert.rejects(() => fetchAudioNovelHome("hello", async () => response({}, false, 410)), /unavailable/i);
});
