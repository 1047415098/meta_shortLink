import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import {
  createNovelLink,
  deleteNovelLink,
  getNovelLinkStats,
  listNovelLinks,
  novelLinkPayload,
  updateNovelLink,
} from "../src/api/novelLinks.js";
import * as novelLinksAPI from "../src/api/novelLinks.js";

test("novel distribution client keeps links and statistics on dedicated endpoints", async (t) => {
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
  const payload = novelLinkPayload({
    name: "投手 A",
    novel_id: "7",
    entry_chapter_id: "12",
    enabled: true,
    ignored: "no",
  });
  assert.equal(payload.novel_id, 7);
  assert.equal(payload.entry_chapter_id, 12);
  assert.equal(payload.ignored, undefined);
  await listNovelLinks(7);
  await createNovelLink(payload);
  await updateNovelLink(2, payload);
  await getNovelLinkStats(2, { page: 1 });
  await deleteNovelLink(2);
  assert.deepEqual(
    calls.map(({ url }) => url),
    [
      "/api/v1/novel-links/query",
      "/api/v1/novel-links",
      "/api/v1/novel-links/2",
      "/api/v1/novel-links/2/stats",
      "/api/v1/novel-links/2",
    ],
  );
  assert.equal(calls[0].options.method, "POST");
  assert.deepEqual(JSON.parse(calls[0].options.body), { novel_id: 7 });
});

test("novel link payload always uses hidden dynamic attribution defaults", () => {
  // Hidden controls must not leave stale fixed-attribution values in edited links.
  const payload = novelLinkPayload({
    attribution_mode: "bound",
    channel: "instagram",
    campaign_id: "campaign-old",
    adset_id: "adset-old",
    ad_id: "ad-old",
  });
  assert.equal(payload.attribution_mode, "dynamic");
  assert.equal(payload.channel, "facebook");
  assert.equal(payload.campaign_id, "");
  assert.equal(payload.adset_id, "");
  assert.equal(payload.ad_id, "");
  assert.equal(payload.time_spent_threshold, 10);
  assert.equal(
    novelLinkPayload({ time_spent_threshold: 30 }).time_spent_threshold,
    30,
  );
});

test("novel link payload selects exactly one platform", () => {
  const tiktok = novelLinkPayload({
    ad_platform: "tiktok",
    novel_id: 7,
    tiktok_pixel_id: 9,
    meta_pixel_id: 4,
    meta_connection_id: 3,
  });
  assert.equal(tiktok.ad_platform, "tiktok");
  assert.equal(tiktok.channel, "tiktok");
  assert.equal(tiktok.tiktok_pixel_id, 9);
  assert.equal(tiktok.meta_pixel_id, null);
  assert.equal(tiktok.meta_connection_id, null);
  assert.equal(tiktok.attribution_mode, "dynamic");

  const meta = novelLinkPayload({
    ad_platform: "meta",
    novel_id: 7,
    tiktok_pixel_id: 9,
    meta_pixel_id: 4,
    meta_connection_id: 3,
  });
  assert.equal(meta.tiktok_pixel_id, null);
  assert.equal(meta.meta_pixel_id, 4);
  assert.equal(meta.meta_connection_id, 3);
});

test("novel link form only exposes the dwell-time attribution option", async () => {
  // The operator only chooses the threshold; dynamic attribution fields stay implementation details.
  const source = await readFile(
    new URL("../src/views/NovelLinkListView.vue", import.meta.url),
    "utf8",
  );
  const template = source.split("<script setup>")[0];
  for (const label of [
    "归因方式",
    "渠道",
    "广告 ID（可选）",
    "广告系列 ID",
    "广告组 ID",
  ]) {
    assert.doesNotMatch(template, new RegExp(`label="${label}"`));
  }
  assert.match(template, /label="停留时长回传（秒）"/);
  assert.match(template, /达到设置的前台可见时长后才回传一次 Meta TimeSpent/);
  assert.match(template, /TikTok ViewContent/);
  assert.match(template, /复制普通短链/);
  assert.match(template, /复制 TikTok 投放模板/);
});

test("novel link form binds an enabled entry chapter and locks it after visits", async () => {
  const source = await readFile(
    new URL("../src/views/NovelLinkListView.vue", import.meta.url),
    "utf8",
  );
  const template = source.split("<script setup>")[0];
  const script = source.split("<script setup>")[1].split("</script>")[0];

  assert.match(template, /label="入口章节" required/);
  assert.match(template, /v-model="form\.entry_chapter_id"/);
  assert.match(template, /:disabled="Boolean\(editing\?\.first_visited_at\)"/);
  assert.match(
    template,
    /:disabled="!chapter\.enabled \|\| chapter\.entry_unavailable"/,
  );
  assert.match(template, /@change="changeNovel"/);
  assert.match(template, /简介页（兼容旧链接）/);
  assert.match(template, /首次访问后，小说和入口章节均不可修改/);
  assert.match(script, /listChapters/);
  assert.match(script, /novelEntryChapterOptions/);
  assert.match(script, /const needsEntryChapter = !editing\.value;/);
  assert.match(
    script,
    /form\.entry_chapter_id = lockedLink\?\.entry_chapter_id/,
  );
});

