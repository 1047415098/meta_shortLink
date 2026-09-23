import test from "node:test";
import assert from "node:assert/strict";
import { chapterPath, homePath, searchPath, storiesPath, storyPath } from "../src/lib/routes.js";

test("novel paths preserve the literal short code", () => {
  assert.equal(homePath("Ab_C"), "/novel/Ab_C");
  assert.equal(searchPath("Ab_C"), "/novel/Ab_C/search");
  assert.equal(storiesPath("Ab_C"), "/novel/Ab_C/stories");
  assert.equal(storyPath("Ab_C", "red moon"), "/novel/Ab_C/stories/red%20moon");
  assert.equal(chapterPath("Ab_C", "red moon", 2), "/novel/Ab_C/stories/red%20moon/chapters/2");
});
