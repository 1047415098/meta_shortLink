import assert from "node:assert/strict";
import test from "node:test";
import { audioDetailPath, audioListPath, storyPath } from "../src/lib/routes.js";

test("story URLs preserve the short-link code", () => {
  assert.equal(storyPath("hello", "the-glass-orchard"), "/audio-novel/hello/stories/the-glass-orchard");
});

test("audio fiction URLs preserve code and encode the slug", () => {
  assert.equal(audioListPath("hello world"), "/audio-novel/hello%20world/audio");
  assert.equal(audioDetailPath("hello world", "glass/orchard"), "/audio-novel/hello%20world/audio/glass%2Forchard");
});
