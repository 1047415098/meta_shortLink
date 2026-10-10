// The loader reaches a reassuring near-complete state quickly, then uses each campaign link's tail duration.
export const STARTUP_FAST_MS = 3000;
export const STARTUP_DEFAULT_TAIL_MS = 5000;
const STARTUP_MIN_TAIL_MS = 1000;
const STARTUP_MAX_TAIL_MS = 60000;

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

// Only the final 10% is configurable; invalid or legacy bootstrap data stays on the five-second default.
export function startupTailMs(bootstrap) {
  const seconds = Number(bootstrap?.link?.startup_tail_seconds);
  if (!Number.isFinite(seconds) || seconds < 1 || seconds > 60) return STARTUP_DEFAULT_TAIL_MS;
  return Math.round(seconds * 1000);
}

export function isNovelStartupEntryPath(link, pathname = globalThis.location?.pathname || "") {
  const code = String(link?.code || "").trim();
  if (!code) return false;
  const entryPath = `/novel/${encodeURIComponent(code)}`;
  // 根短链使用 push 保留首页历史；分享的详情和章节地址继续使用 replace。
  return pathname === entryPath || pathname === `${entryPath}/`;
}

// Progress helpers may be called by tests or future UI code, so they receive the same safe fallback as bootstrap data.
function normalizedStartupTailMs(tailMs) {
  const value = Number(tailMs);
  return Number.isFinite(value) && value >= STARTUP_MIN_TAIL_MS && value <= STARTUP_MAX_TAIL_MS
    ? Math.round(value)
    : STARTUP_DEFAULT_TAIL_MS;
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

export function startupProgressAt(elapsedMs, tailMs = STARTUP_DEFAULT_TAIL_MS) {
  const elapsed = Math.max(0, Number(elapsedMs) || 0);
  const safeTailMs = normalizedStartupTailMs(tailMs);
  const totalMs = STARTUP_FAST_MS + safeTailMs;
  if (elapsed < STARTUP_FAST_MS) return Math.round((elapsed / STARTUP_FAST_MS) * 90);
  if (elapsed < totalMs) return 90 + Math.round(((elapsed - STARTUP_FAST_MS) / safeTailMs) * 10);
  // The configured visual duration completes the bar; readiness still controls when the layer closes.
  return 100;
}

export function canFinishStartupLoader(elapsedMs, contentReady, tailMs = STARTUP_DEFAULT_TAIL_MS) {
  const safeTailMs = normalizedStartupTailMs(tailMs);
  return Number(elapsedMs) >= STARTUP_FAST_MS + safeTailMs && Boolean(contentReady);
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
  // This first-document value prevents an API request from delaying the visual loading policy.
  const tailMs = startupTailMs(bootstrap);
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
    const progress = startupProgressAt(elapsed, tailMs);
    if (fill) fill.style.width = `${progress}%`;
    // Keep the numeric state available to assistive technology without showing a percentage.
    loader.setAttribute("aria-valuenow", String(progress));
    if (canFinishStartupLoader(elapsed, contentReady, tailMs)) return close();
    if (elapsed < STARTUP_FAST_MS + tailMs) frameId = windowRef.requestAnimationFrame(render);
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
