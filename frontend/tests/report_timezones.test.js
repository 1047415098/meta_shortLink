import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const source = (name) =>
  readFile(new URL(`../src/${name}`, import.meta.url), "utf8");

test("operator reports offer only fixed UTC-8 and UTC+8", async () => {
  const [timezones, shared, shortLink, novel, audio, logs] = await Promise.all([
    source("constants/reportTimezones.js"),
    source("components/AnalyticsFilter.vue"),
    source("views/LinkStatsView.vue"),
    source("views/NovelLinkStatsView.vue"),
    source("views/AudioNovelLinkStatsView.vue"),
    source("views/RequestLogView.vue"),
  ]);

  // IANA Etc/GMT names reverse their signs, so GMT+8 is the fixed UTC-8 default.
  assert.match(timezones, /DEFAULT_REPORT_TIMEZONE = "Etc\/GMT\+8"/);
  assert.match(timezones, /label: "UTC-8（固定）", value: "Etc\/GMT\+8"/);
  assert.match(timezones, /label: "UTC\+8（固定）", value: "Etc\/GMT-8"/);
  assert.match(shared, /REPORT_TIMEZONES/);
  for (const view of [shortLink, novel, audio, logs]) {
    assert.match(view, /DEFAULT_REPORT_TIMEZONE/);
    assert.doesNotMatch(
      view,
      /Asia\/Shanghai|America\/New_York|America\/Los_Angeles/,
    );
  }
});
