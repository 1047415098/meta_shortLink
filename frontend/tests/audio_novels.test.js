import test from "node:test";
import assert from "node:assert/strict";
import {
  createAudioNovel,
  listAudioNovels,
  normalizeAudioNovelSlug,
  audioNovelPayload,
  previewAudioNovelMarkdown,
  uploadAudioNovelCover,
} from "../src/api/audioNovels.js";

test("audio novel slug normalization keeps lowercase words and single dashes", () => {
  assert.equal(
    normalizeAudioNovelSlug(" The  Glass__Orchard! "),
    "the-glass-orchard",
  );
});

test("audio novel slug normalization preserves Chinese characters", () => {
  assert.equal(normalizeAudioNovelSlug(" 剑 与 Magic！ "), "剑-与-magic");
});

test("audio novel payload never includes author or unimplemented audio fields", () => {
  const payload = audioNovelPayload({
    title: "Story",
    author: "Hidden",
    audio_url: "https://example.com/not-supported.mp3",
    slug: "story",
    enabled: true,
  });
  assert.equal(payload.author, undefined);
  assert.equal(payload.audio_url, undefined);
  assert.equal(payload.title, "Story");
});

test("audio novel client uses the renamed admin endpoints", async (t) => {
  const calls = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    return new Response(
      JSON.stringify({ items: [], body_html: "", path: "" }),
      {
        status: 200,
        headers: { "Content-Type": "application/json" },
      },
    );
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  await listAudioNovels({ q: "glass" });
  await createAudioNovel({ title: "Story" });
  await previewAudioNovelMarkdown("# Story");
  await uploadAudioNovelCover(new Blob(["cover"], { type: "image/png" }));

  assert.deepEqual(
    calls.map(({ url }) => url),
    [
      "/api/v1/audio-novels?q=glass",
      "/api/v1/audio-novels",
      "/api/v1/audio-novels/preview",
      "/api/v1/audio-novel-covers",
    ],
  );
});
