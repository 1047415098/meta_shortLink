export function createVisibleTimeTracker({ threshold = 0, onTick, onThreshold, documentRef = document } = {}) {
  let started = documentRef.visibilityState === "visible" ? performance.now() : null;
  let elapsed = 0;
  let thresholdSent = false;
  let lastSecond = -1;
  const emit = () => {
    const seconds = Math.floor((elapsed + (started === null ? 0 : performance.now() - started)) / 1000);
    if (seconds !== lastSecond) {
      lastSecond = seconds;
      onTick?.(seconds);
    }
    if (!thresholdSent && Number(threshold) > 0 && seconds >= Number(threshold)) {
      thresholdSent = true;
      onThreshold?.();
    }
  };
  const changed = () => {
    if (documentRef.visibilityState === "visible" && started === null) started = performance.now();
    if (documentRef.visibilityState !== "visible" && started !== null) {
      elapsed += performance.now() - started;
      started = null;
    }
    emit();
  };
  documentRef.addEventListener("visibilitychange", changed);
  const timer = window.setInterval(emit, 250);
  emit();
  return () => {
    window.clearInterval(timer);
    documentRef.removeEventListener("visibilitychange", changed);
  };
}

export async function postForm(path, values, { request = fetch, beacon = false, navigatorRef = globalThis.navigator } = {}) {
  const body = new URLSearchParams(values).toString();
  if (beacon && navigatorRef?.sendBeacon) {
    return { ok: navigatorRef.sendBeacon(path, new Blob([body], { type: "application/x-www-form-urlencoded" })), status: 0 };
  }
  try {
    const response = await request(path, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body, keepalive: true });
    const data = response.headers?.get?.("content-type")?.includes("application/json") ? await response.json() : null;
    return { ok: response.ok !== false, status: Number(response.status || 0), data };
  } catch {
    return { ok: false, status: 0, data: null };
  }
}
