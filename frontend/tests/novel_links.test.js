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
    enabled: true,
    ignored: "no",
  });
  assert.equal(payload.novel_id, 7);
  assert.equal(payload.ignored, undefined);
  await listNovelLinks(7);
  await createNovelLink(payload);
  await updateNovelLink(2, payload);
  await getNovelLinkStats(2, { page: 1 });
  await deleteNovelLink(2);
  assert.deepEqual(
    calls.map(({ url }) => url),
    [
      "/api/v1/novel-links?novel_id=7",
      "/api/v1/novel-links",
      "/api/v1/novel-links/2",
      "/api/v1/novel-links/2/stats",
      "/api/v1/novel-links/2",
    ],
  );
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
});

test("admin registers the numeric input used by the dwell-time field", async () => {
  // Component source alone is insufficient because this project registers Element Plus widgets explicitly.
  const source = await readFile(
    new URL("../src/main.js", import.meta.url),
    "utf8",
  );
  assert.match(source, /import \{ ElInputNumber \} from "element-plus\/es\/components\/input-number\/index"/);
  assert.match(source, /ElInputNumber,\s*\n\s*ElSelect/);
});
