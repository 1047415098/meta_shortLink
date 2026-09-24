import test from "node:test";
import assert from "node:assert/strict";
import {
  completeAudioNovelPlayback,
  confirmAudioNovelView,
  fetchAudioNovelHome,
  fetchAudioNovelList,
  fetchAudioNovelStory,
  fetchAudioNovelAudioList,
  fetchAudioNovelAudio,
  formatAudioSize,
  reportAudioNovelPlayback,
  reportAudioNovelVisibleTime,
  startAudioNovelPlayback,
} from "../src/lib/api.js";

function response(body, ok = true, status = 200) { return { ok, status, json: async () => body }; }

test("audio novel API safely encodes code, slug and paging", async () => {
  const urls = [];
  const request = async (url) => { urls.push(url); return response(url.includes("/home") ? { featured: null } : url.includes("stories?") ? { items: [], page: 2, pages: 0, total: 0 } : { story: {}, related: [] }); };
  await fetchAudioNovelHome("hello world", request);
  await fetchAudioNovelList("hello world", 2, 6, request);
  await fetchAudioNovelStory("hello world", "glass/orchard", request);
  assert.deepEqual(urls, ["/audio-novel-api/hello%20world/home", "/audio-novel-api/hello%20world/stories?page=2&page_size=6", "/audio-novel-api/hello%20world/stories/glass%2Forchard"]);
});

test("audio fiction API safely encodes list and detail paths", async () => {
  const urls = [];
  const request = async (url) => {
    urls.push(url);
    return response(url.includes("?page=") ? { items: [] } : { audio: { slug: "glass/orchard" } });
  };
  await fetchAudioNovelAudioList("hello world", 2, 8, request);
  await fetchAudioNovelAudio("hello world", "glass/orchard", request);
  assert.deepEqual(urls, [
    "/audio-novel-api/hello%20world/audio?page=2&page_size=8",
    "/audio-novel-api/hello%20world/audio/glass%2Forchard",
  ]);
  assert.equal(formatAudioSize(23100419), "22.03 MB");
});

test("audio novel API exposes readable failures", async () => {
  await assert.rejects(() => fetchAudioNovelHome("hello", async () => response({}, false, 410)), /unavailable/i);
});

test("audio campaign actions preserve exact JSON counters and confirmed events", async () => {
  const calls = [];
  const request = async (url, options) => {
    calls.push({ url, options });
    return response({ playback_seconds: 10, confirmed_events: [{ name: "ViewContent", event_id: "audio_v1_qualified" }] });
  };
  await confirmAudioNovelView({ code: "hello world", ticket: "view-ticket", request });
  await startAudioNovelPlayback({ code: "hello world", ticket: "play-ticket", request });
  await reportAudioNovelPlayback({ code: "hello world", ticket: "play-ticket", playbackSeconds: 10, mediaConsumedSeconds: 12.5, request });
  await completeAudioNovelPlayback({ code: "hello world", ticket: "play-ticket", playbackSeconds: 20, mediaConsumedSeconds: 30, request });

  assert.equal(calls[0].url, "/audio-novel/hello%20world/view");
  assert.equal(calls[0].options.body, "ticket=view-ticket");
  assert.equal(calls[1].url, "/audio-novel/hello%20world/start-listening");
  assert.deepEqual(JSON.parse(calls[1].options.body), { ticket: "play-ticket", playback_seconds: 0, media_consumed_seconds: 0 });
  assert.deepEqual(JSON.parse(calls[2].options.body), { ticket: "play-ticket", playback_seconds: 10, media_consumed_seconds: 12.5 });
  assert.deepEqual(JSON.parse(calls[3].options.body), { ticket: "play-ticket", playback_seconds: 20, media_consumed_seconds: 30, ended: true });
});

test("playback action errors stay explicit for the caller to isolate from media", async () => {
  await assert.rejects(
    () => startAudioNovelPlayback({ code: "hello", ticket: "signed", request: async () => response({}, false, 429) }),
    (error) => error.status === 429 && /Listening can continue/.test(error.message),
  );
});

test("pagehide playback uses a JSON Beacon", async () => {
  const calls = [];
  const navigatorRef = { sendBeacon(url, body) { calls.push({ url, body }); return true; } };
  const result = await reportAudioNovelPlayback({
    code: "hello", ticket: "signed", playbackSeconds: 8, mediaConsumedSeconds: 9.25,
    useBeacon: true, navigatorRef,
  });
  assert.equal(result.ok, true);
  assert.equal(calls[0].url, "/audio-novel/hello/playback-time");
  assert.equal(calls[0].body.type, "application/json");
  assert.deepEqual(JSON.parse(await calls[0].body.text()), { ticket: "signed", playback_seconds: 8, media_consumed_seconds: 9.25 });
});

test("campaign visible time uses JSON fetch and Beacon payloads", async () => {
  const fetchCalls = [];
  await reportAudioNovelVisibleTime({
    code: "hello world", ticket: "signed-ticket", visibleSeconds: 12,
    request: async (url, options) => { fetchCalls.push({ url, options }); return response({ visible_seconds: 12 }); },
  });
  assert.equal(fetchCalls[0].url, "/audio-novel/hello%20world/visible-time");
  assert.deepEqual(JSON.parse(fetchCalls[0].options.body), { ticket: "signed-ticket", visible_seconds: 12 });

  const beaconCalls = [];
  const navigatorRef = { sendBeacon(url, body) { beaconCalls.push({ url, body }); return true; } };
  await reportAudioNovelVisibleTime({ code: "hello", ticket: "signed-ticket", visibleSeconds: 18, useBeacon: true, navigatorRef });
  assert.equal(beaconCalls[0].url, "/audio-novel/hello/visible-time");
  assert.equal(beaconCalls[0].body.type, "application/json");
  assert.deepEqual(JSON.parse(await beaconCalls[0].body.text()), { ticket: "signed-ticket", visible_seconds: 18 });
});
