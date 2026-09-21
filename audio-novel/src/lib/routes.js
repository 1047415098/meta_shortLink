export function storyPath(code, slug) {
  return `/audio-novel/${encodeURIComponent(code)}/stories/${encodeURIComponent(slug)}`;
}
