import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const source = (name) =>
  readFile(new URL(`../src/${name}`, import.meta.url), "utf8");

test("all operator reports initialise their date range to today", async () => {
  const [shared, shortLink, novel, audio] = await Promise.all([
    source("composables/useReport.js"),
    source("views/LinkStatsView.vue"),
    source("views/NovelLinkStatsView.vue"),
    source("views/AudioNovelLinkStatsView.vue"),
  ]);

  // Shared query reports and each independent campaign report must start/end today.
  for (const view of [shared, shortLink, novel, audio]) {
    assert.doesNotMatch(view, /setUTCDate\([^)]*- 6\)/);
  }
  assert.match(shared, /start:\s*end,/);
  assert.match(shortLink, /return \{ start: end, end, tz, ad_id: "" \}/);
  assert.match(novel, /start:\s*today\(\),/);
  assert.match(audio, /start:\s*today\(\),/);
});
