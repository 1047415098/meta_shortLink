import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

import {
  deleteTikTokConnection,
  deleteTikTokPixel,
  listTikTokConnections,
  listTikTokEvents,
  listTikTokPixels,
  retryTikTokEvent,
  saveTikTokConnection,
  saveTikTokPixel,
  testTikTokPixel,
} from "../src/api/tiktok.js";
import {
  tiktokConnectionPayload,
  tiktokTemplateURL,
} from "../src/utils/tiktok.js";

test("TikTok admin client uses exact centralized API paths", async (t) => {
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
  await listTikTokConnections();
  await saveTikTokConnection(null, {
    name: "Production",
    access_token: "token",
    enabled: true,
  });
  await saveTikTokConnection(2, {
    name: "Production",
    access_token: "",
    enabled: true,
  });
  await deleteTikTokConnection(2);
  await listTikTokPixels(2);
  await saveTikTokPixel(null, {
    connection_id: 2,
    name: "Pixel",
    pixel_code: "C0ABC123",
    enabled: true,
  });
  await saveTikTokPixel(3, {
    connection_id: 2,
    name: "Pixel",
    pixel_code: "C0ABC123",
    enabled: false,
  });
  await testTikTokPixel(3);
  await deleteTikTokPixel(3);
  await listTikTokEvents({ status: "failed", page: 2 });
  await retryTikTokEvent("novel_visit_qualified");
  assert.deepEqual(
    calls.map(({ url }) => url),
    [
      "/api/v1/tiktok-connections",
      "/api/v1/tiktok-connections",
      "/api/v1/tiktok-connections/2",
      "/api/v1/tiktok-connections/2",
      "/api/v1/tiktok-pixels?connection_id=2",
      "/api/v1/tiktok-pixels",
      "/api/v1/tiktok-pixels/3",
      "/api/v1/tiktok-pixels/3/test",
      "/api/v1/tiktok-pixels/3",
      "/api/v1/tiktok-events?status=failed&page=2",
      "/api/v1/tiktok-events/novel_visit_qualified/retry",
    ],
  );
  assert.equal(JSON.parse(calls[2].options.body).access_token, undefined);
});

test("TikTok helpers omit blank edited secrets and match the server template", () => {
  assert.deepEqual(
    tiktokConnectionPayload(
      { name: " Main ", enabled: true, access_token: "" },
      true,
    ),
    {
      name: "Main",
      enabled: true,
    },
  );
  assert.equal(
    tiktokTemplateURL("https://example.com/", "wife-a"),
    "https://example.com/novel/wife-a?utm_source=tiktok&utm_medium=paid_social&campaign_id=__CAMPAIGN_ID__&adgroup_id=__AID__&creative_id=__CID__&ad_id_v2=__ADID_V2__&placement=__PLACEMENT__",
  );
});

test("TikTok pages explain API acceptance without claiming ad attribution", async () => {
  const sources = await Promise.all(
    [
      "TikTokConnectionsView.vue",
      "TikTokPixelsView.vue",
      "TikTokEventsView.vue",
    ].map((name) =>
      readFile(new URL(`../src/views/${name}`, import.meta.url), "utf8"),
    ),
  );
  assert.match(sources[0], /Access Token/);
  assert.match(sources[1], /发送测试事件/);
  assert.match(
    sources[2],
    /TikTok 是否归因：本系统未知，请到 TikTok Ads Manager 查看。/,
  );
  assert.match(sources[2], /TikTok 已接收/);
  assert.match(sources[2], /"StartListening"/);
  assert.match(sources[2], /audio_novel_id/);
  assert.match(sources[2], /语音小说事件/);
});
