// 语音小说前端独立构建，因此保留一份无框架依赖的可见时间实现。
export function formatVisibleTime(totalSeconds) {
  const seconds = Math.max(0, Math.floor(Number(totalSeconds) || 0));
  const minutes = Math.floor(seconds / 60);
  return `${String(minutes).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

export function createVisibleTimeTracker({ threshold = 0, onTick, onThreshold, documentRef = document, now = () => performance.now(), schedule = (fn) => setInterval(fn, 250), cancel = clearInterval } = {}) {
  let visibleStartedAt = documentRef.visibilityState === "visible" ? now() : null;
  let elapsed = 0;
  let lastSecond = -1;
  let thresholdSent = false;
  const emit = () => {
    const current = elapsed + (visibleStartedAt === null ? 0 : now() - visibleStartedAt);
    const seconds = Math.max(0, Math.floor(current / 1000));
    if (seconds !== lastSecond) {
      lastSecond = seconds;
      onTick?.(seconds);
    }
    const target = Number(threshold);
    if (!thresholdSent && Number.isInteger(target) && target > 0 && seconds >= target) {
      thresholdSent = true;
      onThreshold?.();
    }
  };
  const visibilityChanged = () => {
    if (documentRef.visibilityState === "visible") {
      if (visibleStartedAt === null) visibleStartedAt = now();
    } else if (visibleStartedAt !== null) {
      elapsed += now() - visibleStartedAt;
      visibleStartedAt = null;
    }
    emit();
  };
  documentRef.addEventListener("visibilitychange", visibilityChanged);
  const timer = schedule(emit);
  emit();
  return () => {
    cancel(timer);
    documentRef.removeEventListener("visibilitychange", visibilityChanged);
  };
}

export function trackMetaTimeSpent({ eventId, scope = window, documentRef = document } = {}) {
  const event = String(eventId || "");
  const state = documentRef.documentElement.dataset;
  if (!event || typeof scope.fbq !== "function" || state.audioNovelMetaTimeSpent === event) return false;
  state.audioNovelMetaTimeSpent = event;
  scope.fbq("trackCustom", "TimeSpent", {}, { eventID: event });
  return true;
}

export async function reportTimeSpent({ code, ticket, request = fetch, state = document.documentElement.dataset } = {}) {
  if (!ticket || state.audioNovelTimeSpentSent === "true") return false;
  state.audioNovelTimeSpentSent = "true";
  try {
    await request(`/audio-novel/${encodeURIComponent(code)}/time-spent`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ ticket }).toString(),
      keepalive: true,
    });
    return true;
  } catch {
    return false;
  }
}
