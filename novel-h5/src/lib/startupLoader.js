// The loader reaches a reassuring near-complete state quickly, then eases into readiness.
export const STARTUP_FAST_MS = 3000;
export const STARTUP_SLOW_MS = 7000;
export const STARTUP_TOTAL_MS = STARTUP_FAST_MS + STARTUP_SLOW_MS;

const startupCoverPattern = /^\/novel-uploads\/[a-f0-9]{32}\.(jpg|png|webp)$/;
// Existing campaign novels may retain a cover on this historical CDN during the upload migration.
const legacyStartupCoverPattern = /^https:\/\/cdn\.overseas-new-media\.com\/xiaoyao-writer\/prod\/content\/cover\/[A-Za-z0-9_-]+\.(?:jpg|png|webp)$/;

// The server injects this public path into the first document, so no extra API call is needed.
export function startupCoverPath(bootstrap) {
  const path = bootstrap?.startup_cover_path;
  return typeof path === "string" && (startupCoverPattern.test(path) || legacyStartupCoverPattern.test(path)) ? path : "";
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
  const cover = documentRef.getElementById("novel-startup-loader-cover");
  let bootstrap = null;
  try {
    bootstrap = JSON.parse(documentRef.getElementById("novel-h5-data")?.textContent || "null");
  } catch {
    // A malformed bootstrap keeps the neutral background; the app still renders its normal error view.
  }
  const coverPath = startupCoverPath(bootstrap);
  if (cover && coverPath) {
    // Reveal the cover only after a successful load so a deleted upload cannot show a broken image.
    cover.addEventListener("load", () => loader.classList.add("has-cover"), { once: true });
    cover.src = coverPath;
  }
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
