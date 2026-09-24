const sentAudioEventIDs = new WeakMap();

function documentEvents(documentRef) {
  let events = sentAudioEventIDs.get(documentRef);
  if (!events) {
    events = new Set();
    sentAudioEventIDs.set(documentRef, events);
  }
  return events;
}

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
  if (eventId) trackMetaAudioEvent({ name: "PageView", eventId, scope, documentRef });
  return true;
}

export function trackMetaAudioEvent({ name, eventId, content, scope = window, documentRef = document } = {}) {
  const event = String(name || "").trim();
  const id = String(eventId || "").trim();
  const sent = documentEvents(documentRef);
  if (!/^(PageView|StartListening|ViewContent)$/.test(event) || !/^[^\s\r\n]{1,160}$/.test(id) || typeof scope.fbq !== "function" || sent.has(id)) return false;
  try {
    // Match the server payload exactly so browser/server event deduplication
    // describes the same audio content type on both delivery channels.
    const parameters = { content_type: "audio_novel" };
    if (content?.id) {
      parameters.content_ids = [`audio_novel:${content.id}`];
      parameters.content_name = content.title || "";
    }
    scope.fbq(event === "StartListening" ? "trackCustom" : "track", event, parameters, { eventID: id });
    sent.add(id);
    return true;
  } catch {
    return false;
  }
}

export function trackMetaConsult(eventId, scope = window, documentRef = document) {
  if (!eventId || typeof scope.fbq !== "function") return false;
  const state = documentRef.documentElement.dataset;
  if (state.audioNovelMetaManual === eventId) return false;
  state.audioNovelMetaManual = eventId;
  scope.fbq("track", "AddToCart", {}, { eventID: eventId });
  return true;
}
