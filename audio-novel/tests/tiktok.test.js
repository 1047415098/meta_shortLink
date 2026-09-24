import test from "node:test";
import assert from "node:assert/strict";
import { installMetaPixel, trackMetaAudioEvent } from "../src/lib/meta.js";
import { installTikTokPixel, readTikTokTTP, trackTikTokAudioEvent } from "../src/lib/tiktok.js";

function fakeDocument(cookie = "") {
  const appended = [];
  const documentRef = {
    cookie,
    documentElement: { dataset: {} },
    head: { appendChild(node) { appended.push(node); } },
    createElement: () => ({}),
  };
  return { appended, documentRef };
}

test("TikTok Pixel loads once and sends each server-confirmed event id once", () => {
  const scope = {};
  const { appended, documentRef } = fakeDocument();
  assert.equal(installTikTokPixel({ pixelCode: "C0ABC123", scope, documentRef }), true);
  assert.equal(installTikTokPixel({ pixelCode: "C0ABC123", scope, documentRef }), true);
  assert.equal(trackTikTokAudioEvent({ name: "PageView", eventId: "audio_v1_view", scope, documentRef }), true);
  assert.equal(trackTikTokAudioEvent({ name: "PageView", eventId: "audio_v1_view", scope, documentRef }), false);
  assert.equal(trackTikTokAudioEvent({ name: "StartListening", eventId: "audio_v1_start", content: { id: 42, title: "Frozen Audio" }, scope, documentRef }), true);
  assert.equal(appended.filter((node) => node.id === "audio-novel-tiktok-pixel").length, 1);
  assert.equal(scope.ttq.filter((item) => item[0] === "page").length, 1);
  const page = scope.ttq.find((item) => item[0] === "page");
  assert.equal(page[1].content_type, "audio_novel");
  assert.equal(page[2].event_id, "audio_v1_view");
  const start = scope.ttq.find((item) => item[0] === "track");
  assert.equal(start[1], "StartListening");
  assert.equal(start[2].content_type, "audio_novel");
  assert.deepEqual(start[2].contents, [{ content_id: "audio_novel:42", content_name: "Frozen Audio" }]);
  assert.equal(start[3].event_id, "audio_v1_start");
});

test("event id deduplication is isolated to one document and retries after SDK errors", () => {
  const first = fakeDocument();
  const second = fakeDocument();
  const brokenScope = { ttq: { track() { throw new Error("blocked"); } } };
  assert.equal(trackTikTokAudioEvent({ name: "ViewContent", eventId: "audio_v1_qualified", scope: brokenScope, documentRef: first.documentRef }), false);
  const healthyScope = { ttq: { track() {} } };
  assert.equal(trackTikTokAudioEvent({ name: "ViewContent", eventId: "audio_v1_qualified", scope: healthyScope, documentRef: first.documentRef }), true);
  assert.equal(trackTikTokAudioEvent({ name: "ViewContent", eventId: "audio_v1_qualified", scope: healthyScope, documentRef: first.documentRef }), false);
  assert.equal(trackTikTokAudioEvent({ name: "ViewContent", eventId: "audio_v1_qualified", scope: healthyScope, documentRef: second.documentRef }), true);
  assert.equal(trackTikTokAudioEvent({ name: "Purchase", eventId: "audio_v1_purchase", scope: healthyScope, documentRef: second.documentRef }), false);
});

test("Meta Pixel sends PageView, custom StartListening and ViewContent with exact event ids", () => {
  const scope = {};
  const { appended, documentRef } = fakeDocument();
  assert.equal(installMetaPixel({ pixelId: "123456789", scope, documentRef }), true);
  assert.equal(trackMetaAudioEvent({ name: "PageView", eventId: "audio_v1_view", scope, documentRef }), true);
  assert.equal(trackMetaAudioEvent({ name: "StartListening", eventId: "audio_v1_start", scope, documentRef }), true);
  assert.equal(trackMetaAudioEvent({ name: "ViewContent", eventId: "audio_v1_qualified", content: { id: 42, title: "Frozen Audio" }, scope, documentRef }), true);
  assert.equal(trackMetaAudioEvent({ name: "ViewContent", eventId: "audio_v1_qualified", scope, documentRef }), false);
  assert.equal(appended.filter((node) => node.id === "audio-novel-meta-pixel").length, 1);
  const pageView = scope.fbq.queue.find((args) => args[1] === "PageView");
  assert.equal(pageView[2].content_type, "audio_novel");
  assert.deepEqual(scope.fbq.queue.map((args) => [args[0], args[1], args[3]?.eventID]), [
    ["init", "123456789", undefined],
    ["track", "PageView", "audio_v1_view"],
    ["trackCustom", "StartListening", "audio_v1_start"],
    ["track", "ViewContent", "audio_v1_qualified"],
  ]);
  const qualified = scope.fbq.queue.find((args) => args[1] === "ViewContent");
  assert.equal(qualified[2].content_type, "audio_novel");
  assert.deepEqual(qualified[2].content_ids, ["audio_novel:42"]);
});

test("advertising helpers reject unsafe input and SDK failures stay isolated", () => {
  const scope = {};
  const { appended, documentRef } = fakeDocument();
  assert.equal(installTikTokPixel({ pixelCode: "bad code", scope, documentRef }), false);
  assert.equal(appended.length, 0);
  assert.equal(installTikTokPixel({ pixelCode: "C0ABC123", scope, documentRef }), true);
  assert.doesNotThrow(() => appended[0].onerror());
  assert.equal(trackTikTokAudioEvent({ name: "ViewContent", eventId: "audio-v1", scope, documentRef }), false);

  const throwingScope = { fbq() { throw new Error("SDK unavailable"); } };
  assert.equal(trackMetaAudioEvent({ name: "ViewContent", eventId: "audio-v2", scope: throwingScope, documentRef }), false);
});

test("TikTok first-party Cookie reader returns only a valid _ttp value", () => {
  const { documentRef } = fakeDocument("private=secret; _ttp=ttp%2Dcookie%2D123; theme=dark");
  assert.equal(readTikTokTTP(documentRef), "ttp-cookie-123");
  documentRef.cookie = "_ttp=bad%0Avalue";
  assert.equal(readTikTokTTP(documentRef), "");
});
