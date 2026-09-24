import { request } from "./http.js";

// Only send controls that operators can see; attribution IDs are read
// dynamically from the public URL and must never survive as hidden form state.
export function audioNovelLinkPayload(source = {}) {
  const platform = source.ad_platform === "tiktok" ? "tiktok" : "meta";
  const threshold = Number(source.time_spent_threshold);
  return {
    name: source.name || "",
    code: source.code || "",
    audio_novel_id: Number(source.audio_novel_id) || 0,
    enabled: Boolean(source.enabled),
    ad_platform: platform,
    meta_connection_id:
      platform === "meta" ? source.meta_connection_id || null : null,
    meta_pixel_id: platform === "meta" ? source.meta_pixel_id || null : null,
    tiktok_pixel_id:
      platform === "tiktok" ? source.tiktok_pixel_id || null : null,
    time_spent_threshold:
      source.time_spent_threshold === "" ||
      source.time_spent_threshold == null ||
      !Number.isFinite(threshold)
        ? 10
        : threshold,
  };
}

export function listAudioNovelLinks(audioNovelId) {
  const query = new URLSearchParams();
  if (audioNovelId) query.set("audio_novel_id", audioNovelId);
  return request(`/audio-novel-links?${query}`);
}

export const createAudioNovelLink = (data) =>
  request("/audio-novel-links", {
    method: "POST",
    body: JSON.stringify(audioNovelLinkPayload(data)),
  });

export const updateAudioNovelLink = (id, data) =>
  request(`/audio-novel-links/${id}`, {
    method: "PATCH",
    body: JSON.stringify(audioNovelLinkPayload(data)),
  });

export const deleteAudioNovelLink = (id) =>
  request(`/audio-novel-links/${id}`, { method: "DELETE" });

export const getAudioNovelLinkStats = (id, filters) =>
  request(`/audio-novel-links/${id}/stats`, {
    method: "POST",
    body: JSON.stringify(filters),
  });