test("novel link table shows the bound entry chapter", async () => {
  const source = await readFile(
    new URL("../src/views/NovelLinkListView.vue", import.meta.url),
    "utf8",
  );
  const template = source.split("<script setup>")[0];
  assert.match(template, /label="入口章节"/);
  assert.match(template, /entryChapterLabel\(row\)/);
});

test("entry chapter choices keep frozen unavailable chapters only for visited links", () => {
  assert.equal(typeof novelLinksAPI.novelEntryChapterOptions, "function");
  const chapters = [
    { id: 4, chapter_number: 4, title: "Four", enabled: true },
    { id: 1, chapter_number: 1, title: "One", enabled: true },
    { id: 2, chapter_number: 2, title: "Two", enabled: false },
    {
      id: 3,
      chapter_number: 3,
      title: "Three",
      enabled: true,
      deleted_at: "2026-09-25T00:00:00Z",
    },
  ];

  assert.deepEqual(
    novelLinksAPI.novelEntryChapterOptions(chapters).map(({ id }) => id),
    [1, 4],
  );
  const disabled = novelLinksAPI.novelEntryChapterOptions(chapters, {
    first_visited_at: "2026-09-25T01:00:00Z",
    entry_chapter_id: 2,
    entry_chapter_number: 2,
    entry_chapter_title: "Original two",
  });
  assert.equal(disabled.find(({ id }) => id === 2)?.enabled, false);

  const missing = novelLinksAPI.novelEntryChapterOptions(chapters, {
    first_visited_at: "2026-09-25T01:00:00Z",
    entry_chapter_id: 9,
    entry_chapter_number: 9,
    entry_chapter_title: "Deleted chapter",
  });
  assert.deepEqual(
    missing.find(({ id }) => id === 9),
    {
      id: 9,
      chapter_number: 9,
      title: "Deleted chapter",
      enabled: false,
      entry_unavailable: true,
    },
  );
});

test("chapter loading ignores stale responses after the selected novel changes", async () => {
  const source = await readFile(
    new URL("../src/views/NovelLinkListView.vue", import.meta.url),
    "utf8",
  );
  const script = source.split("<script setup>")[1].split("</script>")[0];
  assert.match(script, /let chapterLoadRequest = 0/);
  assert.match(script, /const requestID = \+\+chapterLoadRequest/);
  assert.match(script, /if \(requestID !== chapterLoadRequest\) return/);
  assert.match(script, /if \(requestID === chapterLoadRequest\)/);
});

test("admin registers the numeric input used by the dwell-time field", async () => {
  // Component source alone is insufficient because this project registers Element Plus widgets explicitly.
  const source = await readFile(
    new URL("../src/main.js", import.meta.url),
    "utf8",
  );
  assert.match(
    source,
    /import \{ ElInputNumber \} from "element-plus\/es\/components\/input-number\/index"/,
  );
  assert.match(source, /ElInputNumber,\s*\n\s*ElSelect/);
});

test("TikTok novel statistics expose funnel, delivery and attribution boundaries", async () => {
  const source = await readFile(
    new URL("../src/views/NovelLinkStatsView.vue", import.meta.url),
    "utf8",
  );
  for (const key of [
    "start_reading_visitors",
    "qualified_visitors",
    "qualified_rate",
    "tiktok_pending_events",
    "tiktok_accepted_events",
    "tiktok_failed_events",
  ])
    assert.match(source, new RegExp(key));
  for (const filter of [
    "campaign_id",
    "adgroup_id",
    "creative_id",
    "ad_id_v2",
    "event_status",
  ])
    assert.match(source, new RegExp(filter));
  assert.match(
    source,
    /TikTok 是否归因：本系统未知，请到 TikTok Ads Manager 查看。/,
  );
  assert.match(source, /TikTok 已接收/);
});

test("novel statistics show the fixed ten-second unique visitor metric", async () => {
  const source = await readFile(
    new URL("../src/views/NovelLinkStatsView.vue", import.meta.url),
    "utf8",
  );

  // 该指标属于 Meta/TikTok 共用基础卡片，不依赖平台回传是否成功。
  assert.match(source, /key: "ten_second_unique_visitors"/);
  assert.match(source, /label: "停留满10秒独立访客"/);
  assert.match(source, /单次前台可见时长达到10秒，按匿名访客 Cookie 去重/);
});

test("novel visit statistics show the frozen entry chapter and identify legacy visits", async () => {
  const source = await readFile(
    new URL("../src/views/NovelLinkStatsView.vue", import.meta.url),
    "utf8",
  );
  const template = source.split("<script setup>")[0];

  // 该列必须使用访问快照，避免章节后续改名时覆盖历史投放数据。
  assert.match(template, /label="入口章节"/);
  assert.match(template, /row\.entry_chapter_number/);
  assert.match(template, /row\.entry_chapter_title/);
  assert.match(template, /未记录（兼容旧链接）/);
});
