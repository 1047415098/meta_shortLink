import test from "node:test";
import assert from "node:assert/strict";
import { clearProgress, readProgress, saveProgress } from "../src/lib/progress.js";

function storage() { const values = new Map(); return { getItem:k=>values.get(k) ?? null, setItem:(k,v)=>values.set(k,v), removeItem:k=>values.delete(k), values }; }
test("reading progress saves, validates and removes values", () => {
  const store = storage(); saveProgress("story", 3, -20, store, 123); assert.deepEqual(readProgress("story", [1,3], store), { chapterNumber:3, scrollY:0, updatedAt:123 }); clearProgress("story", store); assert.equal(readProgress("story", [1], store), null);
});
test("reading progress falls back safely for corrupt or removed chapters", () => {
  const store = storage(); store.setItem("novel-progress:story", "bad"); assert.equal(readProgress("story", [1], store), null);
  saveProgress("story", 9, 40, store, 123); assert.deepEqual(readProgress("story", [1,2], store), { chapterNumber:1, scrollY:0, updatedAt:123 });
  assert.equal(readProgress("other", [1], store), null);
});
