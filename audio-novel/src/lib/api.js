async function readJSON(url, request) {
  const response = await request(url, { headers: { Accept: "application/json" } });
  if (!response.ok) {
    const error = new Error(response.status === 410 ? "This literature archive is unavailable." : "The archive could not be loaded. Please try again.");
    error.status = response.status;
    throw error;
  }
  return response.json();
}

async function readActionJSON(url, options, request) {
  const response = await request(url, options);
  if (!response.ok) {
    const error = new Error("Playback could not be recorded. Listening can continue.");
    error.status = response.status;
    throw error;
  }
  if (response.status === 204 || typeof response.json !== "function") return { confirmed_events: [] };
  return response.json();
}

function playbackPayload(ticket, playbackSeconds = 0, mediaConsumedSeconds = 0) {
  return {
    ticket: String(ticket || ""),
    playback_seconds: Math.max(0, Math.floor(Number(playbackSeconds) || 0)),
    media_consumed_seconds: Math.max(0, Number(mediaConsumedSeconds) || 0),
  };
}

export async function fetchAudioNovelHome(code, request = fetch) {
  const data = await readJSON(`/audio-novel-api/${encodeURIComponent(code)}/home`, request);
  if (!("featured" in data)) throw new Error("The archive returned an incomplete response.");
  return data;
}
export async function fetchAudioNovelList(code, page = 1, pageSize = 6, request = fetch) {
  const data = await readJSON(`/audio-novel-api/${encodeURIComponent(code)}/stories?page=${Number(page) || 1}&page_size=${Number(pageSize) || 6}`, request);
  if (!Array.isArray(data.items)) throw new Error("The archive returned an incomplete response.");
  return data;
}
export async function fetchAudioNovelStory(code, slug, request = fetch) {
  const data = await readJSON(`/audio-novel-api/${encodeURIComponent(code)}/stories/${encodeURIComponent(slug)}`, request);
  if (!data.story || !Array.isArray(data.related)) throw new Error("The archive returned an incomplete response.");
  return data;
}

export async function fetchAudioNovelAudioList(code, page = 1, pageSize = 6, request = fetch) {
  const data = await readJSON(`/audio-novel-api/${encodeURIComponent(code)}/audio?page=${Number(page) || 1}&page_size=${Number(pageSize) || 6}`, request);
  if (!Array.isArray(data.items)) throw new Error("The archive returned an incomplete response.");
  return data;
}

export async function fetchAudioNovelAudio(code, slug, request = fetch) {
  const data = await readJSON(`/audio-novel-api/${encodeURIComponent(code)}/audio/${encodeURIComponent(slug)}`, request);
  if (!data.audio) throw new Error("The archive returned an incomplete response.");
  return data;
}

export function confirmAudioNovelView({ code, ticket, request = fetch } = {}) {
  return readActionJSON(`/audio-novel/${encodeURIComponent(code)}/view`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({ ticket: String(ticket || "") }).toString(),
    keepalive: true,
  }, request);
}

export function startAudioNovelPlayback({ code, ticket, request = fetch } = {}) {
  return readActionJSON(`/audio-novel/${encodeURIComponent(code)}/start-listening`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(playbackPayload(ticket)),
    keepalive: true,
  }, request);
}

export async function reportAudioNovelPlayback({ code, ticket, playbackSeconds, mediaConsumedSeconds, useBeacon = false, request = fetch, navigatorRef = globalThis.navigator } = {}) {
  const url = `/audio-novel/${encodeURIComponent(code)}/playback-time`;
  const payload = playbackPayload(ticket, playbackSeconds, mediaConsumedSeconds);
  if (useBeacon && typeof navigatorRef?.sendBeacon === "function") {
    // JSON Blob 让 pagehide 与常规 fetch 共用同一套严格服务端解码规则。
    return { ok: navigatorRef.sendBeacon(url, new Blob([JSON.stringify(payload)], { type: "application/json" })) };
  }
  return readActionJSON(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    keepalive: true,
  }, request);
}

export async function reportAudioNovelVisibleTime({ code, ticket, visibleSeconds, useBeacon = false, request = fetch, navigatorRef = globalThis.navigator } = {}) {
  const url = `/audio-novel/${encodeURIComponent(code)}/visible-time`;
  const payload = {
    ticket: String(ticket || ""),
    visible_seconds: Math.max(0, Math.floor(Number(visibleSeconds) || 0)),
  };
  if (useBeacon && typeof navigatorRef?.sendBeacon === "function") {
    // Beacon keeps the same strict JSON contract while the document is hidden or leaving.
    return { ok: navigatorRef.sendBeacon(url, new Blob([JSON.stringify(payload)], { type: "application/json" })) };
  }
  return readActionJSON(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    keepalive: true,
  }, request);
}

export function completeAudioNovelPlayback({ code, ticket, playbackSeconds, mediaConsumedSeconds, request = fetch } = {}) {
  return readActionJSON(`/audio-novel/${encodeURIComponent(code)}/complete`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ...playbackPayload(ticket, playbackSeconds, mediaConsumedSeconds), ended: true }),
    keepalive: true,
  }, request);
}

export function formatAudioSize(bytes) {
  return `${(Math.max(0, Number(bytes) || 0) / 1024 / 1024).toFixed(2)} MB`;
}
