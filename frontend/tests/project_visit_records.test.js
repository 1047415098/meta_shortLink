import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const read = (path) => readFile(new URL(path, import.meta.url), "utf8");

test("every project statistics page includes the shared complete visit list", async () => {
  const pages = {
    "../src/views/LinkStatsView.vue": "short_link",
    "../src/views/AudioNovelLinkStatsView.vue": "audio_novel",
    "../src/views/NovelLinkStatsView.vue": "novel",
    "../src/views/CoverLinkStatsView.vue": "cover",
  };
  for (const [path, surface] of Object.entries(pages)) {
    const source = await read(path);
    // Every project remains isolated by both link ID and its frozen visit surface.
    assert.match(source, /ProjectVisitRecords/);
    assert.match(source, new RegExp(`surface="${surface}"`));
    assert.match(source, /:request-key="recordsVersion"/);
  }
  // Project behavior rows are merged into the shared table instead of being rendered twice.
  for (const path of [
    "../src/views/AudioNovelLinkStatsView.vue",
    "../src/views/NovelLinkStatsView.vue",
    "../src/views/CoverLinkStatsView.vue",
  ]) {
    const source = await read(path);
    assert.doesNotMatch(source, /播放行为明细|阅读行为明细|封面行为明细/);
  }
});

test("shared project visit list matches the system visit columns and opens detail", async () => {
  const source = await read("../src/components/ProjectVisitRecords.vue");
  for (const label of [
    "访问时间",
    "短码",
    "统计口径",
    "访问行为",
    "入口",
    "设备",
    "地区",
    "来源 / 广告",
    "Campaign / Ad Group",
    "Creative / Placement",
    "访客标识",
    "流量分类",
    "操作",
  ])
    assert.match(source, new RegExp(label.replace("/", "\\/")));
  // Filters remain in a JSON request body through the existing shared client.
  assert.match(source, /getVisits\(\{/);
  assert.match(source, /link_id: String\(props\.linkId\)/);
  assert.match(source, /traffic_scope: trafficScope\.value/);
  assert.match(source, /有效访问/);
  assert.match(source, /全部记录/);
  assert.match(source, /异常流量/);
  assert.match(source, /visible_seconds/);
  assert.match(source, /playback_seconds/);
  assert.match(source, /entry_chapter_number/);
  assert.match(source, /detailDialog\?\.open\(row\.id\)/);
});

test("shared visit detail exposes all diagnostic and attribution sections", async () => {
  const source = await read("../src/components/VisitDetailDialog.vue");
  for (const heading of [
    "基础访问",
    "设备与地区",
    "原始请求信息",
    "广告归因",
    "内容与用户行为",
    "广告平台快照",
    "Meta / TikTok 回传记录",
  ])
    assert.match(source, new RegExp(heading.replace("/", "\\/")));
  assert.match(source, /await getVisit\(id\)/);
  assert.match(source, /defineExpose\(\{ open \}\)/);
});
