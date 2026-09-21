import assert from "node:assert/strict";
import test from "node:test";
import { clampReadingSize } from "../src/lib/reading.js";

test("reading size stays in the accessible range", () => {
  assert.equal(clampReadingSize(11), 14);
  assert.equal(clampReadingSize(30), 24);
  assert.equal(clampReadingSize("bad"), 18);
});
