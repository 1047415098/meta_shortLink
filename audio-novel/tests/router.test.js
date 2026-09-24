import assert from "node:assert/strict";
import test from "node:test";
import { audioDetailPath, audioEntryLocation, audioListPath, storyPath } from "../src/lib/routes.js";

test("story URLs preserve the short-link code", () => {
  assert.equal(storyPath("hello", "the-glass-orchard"), "/audio-novel/hello/stories/the-glass-orchard");
});

test("audio fiction URLs preserve code and encode the slug", () => {
  assert.equal(audioListPath("hello world"), "/audio-novel/hello%20world/audio");
  assert.equal(audioDetailPath("hello world", "glass/orchard"), "/audio-novel/hello%20world/audio/glass%2Forchard");
});

test("a bound audio campaign replaces only the root route and preserves attribution query", async () => {
  const bootstrap = { link: { code: "hello world" }, entry_audio_slug: "glass/orchard" };
  assert.deepEqual(audioEntryLocation(bootstrap, { name: "home", query: { ttclid: "click-1", campaign_id: "__CAMPAIGN_ID__" } }), {
    path: "/audio-novel/hello%20world/audio/glass%2Forchard",
    query: { ttclid: "click-1", campaign_id: "__CAMPAIGN_ID__" },
  });
  assert.equal(audioEntryLocation({ link: { code: "hello" } }, { name: "home", query: {} }), null);
  assert.equal(audioEntryLocation(bootstrap, { name: "audio-detail", query: {} }), null);
  assert.equal(audioEntryLocation({ ...bootstrap, error: { status: 410 } }, { name: "home", query: {} }), null);

  const source = await import("node:fs/promises").then(({ readFile }) => readFile(new URL("../src/App.vue", import.meta.url), "utf8"));
  assert.match(source, /router\.replace\(entryLocation\)/);
  assert.doesNotMatch(source, /location\.replace/);
});
