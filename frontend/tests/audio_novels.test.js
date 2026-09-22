import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import {
  createAudioNovel,
  listAudioNovels,
  normalizeAudioNovelSlug,
  audioNovelPayload,
  previewAudioNovelMarkdown,
  uploadAudioNovelCover,
  uploadAudioNovelAudio,
  removeAudioNovelAudio,
  formatAudioDuration,
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

test("audio novel payload whitelists complete podcast metadata", () => {
  const payload = audioNovelPayload({
    title: "Story",
    author: "Hidden",
    audio_url: "https://example.com/not-supported.mp3",
    slug: "story",
    enabled: true,
    audio_path: "/audio-novel-audio/0123456789abcdef0123456789abcdef.mp3",
    audio_duration: "32:05",
    audio_size_bytes: 23100419,
  });
  assert.equal(payload.author, undefined);
  assert.equal(payload.audio_url, undefined);
  assert.equal(payload.title, "Story");
  assert.equal(
    payload.audio_path,
    "/audio-novel-audio/0123456789abcdef0123456789abcdef.mp3",
  );
  assert.equal(payload.audio_duration, "32:05");
  assert.equal(payload.audio_size_bytes, 23100419);
  assert.deepEqual(Object.keys(payload), [
    "title",
    "slug",
    "category",
    "excerpt",
    "body_markdown",
    "cover_path",
    "audio_path",
    "audio_duration",
    "audio_size_bytes",
    "published_at",
    "enabled",
    "featured",
  ]);
});

test("audio duration formatting matches list and detail metadata", () => {
  assert.equal(formatAudioDuration(65), "01:05");
  assert.equal(formatAudioDuration(1925.8), "32:05");
  assert.equal(formatAudioDuration(3723), "1:02:03");
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
  await uploadAudioNovelAudio(new Blob(["ID3audio"], { type: "audio/mpeg" }));
  await removeAudioNovelAudio(42);

  assert.deepEqual(
    calls.map(({ url }) => url),
    [
      "/api/v1/audio-novels?q=glass",
      "/api/v1/audio-novels",
      "/api/v1/audio-novels/preview",
      "/api/v1/audio-novel-covers",
      "/api/v1/audio-novel-audio",
      "/api/v1/audio-novels/42/audio",
    ],
  );
  assert.equal(calls.at(-1).options.method, "DELETE");
});

test("audio novel admin renders safe podcast controls and list status", async () => {
  const form = await readFile(
    new URL("../src/views/AudioNovelFormView.vue", import.meta.url),
    "utf8",
  );
  const list = await readFile(
    new URL("../src/views/AudioNovelListView.vue", import.meta.url),
    "utf8",
  );
  assert.match(form, /accept="audio\/mpeg,\.mp3"/);
  assert.match(form, /preload="metadata"/);
  assert.doesNotMatch(form, /autoplay/);
  assert.match(form, /替换 MP3/);
  assert.match(form, /移除音频/);
  assert.match(list, /已上传/);
  assert.match(list, /无音频/);
  assert.doesNotMatch(list, /<audio/);
});
