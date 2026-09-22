export function storyPath(code, slug) {
  return `/audio-novel/${encodeURIComponent(code)}/stories/${encodeURIComponent(slug)}`;
}

export function audioListPath(code) {
  return `/audio-novel/${encodeURIComponent(code)}/audio`;
}

export function audioDetailPath(code, slug) {
  return `${audioListPath(code)}/${encodeURIComponent(slug)}`;
}
