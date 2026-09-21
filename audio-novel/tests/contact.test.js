import assert from "node:assert/strict";
import test from "node:test";
import { submitAudioNovelContact } from "../src/lib/contact.js";

test("audio novel consultation is manual and scoped to the current code", async () => {
  let captured;
  const data = await submitAudioNovelContact({
    code: "hello",
    ticket: "signed.ticket",
    request: async (url, options) => {
      captured = { url, options };
      return { ok: true, json: async () => ({ target_url: "https://wa.me/123" }) };
    }
  });
  assert.equal(captured.url, "/audio-novel/hello/contact");
  assert.equal(captured.options.body.get("trigger"), "manual");
  assert.equal(data.target_url, "https://wa.me/123");
});
