import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { compile } from "@vue/compiler-dom";
import { parse } from "@vue/compiler-sfc";
import { createSSRApp } from "vue";
import * as Vue from "vue";
import { renderToString } from "@vue/server-renderer";
import { formatAudioSize } from "../src/lib/api.js";

async function renderStory(story) {
  const source = await readFile(new URL("../src/views/StoryDetailView.vue", import.meta.url), "utf8");
  const { descriptor } = parse(source);
  const { code } = compile(descriptor.template.content, { mode: "function" });
  const renderTemplate = new Function("Vue", code)(Vue);
  const state = {
    bootstrap: { link: { code: "hello" } }, loading: false, error: "", story, related: [],
    size: 18, immersive: false, formatAudioSize, setSize() {}, setImmersive() {}, load() {},
  };
  // 直接执行真实 Vue 模板，避免只检查源代码字符串而漏掉条件渲染错误。
  const app = createSSRApp({ render: () => renderTemplate(state, []) });
  app.component("RouterLink", { props: ["to"], template: "<a><slot /></a>" });
  app.component("WhatsAppAction", { template: "<div></div>" });
  return renderToString(app);
}

test("story detail renders an MP3 player only when the story has audio", async () => {
  const base = {
    category: "Fantasy", title: "Sous Lumière Aigre", published_at: "2026-09-17",
    body_html: "<p>Story</p>", slug: "sous-lumiere-aigre",
  };
  const withAudio = await renderStory({
    ...base,
    audio_path: "/audio-novel-audio/0123456789abcdef0123456789abcdef.mp3",
    audio_duration: "32:05",
    audio_size_bytes: 23_100_419,
  });
  const withoutAudio = await renderStory({ ...base, audio_path: "", audio_duration: "", audio_size_bytes: 0 });

  assert.match(withAudio, /<audio[^>]+controls[^>]+preload="metadata"/);
  assert.match(withAudio, /0123456789abcdef0123456789abcdef\.mp3/);
  assert.doesNotMatch(withAudio, /autoplay/);
  assert.doesNotMatch(withoutAudio, /<audio/);
});

test("public audio pages use safe native players and complete navigation", async () => {
  const list = await readFile(new URL("../src/views/AudioListView.vue", import.meta.url), "utf8");
  const detail = await readFile(new URL("../src/views/AudioDetailView.vue", import.meta.url), "utf8");
  const story = await readFile(new URL("../src/views/StoryDetailView.vue", import.meta.url), "utf8");
  const header = await readFile(new URL("../src/components/SiteHeader.vue", import.meta.url), "utf8");
  const combined = `${list}\n${detail}`;
  assert.match(combined, /preload="metadata"/);
  assert.doesNotMatch(combined, /autoplay/);
  assert.match(combined, /Download/);
  assert.match(detail, /Read Story/);
  assert.match(detail, /This Podcast is unavailable/);
  assert.match(detail, /name:\s*["']story["']/);
  assert.match(story, /Podcast/);
  assert.match(story, /story\.audio_path/);
  assert.match(header, /Audio Fiction/);
});
