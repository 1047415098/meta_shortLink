const PIXEL_CODE = /^[A-Za-z0-9_-]{5,64}$/;
const EVENT_ID = /^[^\s\r\n]{1,160}$/;
const sentEventIDs = new WeakMap();

function documentEvents(documentRef) {
  let events = sentEventIDs.get(documentRef);
  if (!events) {
    events = new Set();
    sentEventIDs.set(documentRef, events);
  }
  return events;
}

function installQueue(scope, documentRef) {
  if (scope.ttq) return scope.ttq;
  const queue = [];
  scope.TiktokAnalyticsObject = "ttq";
  // 保留 TikTok 官方队列结构，SDK 下载前产生的事件会在加载后按序重放。
  queue.methods = ["page", "track", "identify", "instances", "debug", "on", "off", "once", "ready", "alias", "group", "enableCookie", "disableCookie"];
  queue.setAndDefer = (target, method) => { target[method] = (...args) => target.push([method, ...args]); };
  for (const method of queue.methods) queue.setAndDefer(queue, method);
  queue.instance = (pixel) => {
    const instance = queue._i?.[pixel] || [];
    for (const method of queue.methods) queue.setAndDefer(instance, method);
    return instance;
  };
  queue.load = (pixel, options = {}, configureScript) => {
    const base = "https://analytics.tiktok.com/i18n/pixel/events.js";
    queue._i ||= {};
    queue._i[pixel] = [];
    queue._i[pixel]._u = base;
    queue._t ||= {};
    queue._t[pixel] = Date.now();
    queue._o ||= {};
    queue._o[pixel] = options;
    const script = documentRef.createElement("script");
    script.type = "text/javascript";
    script.async = true;
    script.src = `${base}?sdkid=${encodeURIComponent(pixel)}&lib=ttq`;
    configureScript?.(script);
    documentRef.head.appendChild(script);
    return script;
  };
  scope.ttq = queue;
  return queue;
}

export function installTikTokPixel({ pixelCode, scope = window, documentRef = document } = {}) {
  const pixel = String(pixelCode || "").trim();
  if (!PIXEL_CODE.test(pixel)) return false;
  const state = documentRef.documentElement?.dataset || {};
  if (state.audioNovelTikTokPixel === pixel) return state.audioNovelTikTokLoadFailed !== "true";
  if (state.audioNovelTikTokPixel && state.audioNovelTikTokPixel !== pixel) return false;
  try {
    const ttq = installQueue(scope, documentRef);
    ttq.load(pixel, {}, (script) => {
      script.id = "audio-novel-tiktok-pixel";
      script.onerror = () => { state.audioNovelTikTokLoadFailed = "true"; };
    });
    state.audioNovelTikTokPixel = pixel;
    return true;
  } catch {
    state.audioNovelTikTokLoadFailed = "true";
    return false;
  }
}

export function trackTikTokAudioEvent({ name, eventId, content, scope = window, documentRef = document } = {}) {
  const event = String(name || "").trim();
  const id = String(eventId || "").trim();
  const state = documentRef.documentElement?.dataset || {};
  const sent = documentEvents(documentRef);
  if (!/^(PageView|StartListening|ViewContent)$/.test(event) || !EVENT_ID.test(id) || state.audioNovelTikTokLoadFailed === "true" || sent.has(id)) return false;
  try {
    const properties = {
      // Keep browser properties identical to the server-side audio event.
      content_type: "audio_novel",
      contents: content?.id ? [{ content_id: `audio_novel:${content.id}`, content_name: content.title || "" }] : [],
    };
    if (event === "PageView") {
      if (typeof scope.ttq?.page !== "function") return false;
      scope.ttq.page(properties, { event_id: id });
    } else {
      if (typeof scope.ttq?.track !== "function") return false;
      scope.ttq.track(event, properties, { event_id: id });
    }
    sent.add(id);
    return true;
  } catch {
    return false;
  }
}

export function readTikTokTTP(documentRef = document) {
  const pair = String(documentRef.cookie || "").split(";").map((item) => item.trim()).find((item) => item.startsWith("_ttp="));
  if (!pair) return "";
  try {
    const value = decodeURIComponent(pair.slice(5)).trim();
    const unresolved = (value.length > 4 && value.startsWith("__") && value.endsWith("__")) || value.includes("{{") || value.includes("}}");
    if (!value || value.length > 512 || /[\r\n]/.test(value) || unresolved) return "";
    return value;
  } catch {
    return "";
  }
}
