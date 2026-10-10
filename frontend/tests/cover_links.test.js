import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import {
  coverLinkPayload,
  createCoverLink,
  deleteCoverLinks,
  getCoverLinkStats,
  listCoverLinks,
} from "../src/api/coverLinks.js";

test("cover project client uses isolated JSON endpoints and owns no novel fields", async (t) => {
  const payload = coverLinkPayload({
    name: "Buyer A",
    code: "cover-a",
    enabled: true,
    ad_platform: "meta",
    meta_connection_id: 2,
    meta_pixel_id: 3,
    novel_id: 99,
    entry_chapter_id: 100,
    time_spent_threshold: 10,
  });
  assert.equal(payload.novel_id, undefined);
  assert.equal(payload.entry_chapter_id, undefined);
  assert.equal(payload.attribution_mode, "dynamic");

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
  await listCoverLinks();
  await createCoverLink(payload);
  await deleteCoverLinks([7, 8]);
  await getCoverLinkStats(7, {
    start: "2026-10-09",
    end: "2026-10-09",
    tz: "Etc/GMT+8",
    page: 1,
  });
  assert.deepEqual(
    calls.map(({ url }) => url),
    [
      "/api/v1/cover-links/query",
      "/api/v1/cover-links",
      "/api/v1/cover-links/batch-delete",
      "/api/v1/cover-links/7/stats",
    ],
  );
  assert.deepEqual(JSON.parse(calls[2].options.body), { ids: [7, 8] });
  assert.ok(calls.every(({ options }) => options.method === "POST"));
});

test("cover admin exposes links and attribution without novel content controls", async () => {
  const [layout, router, list, stats] = await Promise.all([
    readFile(
      new URL("../src/layouts/AdminLayout.vue", import.meta.url),
      "utf8",
    ),
    readFile(new URL("../src/router/index.js", import.meta.url), "utf8"),
    readFile(
      new URL("../src/views/CoverLinkListView.vue", import.meta.url),
      "utf8",
    ),
    readFile(
      new URL("../src/views/CoverLinkStatsView.vue", import.meta.url),
      "utf8",
    ),
  ]);
  // 菜单与路由独立于免费小说，不提供小说新增、编辑、推荐或内容绑定。
  assert.match(layout, /label: "封面项目"/);
  assert.match(layout, /name: "cover-links", label: "封面链接管理"/);
  assert.match(router, /path: "cover-links"/);
  assert.match(router, /path: "cover-links\/:id\/stats"/);
  assert.match(list, /创建封面链接/);
  assert.match(list, /删除所选/);
  assert.match(list, /历史统计会保留/);
  assert.match(list, /Meta/);
  assert.match(list, /TikTok/);
  assert.match(list, /停留时长回传（秒）/);
  assert.doesNotMatch(
    list,
    /label="绑定小说"|label="入口章节"|设置推荐按钮|新增小说按钮|编辑小说按钮/,
  );
  assert.match(stats, /匿名独立访客/);
  assert.match(stats, /平均可见时长/);
  assert.match(stats, /平台已接收/);
});
