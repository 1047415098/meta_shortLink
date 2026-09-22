import { request } from "./http.js";

// Keep admin-only presentation fields out of the mutation contract.
export function novelLinkPayload(source = {}) {
  const requestedThreshold = Number(source.time_spent_threshold);
  // 小说投放只暴露停留阈值；隐藏的归因字段统一使用动态 Meta 广告参数。
  return {
    name: source.name || "",
    code: source.code || "",
    novel_id: Number(source.novel_id) || 0,
    enabled: Boolean(source.enabled),
    channel: "facebook",
    campaign_id: "",
    adset_id: "",
    ad_id: "",
    meta_connection_id: source.meta_connection_id || null,
    meta_pixel_id: source.meta_pixel_id || null,
    attribution_mode: "dynamic",
    time_spent_threshold:
      source.time_spent_threshold === "" ||
      source.time_spent_threshold == null ||
      !Number.isFinite(requestedThreshold)
        ? 10
        : requestedThreshold,
  };
}
export function listNovelLinks(novelId) {
  const query = new URLSearchParams();
  if (novelId) query.set("novel_id", novelId);
  return request(`/novel-links?${query}`);
}
export const createNovelLink = (data) =>
  request("/novel-links", {
    method: "POST",
    body: JSON.stringify(novelLinkPayload(data)),
  });
export const updateNovelLink = (id, data) =>
  request(`/novel-links/${id}`, {
    method: "PATCH",
    body: JSON.stringify(novelLinkPayload(data)),
  });
export const deleteNovelLink = (id) =>
  request(`/novel-links/${id}`, { method: "DELETE" });
export const getNovelLinkStats = (id, filters) =>
  request(`/novel-links/${id}/stats`, {
    method: "POST",
    body: JSON.stringify(filters),
  });
