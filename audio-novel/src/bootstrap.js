const fallback = {
  // Vite 独立开发时提供可浏览数据；生产页面始终由 Gin 注入真实配置。
  link: import.meta.env?.DEV ? { code: "hello", target_url: "https://wa.me/10000000000" } : null,
  ticket: "",
  surface: "audio_novel",
  cookie_enabled: false,
  meta_measurement: false,
  error: null
};

export function readBootstrap(documentRef = document) {
  const node = documentRef.getElementById("audio-novel-data");
  if (!node?.textContent) return fallback;
  try {
    return { ...fallback, ...JSON.parse(node.textContent) };
  } catch {
    return { ...fallback, error: { status: 503, message: "The archive could not be opened." } };
  }
}
