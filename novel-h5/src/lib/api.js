async function readJSON(url, request) {
  const response = await request(url, { headers: { Accept: "application/json" } });
  if (!response.ok) {
    if (response.status === 404) throw new Error("This story could not be found.");
    if (response.status === 410) throw new Error("This story archive is unavailable.");
    throw new Error("The story archive could not be loaded. Please try again.");
  }
  return response.json();
}
export async function fetchNovelHome(code, request = fetch) {
  const data = await readJSON(`/novel-api/${encodeURIComponent(code)}/home`, request);
  if (!("featured" in data) || !Array.isArray(data.ranking) || !Array.isArray(data.items)) throw new Error("The archive returned an incomplete response.");
  return data;
}
export async function fetchNovelList(code, { q = "", page = 1, pageSize = 20 } = {}, request = fetch) {
  const query = new URLSearchParams(); if (q) query.set("q", q); query.set("page", Number(page) || 1); query.set("page_size", Number(pageSize) || 20);
  const data = await readJSON(`/novel-api/${encodeURIComponent(code)}/stories?${query}`, request);
  if (!Array.isArray(data.items)) throw new Error("The archive returned an incomplete response.");
  return data;
}
export async function fetchNovelStory(code, slug, request = fetch) {
  const data = await readJSON(`/novel-api/${encodeURIComponent(code)}/stories/${encodeURIComponent(slug)}`, request);
  if (!data.story || !Array.isArray(data.chapters) || !Array.isArray(data.related)) throw new Error("The archive returned an incomplete response.");
  return data;
}
export async function fetchNovelChapter(code, slug, chapterNumber, request = fetch) {
  const data = await readJSON(`/novel-api/${encodeURIComponent(code)}/stories/${encodeURIComponent(slug)}/chapters/${Number(chapterNumber) || 1}`, request);
  if (!data.chapter || !("previous" in data) || !("next" in data)) throw new Error("The archive returned an incomplete response.");
  return data;
}
