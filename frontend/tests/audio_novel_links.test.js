import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import {
  audioNovelLinkPayload,
  createAudioNovelLink,
  deleteAudioNovelLink,
  getAudioNovelLinkStats,
  listAudioNovelLinks,
  updateAudioNovelLink,
} from "../src/api/audioNovelLinks.js";
import { tiktokTemplateURL } from "../src/utils/tiktok.js";

test("audio novel campaign client uses isolated endpoints and a strict payload", async (t) => {
  const calls = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    return new Response(JSON.stringify({ items: [] }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const payload = audioNovelLinkPayload({
    name: "投手 A",
    code: "audio-a",
    audio_novel_id: "7",
    enabled: true,
    ad_platform: "meta",
    meta_connection_id: 3,
    meta_pixel_id: 4,
    tiktok_pixel_id: 9,
    attribution_mode: "bound",
    channel: "instagram",
    campaign_id: "must-not-leak",
    adset_id: "must-not-leak",
    ad_id: "must-not-leak",
  });
  assert.deepEqual(payload, {
    name: "投手 A",
    code: "audio-a",
    audio_novel_id: 7,
    enabled: true,
    ad_platform: "meta",
    meta_connection_id: 3,
    meta_pixel_id: 4,
    tiktok_pixel_id: null,
    time_spent_threshold: 10,
  });

  await listAudioNovelLinks(7);
  await createAudioNovelLink(payload);
  await updateAudioNovelLink(2, payload);
  await getAudioNovelLinkStats(2, { page: 1 });
  await deleteAudioNovelLink(2);
  assert.deepEqual(
    calls.map(({ url }) => url),
    [
      "/api/v1/audio-novel-links?audio_novel_id=7",
      "/api/v1/audio-novel-links",
      "/api/v1/audio-novel-links/2",
      "/api/v1/audio-novel-links/2/stats",
      "/api/v1/audio-novel-links/2",
    ],
  );
});

test("audio novel campaign payload selects exactly one platform", () => {
  const tiktok = audioNovelLinkPayload({
    audio_novel_id: 7,
    ad_platform: "tiktok",
    tiktok_pixel_id: 9,
    meta_connection_id: 3,
    meta_pixel_id: 4,
    time_spent_threshold: 25,
  });
  assert.equal(tiktok.tiktok_pixel_id, 9);
  assert.equal(tiktok.meta_connection_id, null);
  assert.equal(tiktok.meta_pixel_id, null);
  assert.equal(tiktok.time_spent_threshold, 25);
  assert.match(
    tiktokTemplateURL("https://example.com", "audio-a", "audio-novel"),
    /^https:\/\/example\.com\/audio-novel\/audio-a\?/,
  );
});

test("audio novel campaign form exposes only playback attribution controls", async () => {
  const source = await readFile(
    new URL("../src/views/AudioNovelLinkListView.vue", import.meta.url),
    "utf8",
  );
  const template = source.split("<script setup>")[0];
  assert.match(template, /语音小说投放链接/);
  assert.match(template, /达标播放时长（秒）/);
  assert.match(template, /实际播放/);
  assert.match(template, /复制 TikTok 投放模板/);
  assert.match(source, /first_visited_at/);
  assert.match(template, /:disabled="[^\"]*row\.visit_count[^\"]*"/);
  assert.match(source, /tiktokTemplateURL/);
  for (const label of [
    "归因方式",
    "渠道",
    "广告 ID",
    "广告系列 ID",
    "广告组 ID",
  ])
    assert.doesNotMatch(template, new RegExp(`label="${label}`));
});

test("audio novel list and router expose the dedicated campaign page", async () => {
  const [list, router] = await Promise.all([
    readFile(
      new URL("../src/views/AudioNovelListView.vue", import.meta.url),
      "utf8",
    ),
    readFile(new URL("../src/router/index.js", import.meta.url), "utf8"),
  ]);
  assert.match(list, /name: 'audio-novel-links'/);
  assert.match(list, />投放链接</);
  assert.match(router, /path: "audio-novels\/:id\/links"/);
  assert.match(router, /name: "audio-novel-links"/);
  assert.match(router, /activeMenu: "audio-novels"/);
});

test("audio novel statistics explain playback funnels and attribution boundaries", async () => {
  const source = await readFile(
    new URL("../src/views/AudioNovelLinkStatsView.vue", import.meta.url),
    "utf8",
  );
  for (const key of [
    "visits",
    "unique_visitors",
    "average_visible_seconds",
    "total_visible_seconds",
    "average_playback_seconds",
    "total_playback_seconds",
    "started_count",
    "start_rate",
    "qualified_count",
    "qualified_rate",
    "completed_count",
    "completion_rate",
    "pending_events",
    "accepted_events",
    "failed_events",
  ])
    assert.match(source, new RegExp(key));
  for (const filter of [
    "ad_id",
    "campaign_id",
    "adgroup_id",
    "creative_id",
    "ad_id_v2",
    "event_status",
  ])
    assert.match(source, new RegExp(filter));
  assert.match(source, /未采集/);
  assert.match(source, /平台 API 已接收不代表最终广告归因/);
  assert.match(source, /播放完成仅用于内部统计/);
  assert.match(source, /ttclid/);
  // Operators must see a current delivery configuration blocker before
  // interpreting an empty or pending event report as a tracking failure.
  assert.match(source, /delivery_config_status/);
  assert.match(source, /delivery_blocked_reason/);
  assert.match(source, /回传配置阻塞/);
  // Summary cards must not coerce a missing aggregate to 0 seconds.
  assert.match(
    source,
    /card\.duration\s*\?\s*data\.summary\[card\.key\] == null/,
  );
});
