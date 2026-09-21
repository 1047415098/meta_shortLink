export function installMetaPixel({ pixelId, eventId, scope = window, documentRef = document } = {}) {
  const pixel = String(pixelId || "").trim();
  if (!/^\d{5,30}$/.test(pixel)) return false;
  if (!scope.fbq) {
    // 保持 Meta 官方队列协议，远程脚本加载前的事件不会丢失。
    const fbq = function () { fbq.callMethod ? fbq.callMethod.apply(fbq, arguments) : fbq.queue.push(arguments); };
    Object.assign(fbq, { push: fbq, loaded: true, version: "2.0", queue: [] });
    scope.fbq = fbq;
    scope._fbq ||= fbq;
    const script = documentRef.createElement("script");
    script.id = "audio-novel-meta-pixel";
    script.async = true;
    script.src = "https://connect.facebook.net/en_US/fbevents.js";
    documentRef.head.appendChild(script);
  }
  const state = documentRef.documentElement.dataset;
  if (state.audioNovelMetaPixel !== pixel) {
    scope.fbq("init", pixel);
    state.audioNovelMetaPixel = pixel;
  }
  if (eventId && state.audioNovelMetaPageView !== eventId) {
    scope.fbq("track", "PageView", {}, { eventID: eventId });
    state.audioNovelMetaPageView = eventId;
  }
  return true;
}

export function trackMetaConsult(eventId, scope = window, documentRef = document) {
  if (!eventId || typeof scope.fbq !== "function") return false;
  const state = documentRef.documentElement.dataset;
  if (state.audioNovelMetaManual === eventId) return false;
  state.audioNovelMetaManual = eventId;
  scope.fbq("track", "AddToCart", {}, { eventID: eventId });
  return true;
}
