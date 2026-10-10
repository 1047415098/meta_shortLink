import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

import { getVisit } from "../src/api/analytics.js";

test("visit detail API encodes the selected visit id", async () => {
  const originalFetch = globalThis.fetch;
  let requestedURL = "";
  globalThis.fetch = async (url) => {
    requestedURL = url;
    return new Response(JSON.stringify({ id: "visit/id" }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
  try {
    await getVisit("visit/id");
    // Visit identifiers remain path data and can never inject another URL segment.
    assert.equal(requestedURL, "/api/v1/clicks/visit%2Fid");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("visit list provides grouped safe detail diagnostics", async () => {
  const source = await readFile(
    new URL("../src/views/VisitListView.vue", import.meta.url),
    "utf8",
  );
  assert.match(source, />明细<\/el-button/);
  assert.match(source, /访问完整明细/);
  for (const heading of [
    "基础访问",
    "设备与地区",
    "原始请求信息",
    "广告归因",
    "内容与用户行为",
    "广告平台快照",
    "Meta \/ TikTok 回传记录",
  ])
    assert.match(source, new RegExp(heading));
  // Device troubleshooting includes every safe field parsed when the visit is first recorded.
  for (const label of ["设备型号", "系统版本", "浏览器版本"])
    assert.match(source, new RegExp(label));
  assert.match(source, /detail\.device_model/);
  assert.match(source, /detail\.os_version/);
  assert.match(source, /detail\.browser_version/);
  // Admin diagnostics expose raw request values while explicitly excluding credentials.
  for (const field of [
    "detail.client_ip",
    "detail.user_agent",
    "detail.request_url",
    "detail.referrer_url",
    "detail.accept_language",
    "detail.client_hints",
  ])
    assert.match(source, new RegExp(field.replace(".", "\\.")));
  assert.match(source, /后台 Cookie、Authorization/);
  assert.match(source, /row\.response_messages/);
  // 独立封面项目必须显示自己的入口标签，避免与免费小说访问混在一起。
  assert.match(source, /cover: "封面项目"/);
});
