import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { reportDateShortcuts } from "../src/utils/reportDateShortcuts.js";

test("all report calendar shortcuts offer today, seven days and thirty days", () => {
  const shortcuts = reportDateShortcuts(() => "Etc/GMT+8");
  assert.deepEqual(
    shortcuts.map((shortcut) => shortcut.text),
    ["今天", "最近 7 天", "最近 30 天"],
  );

  // Inclusive ranges cover 7 and 30 calendar days respectively.
  const [sevenStart, sevenEnd] = shortcuts[1].value();
  const [thirtyStart, thirtyEnd] = shortcuts[2].value();
  assert.equal((sevenEnd - sevenStart) / 86_400_000, 6);
  assert.equal((thirtyEnd - thirtyStart) / 86_400_000, 29);
});

test("every operator statistics date picker uses the shared shortcuts", async () => {
  const source = (name) =>
    readFile(new URL(`../src/${name}`, import.meta.url), "utf8");
  const views = await Promise.all([
    source("components/AnalyticsFilter.vue"),
    source("views/LinkStatsView.vue"),
    source("views/NovelLinkStatsView.vue"),
    source("views/AudioNovelLinkStatsView.vue"),
    source("views/RequestLogView.vue"),
  ]);
  for (const view of views) {
    assert.match(view, /:shortcuts="dateShortcuts"/);
    assert.match(view, /reportDateShortcuts/);
  }
});
