export const STORAGE_KEY = "audio-novel-reading-size";
export const MIN_SIZE = 14;
export const DEFAULT_SIZE = 18;
export const MAX_SIZE = 24;

export function clampReadingSize(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return DEFAULT_SIZE;
  return Math.min(MAX_SIZE, Math.max(MIN_SIZE, parsed));
}

export function readSavedSize(storage = localStorage) {
  return clampReadingSize(storage.getItem(STORAGE_KEY) || DEFAULT_SIZE);
}

export function saveReadingSize(value, storage = localStorage) {
  const size = clampReadingSize(value);
  storage.setItem(STORAGE_KEY, String(size));
  return size;
}
