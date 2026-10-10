import test from "node:test";
import assert from "node:assert/strict";

import { getLogs } from "../src/api/requestLogs.js";

test("request-log filters send an empty status as numeric zero", async () => {
  const calls = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (path, options) => {
    calls.push({ path, options });
    return new Response(JSON.stringify({ items: [], total: 0, page: 1 }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
  try {
    await getLogs({
      path: "",
      method: "",
      status: "",
      start: "2026-10-09",
      end: "2026-10-09",
      tz: "Etc/GMT+8",
      page: 1,
    });
    await getLogs({ status: "200" });
  } finally {
    globalThis.fetch = originalFetch;
  }

  // 空值和文本输入都在进入请求体前规范成后端需要的数字。
  assert.equal(calls[0].path, "/api/v1/request-logs/query");
  assert.equal(JSON.parse(calls[0].options.body).status, 0);
  assert.equal(JSON.parse(calls[1].options.body).status, 200);
});

test("request-log filters reject an invalid status before querying", async () => {
  // 非法状态码显示明确错误，不再落入通用的 JSON 格式错误。
  await assert.rejects(
    () => getLogs({ status: "abc" }),
    /状态码请输入 100 至 599/,
  );
  await assert.rejects(
    () => getLogs({ status: "99" }),
    /状态码请输入 100 至 599/,
  );
});
