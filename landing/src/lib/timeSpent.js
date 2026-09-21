// Format the visible duration without allowing an unbounded hour value to break the badge.
export function formatVisibleTime(totalSeconds) {
  const seconds = Math.max(0, Math.floor(Number(totalSeconds) || 0));
  const minutes = Math.floor(seconds / 60);
  return `${String(minutes).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

// Count only foreground time. Hidden tabs pause until the same document becomes visible again.
export function createVisibleTimeTracker({
  threshold = 0,
  onTick,
  onThreshold,
  documentRef = document,
  now = () => performance.now(),
  schedule = (fn) => setInterval(fn, 250),
  cancel = clearInterval,
} = {}) {
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

// Browser and server use the same custom-event ID so Meta can deduplicate them.
export function trackMetaTimeSpent({ eventId, scope = window, state = document.documentElement.dataset } = {}) {
  const event = String(eventId || "");
  if (!event || typeof scope.fbq !== "function" || state.metaTimeSpent === event) return false;
  state.metaTimeSpent = event;
  try {
    scope.fbq("trackCustom", "TimeSpent", {}, { eventID: event });
    return true;
  } catch {
    return false;
  }
}

// The signed request carries no client-provided duration; Gin verifies elapsed wall time itself.
export async function reportTimeSpent({ code, ticket, request = fetch, state = document.documentElement.dataset } = {}) {
  if (!ticket || state.timeSpentSent === "true") return false;
  state.timeSpentSent = "true";
  try {
    await request(`/${encodeURIComponent(code)}/time-spent`, {
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
