import assert from "node:assert/strict";
import test from "node:test";
import { storyPath } from "../src/lib/routes.js";

test("story URLs preserve the short-link code", () => {
  assert.equal(storyPath("hello", "the-glass-orchard"), "/audio-novel/hello/stories/the-glass-orchard");
});
