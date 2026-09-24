const defaults = {
  name: "",
  account_id: "",
};
// Graph requests use the backend-owned version below; operators cannot change
// protocol compatibility from an account form.
export const GRAPH_API_VERSION = "v26.0";
// Both consultation triggers use Meta's standard event; event ID suffixes keep
// their operational meaning distinct in the delivery log.
export const META_CONSULT_EVENT = "AddToCart";
// Keep one canonical template for every Meta ad so attribution fields never
// drift between operators or fall back to ambiguous legacy UTM meanings.
export const META_URL_PARAMETERS =
  "utm_source={{site_source_name}}&utm_medium=paid_social&utm_campaign={{campaign.name}}&utm_content={{ad.id}}&campaign_id={{campaign.id}}&campaign_name={{campaign.name}}&adset_id={{adset.id}}&adset_name={{adset.name}}&ad_id={{ad.id}}&ad_name={{ad.name}}&placement={{placement}}&site_source_name={{site_source_name}}";
const pixelDefaults = {
  name: "",
  connection_id: null,
  pixel_id: "",
  // New Pixel targets report both qualified landing views and consultations by
  // default; an operator can still disable either event family independently.
  enabled: true,
  pageview_enabled: true,
  manual_enabled: true,
  // Manual Contact and automatic redirect delivery are controlled separately.
  auto_enabled: true,
};
export function pixelForm(row = {}, capiToken = "") {
  return {
    ...Object.fromEntries(
      Object.entries(pixelDefaults).map(([key, value]) => [
        key,
        row[key] ?? value,
      ]),
    ),
    capi_token: capiToken,
    // Remember the fetched secret only in this edit form so unchanged values
    // are not sent back and marked unverified during unrelated edits.
    original_capi_token: capiToken,
  };
}
export function pixelPayload(form, editing = false) {
  const payload = {};
  for (const [key, fallback] of Object.entries(pixelDefaults)) {
    if (editing && ["connection_id", "pixel_id"].includes(key)) continue;
    const value = form[key] ?? fallback;
    payload[key] = typeof value === "string" ? value.trim() : value;
  }
  // Credential deletion is not a supported operation: edits either preserve
  // the stored token or replace it with a new non-empty value.
  if (
    form.capi_token?.trim() &&
    form.capi_token.trim() !== form.original_capi_token
  )
    payload.capi_token = form.capi_token.trim();
  return payload;
}

// A list-level pause changes only the master state; PATCH preserves the Pixel,
// credential and event rules already stored by the backend.
export function pixelTogglePayload(pixel) {
  return { enabled: !pixel?.enabled };
}

// A short link can target only a Pixel that is enabled and has a usable
// server-side credential; unverified credentials remain selectable for testing.
export function pixelSelectable(pixel) {
  if (!pixel?.enabled || !pixel?.has_capi_token) return false;
  if (pixel.credential_status === "invalid") return false;
  return true;
}

// Keep account and Pixel selection coupled so one account can safely own many Pixels.
export function pixelsForConnection(pixels, connectionID) {
  if (!connectionID) return [];
  return pixels.filter(
    (pixel) => Number(pixel.connection_id) === Number(connectionID),
  );
}

// A short link stores both foreign keys, but the operator only needs to choose
// the concrete Pixel destination; its owning account is derived here.
export function connectionIDForPixel(pixels, pixelID) {
  if (!pixelID) return null;
  return (
    pixels.find((pixel) => Number(pixel.id) === Number(pixelID))
      ?.connection_id ?? null
  );
}

// Explain why a visible Pixel cannot receive link events without exposing its token.
export function pixelUnavailableReason(pixel) {
  if (!pixel?.enabled) return "已停用";
  if (!pixel?.has_capi_token) return "未保存 CAPI Token";
  if (pixel.credential_status === "invalid") return "凭证无效";
  return "";
}
export function buildMetaTrackingURL(base, values) {
  const url = new URL(base);
  const sensitive = /(token|secret|password|authorization|api_key)/i;
  if (
    url.username ||
    url.password ||
    [...url.searchParams.keys()].some((key) => sensitive.test(key))
  )
    throw new Error("链接包含凭证信息，请先移除后重试");
  for (const key of [
    "utm_source",
    "site_source_name",
    "campaign_id",
    "adset_id",
    "ad_id",
  ])
    if (values[key]?.trim()) url.searchParams.set(key, values[key].trim());
  return url.toString();
}
export function credentialStatus(status) {
  // Legacy expiry markers are shown as unverified because the application no
  // longer maintains a separate operator-entered expiry date.
  if (status === "expired") status = "unverified";
  return (
    {
      valid: { label: "有效", type: "success" },
      unverified: { label: "未验证", type: "info" },
      invalid: { label: "无效", type: "danger" },
      missing: { label: "未配置", type: "info" },
    }[status] || { label: status || "未配置", type: "info" }
  );
}
export function connectionForm(row = {}) {
  // Account forms expose grouping metadata only; all secrets belong to Pixels.
  return Object.fromEntries(
    Object.entries(defaults).map(([key, value]) => [key, row[key] ?? value]),
  );
}
export function connectionPayload(form, editing = false) {
  const payload = {};
  for (const [key, fallback] of Object.entries(defaults)) {
    if (editing && key === "account_id") continue;
    const value = form[key] ?? fallback;
    payload[key] = typeof value === "string" ? value.trim() : value;
  }
  return payload;
}
export const statusLabels = {
  pending: "等待发送",
  processing: "发送中",
  succeeded: "Meta 已接收",
  retry: "等待重试",
  failed: "失败",
  expired: "已过期",
  skipped: "已跳过",
};
export function metaEventLabel(event = {}) {
  // Audio events keep the raw Meta name visible while identifying the product funnel.
  if (event.audio_novel_id) {
    if (event.event_name === "PageView") return "语音小说 · 浏览";
    if (event.event_name === "StartListening") return "语音小说 · 开始收听";
    if (event.event_name === "ViewContent") return "语音小说 · 播放达标";
  }
  // Historical names remain readable after the live funnel switches to the
  // standard AddToCart event.
  if (event.event_name === "PageView") return "浏览事件";
  if (event.event_name === "WhatsAppAutoRedirect") return "自动跳转（历史）";
  if (
    event.event_name === "WhatsAppConsultClick" ||
    event.event_name === "Contact"
  )
    return "手动咨询（历史）";
  if (event.event_name === META_CONSULT_EVENT)
    return event.id?.endsWith("_auto")
      ? "自动跳转 · AddToCart"
      : "手动咨询 · AddToCart";
  return "其他事件";
}
export function statusType(status) {
  if (status === "succeeded") return "success";
  if (status === "failed") return "danger";
  if (["retry", "expired"].includes(status)) return "warning";
  return "info";
}
export function timestamp(value) {
  if (!value) return "—";
  return new Date(value).toLocaleString("zh-CN", { hour12: false });
}
