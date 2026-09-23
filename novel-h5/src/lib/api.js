import { translate } from "./i18n.js";
async function readJSON(url, request, locale="en") {
  // 小说 API 统一从请求头读取当前语言，页面地址不再携带 lang 参数。
  const response = await request(url, { headers: { Accept:"application/json", "X-Novel-Language":locale } });
  if (!response.ok) {
    if (response.status === 404) throw new Error(translate(locale,"notFound"));
    if (response.status === 410) throw new Error(translate(locale,"restingMessage"));
    throw new Error(translate(locale,"unavailable"));
  }
  return response.json();
}
export async function fetchNovelHome(code, request = fetch, locale="en") {
  const data = await readJSON(`/novel-api/${encodeURIComponent(code)}/home`, request, locale);
  if (!("featured" in data) || !Array.isArray(data.ranking) || !Array.isArray(data.items)) throw new Error(translate(locale,"incomplete"));
  return data;
}
export async function fetchNovelList(code, { q = "", page = 1, pageSize = 20, locale="en" } = {}, request = fetch) {
  const query = new URLSearchParams(); if (q) query.set("q", q); query.set("page", Number(page) || 1); query.set("page_size", Number(pageSize) || 20);
  const data = await readJSON(`/novel-api/${encodeURIComponent(code)}/stories?${query}`, request, locale);
  if (!Array.isArray(data.items)) throw new Error(translate(locale,"incomplete"));
  return data;
}
export async function fetchNovelStory(code, slug, request = fetch, locale="en") {
  const data = await readJSON(`/novel-api/${encodeURIComponent(code)}/stories/${encodeURIComponent(slug)}`, request, locale);
  if (!data.story || !Array.isArray(data.chapters) || !Array.isArray(data.related)) throw new Error(translate(locale,"incomplete"));
  return data;
}
export async function fetchNovelChapter(code, slug, chapterNumber, request = fetch, locale="en") {
  const data = await readJSON(`/novel-api/${encodeURIComponent(code)}/stories/${encodeURIComponent(slug)}/chapters/${Number(chapterNumber) || 1}`, request, locale);
  if (!data.chapter || !("previous" in data) || !("next" in data)) throw new Error(translate(locale,"incomplete"));
  return data;
}
