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

const TIKTOK_PIXEL_CODE = /^[A-Za-z0-9_-]{5,64}$/;
const TIKTOK_EVENT_ID = /^[^\s\r\n]{1,160}$/;

// Install TikTok's queue-compatible loader without sending a PageView. The
// browser event is emitted only after the signed server endpoint authorizes it.
export function installTikTokPixel({
  pixelCode,
  scope = window,
  document = window.document,
} = {}) {
  const pixel = String(pixelCode || "").trim();
  const state = document.documentElement?.dataset || {};
  if (!TIKTOK_PIXEL_CODE.test(pixel)) return false;
  if (state.shortTikTokPixel === pixel)
    return state.shortTikTokLoadFailed !== "true";
  if (state.shortTikTokPixel && state.shortTikTokPixel !== pixel) return false;
  try {
    if (!scope.ttq) {
      const queue = [];
      scope.TiktokAnalyticsObject = "ttq";
      queue.methods = [
        "page",
        "track",
        "identify",
        "instances",
        "debug",
        "on",
        "off",
        "once",
        "ready",
        "alias",
        "group",
        "enableCookie",
        "disableCookie",
      ];
      queue.setAndDefer = (target, method) => {
        target[method] = (...args) => target.push([method, ...args]);
      };
      for (const method of queue.methods) queue.setAndDefer(queue, method);
      queue.instance = (code) => {
        const instance = queue._i?.[code] || [];
        for (const method of queue.methods) queue.setAndDefer(instance, method);
        return instance;
      };
      queue.load = (code, options = {}) => {
        const base = "https://analytics.tiktok.com/i18n/pixel/events.js";
        // TikTok's SDK reads these official queue fields before replaying any
        // events produced while the remote script is still downloading.
        queue._i ||= {};
        queue._i[code] = [];
        queue._i[code]._u = base;
        queue._t ||= {};
        queue._t[code] = Date.now();
        queue._o ||= {};
        queue._o[code] = options;
        const script = document.createElement("script");
        script.id = "short-link-tiktok-pixel";
        script.async = true;
        script.src = `${base}?sdkid=${encodeURIComponent(code)}&lib=ttq`;
        script.onerror = () => {
          state.shortTikTokLoadFailed = "true";
        };
        document.head.appendChild(script);
      };
      scope.ttq = queue;
    }
    scope.ttq.load(pixel);
    state.shortTikTokPixel = pixel;
    return true;
  } catch {
    state.shortTikTokLoadFailed = "true";
    return false;
  }
}

// PageView, Contact and ViewContent reuse the event ID created by the server,
// allowing TikTok to deduplicate Pixel and Events API deliveries.
export function trackTikTokEvent({
  name,
  eventId,
  content,
  trigger,
  scope = window,
  document = window.document,
} = {}) {
  const event = String(name || "").trim();
  const id = String(eventId || "").trim();
  const state = document.documentElement?.dataset || {};
  const deliveryKey = `shortTikTok${event}`;
  if (
    !/^(PageView|Contact|ViewContent)$/.test(event) ||
    !TIKTOK_EVENT_ID.test(id) ||
    state.shortTikTokLoadFailed === "true" ||
    state[deliveryKey] === id
  )
    return false;
  try {
    const properties = {
      content_type: "product",
      content_name: content?.name || "",
      contents: content?.id
        ? [{ content_id: `short_link:${content.id}`, quantity: 1 }]
        : [],
    };
    if (trigger) properties.trigger = trigger;
    if (event === "PageView") {
      if (typeof scope.ttq?.page !== "function") return false;
      scope.ttq.page(properties, { event_id: id });
    } else {
      if (typeof scope.ttq?.track !== "function") return false;
      scope.ttq.track(event, properties, { event_id: id });
    }
    state[deliveryKey] = id;
    return true;
  } catch {
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
  if (!ticket || state.metaViewSent === "true") return null;
  state.metaViewSent = "true";
  try {
    const response = await request(`/${encodeURIComponent(code)}/view`, {
      method: "POST",
      // Mirror the trusted entry parameters for request-log correlation without changing attribution.
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        ...attributionHeaders,
      },
      body: new URLSearchParams({ ticket }).toString(),
      keepalive: true,
    });
    if (
      !response?.ok ||
      response.status === 204 ||
      typeof response.json !== "function"
    )
      return null;
    const payload = await response.json();
    return payload?.tiktok_event || null;
  } catch {
    // Measurement must never interrupt the visitor flow.
    return null;
  }
}
