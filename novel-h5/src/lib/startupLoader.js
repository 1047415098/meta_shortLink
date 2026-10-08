// The loader reaches a reassuring near-complete state quickly, then eases into readiness.
export const STARTUP_FAST_MS = 3000;
export const STARTUP_SLOW_MS = 7000;
export const STARTUP_TOTAL_MS = STARTUP_FAST_MS + STARTUP_SLOW_MS;

const startupCoverPattern = /^\/novel-uploads\/[a-f0-9]{32}\.(jpg|png|webp)$/;
// Existing campaign novels may retain a cover on this historical CDN during the upload migration.
const legacyStartupCoverPattern = /^https:\/\/cdn\.overseas-new-media\.com\/xiaoyao-writer\/prod\/content\/cover\/[A-Za-z0-9_-]+\.(?:jpg|png|webp)$/;
// A substantial preview fills the paper-like loading view without embedding a full chapter.
const startupPreviewMaxRunes = 1900;

// The server injects this public path into the first document, so no extra API call is needed.
export function startupCoverPath(bootstrap) {
  const path = bootstrap?.startup_cover_path;
  return typeof path === "string" && (startupCoverPattern.test(path) || legacyStartupCoverPattern.test(path)) ? path : "";
}

// The server supplies a short, plain-text chapter excerpt in the selected reading language.
export function startupExcerpt(bootstrap) {
  const source = bootstrap?.startup_excerpt;
  if (typeof source !== "string") return "";
  const text = source.replace(/\s+/g, " ").trim();
  const runes = Array.from(text);
  return runes.length > startupPreviewMaxRunes ? `${runes.slice(0, startupPreviewMaxRunes).join("")}…` : text;
}

export function startupStoryRoute(pathname = "") {
  // 仅接受小说详情/章节路由，避免加载层根据任意地址发起公开内容请求。
  const match = String(pathname).match(/^\/novel\/([A-Za-z0-9_-]{3,40})\/stories\/([^/]+)(?:\/chapters\/\d+)?$/);
  if (!match) return null;
  try {
    return { code: match[1], slug: decodeURIComponent(match[2]) };
  } catch {
    return null;
  }
}

function plainStartupText(html) {
  // API 返回的是已净化的章节 HTML；加载层始终只写入 textContent，不渲染其中标记。
  return String(html || "").replace(/<[^>]*>/g, " ").replace(/&nbsp;/gi, " ").replace(/&amp;/gi, "&").replace(/&lt;/gi, "<").replace(/&gt;/gi, ">");
}

export async function fetchStartupExcerpt(pathname, { locale = "", request = fetch } = {}) {
  const route = startupStoryRoute(pathname);
  if (!route || typeof request !== "function") return "";
  const headers = { Accept: "application/json" };
  if (typeof locale === "string" && locale) headers["X-Novel-Language"] = locale;
  try {
    // 开发服务器没有后端注入的 Bootstrap 时，只读取首个可读章节的一小段公开预览。
    const storyResponse = await request(`/novel-api/${encodeURIComponent(route.code)}/stories/${encodeURIComponent(route.slug)}`, { headers });
    if (!storyResponse.ok) return "";
    const storyData = await storyResponse.json();
    const chapterNumber = Number(storyData?.chapters?.[0]?.chapter_number);
    if (!Number.isInteger(chapterNumber) || chapterNumber < 1) return "";
    const chapterResponse = await request(`/novel-api/${encodeURIComponent(route.code)}/stories/${encodeURIComponent(route.slug)}/chapters/${chapterNumber}`, { headers });
    if (!chapterResponse.ok) return "";
    const chapterData = await chapterResponse.json();
    return startupExcerpt({ startup_excerpt: plainStartupText(chapterData?.chapter?.body_html) });
  } catch {
    // 预览失败不影响正常页面请求，加载层保持原有的中性背景。
    return "";
  }
}

