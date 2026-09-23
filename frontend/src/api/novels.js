import { request } from "./http.js";

export function normalizeNovelSlug(value = "") {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^\p{L}\p{N}]+/gu, "-")
    .replace(/^-|-$/g, "");
}

export const NOVEL_TRANSLATION_LOCALES = [
  { code: "id", name: "印度尼西亚语" },
  { code: "ja", name: "日语" },
  { code: "ko", name: "韩语" },
  { code: "ms", name: "马来语" },
  { code: "pt", name: "葡萄牙语" },
  { code: "fil", name: "菲律宾语" },
  { code: "th", name: "泰语" },
  { code: "vi", name: "越南语" },
];
// 编辑页与 API 共用同一份语言代码，避免默认选择和提交白名单不一致。
export const NOVEL_TRANSLATION_LOCALE_CODES = NOVEL_TRANSLATION_LOCALES.map(
  ({ code }) => code,
);
const translationLocaleCodes = new Set(NOVEL_TRANSLATION_LOCALE_CODES);
export function translationLocalesPayload(locales = []) {
  return {
    locales: [
      ...new Set(
        locales.filter((locale) => translationLocaleCodes.has(locale)),
      ),
    ],
  };
}

// 只有尚无可用新译文的状态才提供单语言生成入口，避免活动任务或已发布译文被重复提交。
export function canGenerateNovelTranslation(status = "") {
  return (
    status === "failed" || status === "not_generated" || status === "stale"
  );
}

// 只提交后端允许的字段，避免把表格和弹窗的临时状态写入内容数据。
export function novelPayload(source) {
  return {
    title: source.title || "",
    slug: source.slug || "",
    author: source.author || "",
    category: source.category || "",
    excerpt: source.excerpt || "",
    cover_path: source.cover_path || "",
    published_at: source.published_at || "",
    enabled: Boolean(source.enabled),
    featured: Boolean(source.featured),
    sort_order: Number(source.sort_order) || 0,
  };
}
export function chapterPayload(source) {
  return {
    chapter_number: Number(source.chapter_number) || 0,
    title: source.title || "",
    body_markdown: source.body_markdown || "",
    enabled: Boolean(source.enabled),
  };
}
export function listNovels(filters = {}) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(filters))
    if (value !== "" && value != null) query.set(key, value);
  return request(`/novels?${query}`);
}
export const getNovel = (id) => request(`/novels/${id}`);
export const createNovel = (data) =>
  request("/novels", {
    method: "POST",
    body: JSON.stringify(novelPayload(data)),
  });
export const updateNovel = (id, data) =>
  request(`/novels/${id}`, {
    method: "PATCH",
    body: JSON.stringify(novelPayload(data)),
  });
export const deleteNovel = (id) =>
  request(`/novels/${id}`, { method: "DELETE" });
export const setNovelEnabled = (id, enabled) =>
  request(`/novels/${id}/status`, {
    method: "PATCH",
    body: JSON.stringify({ enabled }),
  });
export const setNovelFeatured = (id, featured) =>
  request(`/novels/${id}/featured`, {
    method: "PATCH",
    body: JSON.stringify({ featured }),
  });
export const listChapters = (id) => request(`/novels/${id}/chapters`);
export const createChapter = (id, data) =>
  request(`/novels/${id}/chapters`, {
    method: "POST",
    body: JSON.stringify(chapterPayload(data)),
  });
export const updateChapter = (id, chapterId, data) =>
  request(`/novels/${id}/chapters/${chapterId}`, {
    method: "PATCH",
    body: JSON.stringify(chapterPayload(data)),
  });
export const deleteChapter = (id, chapterId) =>
  request(`/novels/${id}/chapters/${chapterId}`, { method: "DELETE" });
export const previewNovelMarkdown = (body_markdown) =>
  request("/novels/preview", {
    method: "POST",
    body: JSON.stringify({ body_markdown }),
  });
export function uploadNovelCover(file) {
  const body = new FormData();
  body.append("file", file);
  return request("/novel-covers", { method: "POST", body });
}
export const setNovelCover = (id, cover_path) =>
  request(`/novels/${id}/cover`, {
    method: "PATCH",
    body: JSON.stringify({ cover_path }),
  });
// 已有小说在返回新路径前完成持久化，列表刷新时不会再读到旧封面。
export async function uploadAndPersistNovelCover(file, novelId) {
  const { path } = await uploadNovelCover(file);
  if (novelId) await setNovelCover(novelId, path);
  return path;
}
export const listNovelTranslations = (id) =>
  request(`/novels/${id}/translations`);
export const generateNovelTranslations = (id, locales) =>
  request(`/novels/${id}/translations`, {
    method: "POST",
    body: JSON.stringify(translationLocalesPayload(locales)),
  });
export const setNovelTranslationEnabled = (id, locale, enabled) =>
  request(`/novels/${id}/translations/${encodeURIComponent(locale)}/status`, {
    method: "PATCH",
    body: JSON.stringify({ enabled: Boolean(enabled) }),
  });
