const fallback = {
  link: import.meta.env?.DEV ? { code: "cover-demo", time_spent_threshold: 10 } : null,
  ticket: "",
  surface: "cover",
  ad_platform: "meta",
  meta_measurement: false,
  tiktok_enabled: false,
  locale: "en",
  available_locales: ["en"],
  error: null,
};

export function readBootstrap(documentRef = document) {
  const node = documentRef.getElementById("cover-h5-data");
  if (!node?.textContent) return fallback;
  try {
    return { ...fallback, ...JSON.parse(node.textContent) };
  } catch {
    return { ...fallback, error: { status: 503, message: "This cover experience is temporarily unavailable." } };
  }
}
