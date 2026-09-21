// Browser Pixel initialization lives beside the server PageView request so
// both delivery paths can share one visit-scoped event identifier.
export function installMetaPixel({
  pixelId,
  eventId,
  scope = window,
  document = window.document,
  state = window.document.documentElement.dataset,
} = {}) {
  const pixel = String(pixelId || "").trim();
  const event = String(eventId || "").trim();
  if (!/^\d{5,30}$/.test(pixel)) return false;

  let changed = false;

  if (!scope.fbq) {
    // This is Meta's queue-compatible loader contract; calls made before the
    // remote library finishes downloading are replayed by fbevents.js.
    const fbq = function () {
      if (fbq.callMethod) fbq.callMethod.apply(fbq, arguments);
      else fbq.queue.push(arguments);
    };
    scope.fbq = fbq;
    if (!scope._fbq) scope._fbq = fbq;
    fbq.push = fbq;
    fbq.loaded = true;
    fbq.version = "2.0";
    fbq.queue = [];

    if (!document.getElementById?.("meta-pixel-script")) {
      const script = document.createElement("script");
      script.id = "meta-pixel-script";
      script.async = true;
      script.src = "https://connect.facebook.net/en_US/fbevents.js";
      const firstScript = document.getElementsByTagName("script")[0];
      if (firstScript?.parentNode)
        firstScript.parentNode.insertBefore(script, firstScript);
      else document.head?.appendChild(script);
    }
    changed = true;
  }

  if (state.metaPixelInitialized !== pixel) {
    scope.fbq("init", pixel);
    state.metaPixelInitialized = pixel;
    changed = true;
  }
  if (event) {
    const deliveryKey = `${pixel}:${event}`;
    if (state.metaPixelPageView !== deliveryKey) {
      state.metaPixelPageView = deliveryKey;
      // Meta deduplicates this browser event against CAPI using the shared ID.
      scope.fbq("track", "PageView", {}, { eventID: event });
      changed = true;
    }
  }
  return changed;
}

export function trackMetaConsult({
  eventId,
  scope = window,
  state = window.document.documentElement.dataset,
} = {}) {
  const event = String(eventId || "").trim();
  if (!event || typeof scope.fbq !== "function") return false;
  if (state.metaPixelManualContact === event) return false;
  state.metaPixelManualContact = event;
  try {
    // Manual browser and server events share this ID so Meta counts one action.
    scope.fbq("track", "AddToCart", {}, { eventID: event });
    return true;
  } catch {
    // Measurement failures must never block the visitor's WhatsApp handoff.
    return false;
  }
}

export async function reportLandingView({
  code,
  ticket,
  request = fetch,
  state = document.documentElement.dataset,
  attributionHeaders = {},
}) {
  if (!ticket || state.metaViewSent === "true") return;
  state.metaViewSent = "true";
  try {
    await request(`/${encodeURIComponent(code)}/view`, {
      method: "POST",
      // Mirror the trusted entry parameters for request-log correlation without changing attribution.
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        ...attributionHeaders,
      },
      body: new URLSearchParams({ ticket }).toString(),
      keepalive: true,
    });
  } catch {
    // Measurement must never interrupt the visitor flow.
  }
}
