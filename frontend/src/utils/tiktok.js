const connectionDefaults = {
  name: "",
  enabled: true,
  access_token: "",
};

const pixelDefaults = {
  connection_id: null,
  name: "",
  pixel_code: "",
  test_event_code: "",
  enabled: true,
};

export function tiktokConnectionForm(row = {}) {
  return Object.fromEntries(
    Object.entries(connectionDefaults).map(([key, value]) => [
      key,
      key === "access_token" ? "" : (row[key] ?? value),
    ]),
  );
}

export function tiktokConnectionPayload(form = {}, editing = false) {
  const payload = {
    name: String(form.name || "").trim(),
    enabled: Boolean(form.enabled),
  };
  const token = String(form.access_token || "").trim();
  // 新增必须传 Token；编辑留空代表继续使用原凭证。
  if (!editing || token) payload.access_token = token;
  return payload;
}

export function tiktokPixelForm(row = {}) {
  return Object.fromEntries(
    Object.entries(pixelDefaults).map(([key, value]) => [
      key,
      row[key] ?? value,
    ]),
  );
}

export function tiktokPixelPayload(form = {}) {
  return {
    connection_id: Number(form.connection_id) || 0,
    name: String(form.name || "").trim(),
    pixel_code: String(form.pixel_code || "").trim(),
    test_event_code: String(form.test_event_code || "").trim(),
    enabled: Boolean(form.enabled),
  };
}

export function tiktokTemplateURL(publicURL, code, productPrefix = "novel") {
  // Both content projects share TikTok macro names while retaining distinct public paths.
  const prefix = productPrefix === "audio-novel" ? "audio-novel" : "novel";
  const url = new URL(`/${prefix}/${encodeURIComponent(code)}`, publicURL);
  // TikTok 宏在广告点击时替换，模板只负责统一参数名称。
  const fields = {
    utm_source: "tiktok",
    utm_medium: "paid_social",
    campaign_id: "__CAMPAIGN_ID__",
    adgroup_id: "__AID__",
    creative_id: "__CID__",
    ad_id_v2: "__ADID_V2__",
    placement: "__PLACEMENT__",
  };
  for (const [key, value] of Object.entries(fields))
    url.searchParams.set(key, value);
  return url.toString();
}

export const tiktokEventStatuses = [
  "pending",
  "sending",
  "accepted",
  "retry",
  "failed",
];

export const tiktokStatusLabels = {
  pending: "已保存/待发送",
  sending: "发送中",
  accepted: "TikTok 已接收",
  retry: "等待重试",
  failed: "发送失败",
};

export function tiktokStatusType(status) {
  if (status === "accepted") return "success";
  if (status === "failed") return "danger";
  if (status === "retry") return "warning";
  return "info";
}

export function localTimestamp(value) {
  if (!value) return "—";
  return new Date(value).toLocaleString("zh-CN", { hour12: false });
}
