import { request } from "./http.js";

export function coverLinkPayload(source = {}) {
  const platform = source.ad_platform === "tiktok" ? "tiktok" : "meta";
  const threshold = Number(source.time_spent_threshold);
  // The cover project always uses dynamic attribution and owns no novel fields.
  return {
    name: source.name || "",
    code: source.code || "",
    enabled: Boolean(source.enabled),
    ad_platform: platform,
    attribution_mode: "dynamic",
    meta_connection_id: platform === "meta" ? source.meta_connection_id || null : null,
    meta_pixel_id: platform === "meta" ? source.meta_pixel_id || null : null,
    tiktok_pixel_id: platform === "tiktok" ? source.tiktok_pixel_id || null : null,
    time_spent_threshold: Number.isFinite(threshold) ? threshold : 10,
  };
}

export const listCoverLinks = () => request("/cover-links/query", { method: "POST", body: "{}" });
export const createCoverLink = (data) => request("/cover-links", { method: "POST", body: JSON.stringify(coverLinkPayload(data)) });
export const updateCoverLink = (id, data) => request(`/cover-links/${id}`, { method: "PATCH", body: JSON.stringify(coverLinkPayload(data)) });
export const deleteCoverLink = (id) => request(`/cover-links/${id}`, { method: "DELETE" });
export const getCoverLinkStats = (id, filters) => request(`/cover-links/${id}/stats`, { method: "POST", body: JSON.stringify(filters) });
