export const STARTUP_FAST_MS = 5000;
export const STARTUP_SLOW_MS = 3000;
export const STARTUP_TOTAL_MS = STARTUP_FAST_MS + STARTUP_SLOW_MS;

export function startupProgressAt(elapsedMs, contentReady) {
  const elapsed = Math.max(0, Number(elapsedMs) || 0);
  if (elapsed < STARTUP_FAST_MS) return Math.round((elapsed / STARTUP_FAST_MS) * 90);
  if (elapsed < STARTUP_TOTAL_MS) return 90 + Math.round(((elapsed - STARTUP_FAST_MS) / STARTUP_SLOW_MS) * 10);
  // 内容未就绪时最多显示 99%，避免进度已满但页面仍为空白。
  return contentReady ? 100 : 99;
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
  const label = documentRef.getElementById("novel-startup-loader-progress");
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
    if (label) label.textContent = `${progress}%`;
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
