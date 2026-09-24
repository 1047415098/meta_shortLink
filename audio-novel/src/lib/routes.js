export function storyPath(code, slug) {
  return `/audio-novel/${encodeURIComponent(code)}/stories/${encodeURIComponent(slug)}`;
}

export function audioListPath(code) {
  return `/audio-novel/${encodeURIComponent(code)}/audio`;
}

export function audioDetailPath(code, slug) {
  return `${audioListPath(code)}/${encodeURIComponent(slug)}`;
}

export function audioEntryLocation(bootstrap, route) {
  if (bootstrap?.error || !bootstrap?.link?.code || !bootstrap?.entry_audio_slug || route?.name !== "home") return null;
  // replace 时保留 TikTok/Meta 动态参数，入口归因和刷新后的地址都保持完整。
  return { path: audioDetailPath(bootstrap.link.code, bootstrap.entry_audio_slug), query: { ...(route.query || {}) } };
}
