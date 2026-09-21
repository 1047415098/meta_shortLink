import { request } from "./http.js";

export function normalizeAudioNovelSlug(value = "") {
  // 保留中文等 Unicode 字母，避免用户离开输入框后 slug 被清空。
  return value
    .toLowerCase()
    .trim()
    .replace(/[^\p{L}\p{N}]+/gu, "-")
    .replace(/^-|-$/g, "");
}

// 只挑选后端允许的字段，避免把作者等旧数据带回新模型。
export function audioNovelPayload(source) {
  return {
    title: source.title || "",
    slug: source.slug || "",
    category: source.category || "",
    excerpt: source.excerpt || "",
    body_markdown: source.body_markdown || "",
    cover_path: source.cover_path || "",
    published_at: source.published_at || "",
    enabled: Boolean(source.enabled),
    featured: Boolean(source.featured),
  };
}

export function listAudioNovels(filters = {}) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(filters))
    if (value !== "" && value != null) query.set(key, value);
  return request(`/audio-novels?${query}`);
}
export const getAudioNovel = (id) => request(`/audio-novels/${id}`);
export const createAudioNovel = (data) =>
  request("/audio-novels", {
    method: "POST",
    body: JSON.stringify(audioNovelPayload(data)),
  });
export const updateAudioNovel = (id, data) =>
  request(`/audio-novels/${id}`, {
    method: "PATCH",
    body: JSON.stringify(audioNovelPayload(data)),
  });
export const setAudioNovelEnabled = (id, enabled) =>
  request(`/audio-novels/${id}/status`, {
    method: "PATCH",
    body: JSON.stringify({ enabled }),
  });
export const setAudioNovelFeatured = (id, featured) =>
  request(`/audio-novels/${id}/featured`, {
    method: "PATCH",
    body: JSON.stringify({ featured }),
  });
export const deleteAudioNovel = (id) =>
  request(`/audio-novels/${id}`, { method: "DELETE" });
export const previewAudioNovelMarkdown = (body_markdown) =>
  request("/audio-novels/preview", {
    method: "POST",
    body: JSON.stringify({ body_markdown }),
  });
export function uploadAudioNovelCover(file) {
  const body = new FormData();
  body.append("file", file);
  return request("/audio-novel-covers", { method: "POST", body });
}
