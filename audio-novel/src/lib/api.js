async function readJSON(url, request) {
  const response = await request(url, { headers: { Accept: "application/json" } });
  if (!response.ok) throw new Error(response.status === 410 ? "This literature archive is unavailable." : "The archive could not be loaded. Please try again.");
  return response.json();
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

