import test from "node:test";
import assert from "node:assert/strict";
import { chapterNavigation, progressChapter } from "../src/lib/reader.js";
const chapters = [{ chapter_number:1 }, { chapter_number:3 }, { chapter_number:4 }];
test("chapter navigation returns previous and next chapters", () => { assert.deepEqual(chapterNavigation(chapters, 3), { previous:1, next:4 }); assert.deepEqual(chapterNavigation(chapters, 4), { previous:3, next:null }); });
test("reading progress picks an existing chapter only", () => { assert.equal(progressChapter(chapters, 3), 3); assert.equal(progressChapter(chapters, 2), 1); });
