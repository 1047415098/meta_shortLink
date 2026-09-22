const key = (slug) => `novel-progress:${slug}`;
export function saveProgress(slug, chapterNumber, scrollY, storage = localStorage, now = Date.now()) {
  if (!slug || !Number.isInteger(Number(chapterNumber)) || Number(chapterNumber) < 1) return;
  storage.setItem(key(slug), JSON.stringify({ chapterNumber:Number(chapterNumber), scrollY:Math.max(0, Number(scrollY) || 0), updatedAt:Number(now) || Date.now() }));
}
export function readProgress(slug, availableChapters, storage = localStorage) {
  let value; try { value = JSON.parse(storage.getItem(key(slug))); } catch { return null; }
  if (!value || !Number.isInteger(value.chapterNumber) || value.chapterNumber < 1 || !Number.isFinite(value.scrollY) || !Number.isFinite(value.updatedAt)) return null;
  // 章节下线后回到第一章，避免保存进度导致阅读页无法打开。
  if (!availableChapters.includes(value.chapterNumber)) return availableChapters.length ? { chapterNumber:availableChapters[0], scrollY:0, updatedAt:value.updatedAt } : null;
  return { chapterNumber:value.chapterNumber, scrollY:Math.max(0, value.scrollY), updatedAt:value.updatedAt };
}
export const clearProgress = (slug, storage = localStorage) => storage.removeItem(key(slug));