export function startupProgressAt(elapsedMs, contentReady) {
  const elapsed = Math.max(0, Number(elapsedMs) || 0);
  if (elapsed < STARTUP_FAST_MS) return Math.round((elapsed / STARTUP_FAST_MS) * 90);
  if (elapsed < STARTUP_TOTAL_MS) return 90 + Math.round(((elapsed - STARTUP_FAST_MS) / STARTUP_SLOW_MS) * 10);
  // Ten seconds always completes the visible bar; readiness still controls when the layer closes.
  return 100;
}

export function canFinishStartupLoader(elapsedMs, contentReady) {
  return Number(elapsedMs) >= STARTUP_TOTAL_MS && Boolean(contentReady);
}

export function markNovelStartupReady(windowRef = window) {
  windowRef.__novelStartupReady = true;
  windowRef.dispatchEvent(new Event("novel-h5-ready"));
}

export function installStartupLoader({ documentRef = document, windowRef = window, now = () => performance.now() } = {}) {
  const loader = documentRef.getElementById("novel-startup-loader");
  if (!loader) return () => {};

  const fill = documentRef.getElementById("novel-startup-loader-fill");
  const excerptLayer = documentRef.getElementById("novel-startup-loader-text");
  let bootstrap = null;
  try {
    bootstrap = JSON.parse(documentRef.getElementById("novel-h5-data")?.textContent || "null");
  } catch {
    // A malformed bootstrap keeps the neutral loading background.
  }
  function showExcerpt(excerpt) {
    if (!excerpt || !excerptLayer) return;
    // A single preview keeps the loading layer visually consistent with the real reader page.
    excerptLayer.textContent = excerpt;
    loader.classList.add("has-excerpt");
  }
  const excerpt = startupExcerpt(bootstrap);
  showExcerpt(excerpt);
  if (!excerpt && typeof windowRef.fetch === "function") {
    // Vite 直开详情页没有服务端 HTML 注入，异步补齐同一份受限正文预览。
    void fetchStartupExcerpt(windowRef.location?.pathname, { locale: bootstrap?.locale, request: windowRef.fetch.bind(windowRef) }).then(showExcerpt);
  }
  // Temporary: leave the cover-background hook dormant while the text-content visual is being evaluated.
  // const cover = documentRef.getElementById("novel-startup-loader-cover");
  // const coverPath = startupCoverPath(bootstrap);
  // if (cover && coverPath) {
  //   cover.addEventListener("load", () => loader.classList.add("has-cover"), { once: true });
  //   cover.src = coverPath;
  // }
  const startedAt = Number(windowRef.__novelStartupStartedAt) || now();
  let contentReady = Boolean(windowRef.__novelStartupReady);
  let frameId;
  let closed = false;

  function close() {
    if (closed) return;
    closed = true;
    loader.classList.add("is-leaving");
    // 动画结束后移除节点，后续 SPA 路由不会再次显示首屏加载层。
    windowRef.setTimeout(() => loader.remove(), 260);
  }

  function render() {
    // 已执行的动画帧不再算作待执行帧，内容稍后就绪时才能立即收尾。
    frameId = undefined;
    const elapsed = now() - startedAt;
    const progress = startupProgressAt(elapsed, contentReady);
    if (fill) fill.style.width = `${progress}%`;
    // Keep the numeric state available to assistive technology without showing a percentage.
    loader.setAttribute("aria-valuenow", String(progress));
    if (canFinishStartupLoader(elapsed, contentReady)) return close();
    if (elapsed < STARTUP_TOTAL_MS) frameId = windowRef.requestAnimationFrame(render);
  }

  function onContentReady() {
    contentReady = true;
    if (!frameId) render();
  }

  windowRef.addEventListener("novel-h5-ready", onContentReady);
  render();
  return () => {
    if (frameId) windowRef.cancelAnimationFrame(frameId);
    windowRef.removeEventListener("novel-h5-ready", onContentReady);
  };
}
