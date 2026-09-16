import test from "node:test";
import assert from "node:assert/strict";

import { deleteConnection, deletePixel } from "../src/api/meta.js";

test("Meta deletion uses authenticated DELETE endpoints", async () => {
  const calls = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (path, options) => {
    calls.push({ path, options });
    return new Response(JSON.stringify({ ok: true }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
  try {
    await deletePixel(17);
    await deleteConnection(9);
  } finally {
    globalThis.fetch = originalFetch;
  }

  // Both UI actions share the request wrapper, which supplies the session and
  // anti-CSRF header while keeping identifiers out of a request body.
  assert.deepEqual(
    calls.map(({ path, options }) => ({
      path,
      method: options.method,
      requestedWith: options.headers["X-Requested-With"],
    })),
    [
      {
        path: "/api/v1/meta/pixels/17",
        method: "DELETE",
        requestedWith: "XMLHttpRequest",
      },
      {
        path: "/api/v1/meta/connections/9",
        method: "DELETE",
        requestedWith: "XMLHttpRequest",
      },
    ],
  );
});
