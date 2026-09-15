const defaults = {
  name: "",
  account_id: "",
  api_version: "v26.0",
};
const pixelDefaults = {
  name: "",
  connection_id: null,
  pixel_id: "",
  // New Pixel targets report both qualified landing views and consultations by
  // default; an operator can still disable either event family independently.
  enabled: true,
  pageview_enabled: true,
  manual_enabled: true,
  manual_event_name: "WhatsAppConsultClick",
  token_expires_at: null,
};
export function expiryDatePayload(value) {
  if (!value) return null;
  if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return `${value}T23:59:59Z`;
  return value;
}
export function expiryDateInput(value) {
  return value ? String(value).slice(0, 10) : null;
}
export function pixelForm(row = {}) {
  const form = {
    ...Object.fromEntries(
      Object.entries(pixelDefaults).map(([key, value]) => [
        key,
        row[key] ?? value,
      ]),
    ),
    capi_token: "",
    clear_capi_token: false,
  };
  form.token_expires_at = expiryDateInput(row.token_expires_at);
  return form;
}
export function pixelPayload(form, editing = false) {
  const payload = {};
  for (const [key, fallback] of Object.entries(pixelDefaults)) {
    if (editing && ["connection_id", "pixel_id"].includes(key)) continue;
    const value = form[key] ?? fallback;
    payload[key] =
      key === "token_expires_at"
        ? expiryDatePayload(value)
        : typeof value === "string"
          ? value.trim()
          : value;
  }
  if (form.clear_capi_token) payload.clear_capi_token = true;
  else if (form.capi_token?.trim()) payload.capi_token = form.capi_token.trim();
  return payload;
}

// A short link can target only a Pixel that is enabled and has a usable
// server-side credential; unverified credentials remain selectable for testing.
export function pixelSelectable(pixel, now = new Date()) {
  if (!pixel?.enabled || !pixel?.has_capi_token) return false;
  if (["invalid", "expired"].includes(pixel.credential_status)) return false;
  if (
    pixel.token_expires_at &&
    new Date(pixel.token_expires_at).getTime() <= now.getTime()
  )
    return false;
  return true;
}

// Keep account and Pixel selection coupled so one account can safely own many Pixels.
export function pixelsForConnection(pixels, connectionID) {
  if (!connectionID) return [];
  return pixels.filter(
    (pixel) => Number(pixel.connection_id) === Number(connectionID),
  );
}

// Auto-select only when there is exactly one valid destination; an ambiguous
// account deliberately requires the operator to choose.
export function preferredPixelID(
  pixels,
  connectionID,
  currentPixelID = null,
  now = new Date(),
) {
  const eligible = pixelsForConnection(pixels, connectionID).filter((pixel) =>
    pixelSelectable(pixel, now),
  );
  if (eligible.some((pixel) => pixel.id === currentPixelID))
    return currentPixelID;
  return eligible.length === 1 ? eligible[0].id : null;
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
export function pixelUnavailableReason(pixel, now = new Date()) {
  if (!pixel?.enabled) return "已停用";
  if (!pixel?.has_capi_token) return "未保存 CAPI Token";
  if (
    pixel.credential_status === "expired" ||
    (pixel.token_expires_at &&
      new Date(pixel.token_expires_at).getTime() <= now.getTime())
  )
    return "凭证已过期";
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
  return (
    {
      valid: { label: "有效", type: "success" },
      unverified: { label: "未验证", type: "info" },
      invalid: { label: "无效", type: "danger" },
      expired: { label: "已过期", type: "warning" },
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
