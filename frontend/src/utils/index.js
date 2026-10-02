export function buildQuery(values) {
  return new URLSearchParams(
    Object.entries(values).filter(
      ([, v]) => v !== "" && v !== null && v !== undefined,
    ),
  ).toString();
}
export function unitCost(cost, count) {
  return Number(count) > 0 ? (Number(cost) / Number(count)).toFixed(2) : "—";
}
// Build a stable three-level GeoIP label while keeping incomplete lookups visible.
export function locationLabel(location = {}) {
  // The collector stores the literal value "unknown" when GeoIP cannot resolve a level.
  const known = (value, fallback) =>
    value && String(value).toLowerCase() !== "unknown" ? value : fallback;
  return [
    known(location.country, "未知国家"),
    known(location.region, "未知州省"),
    known(location.city, "未知城市"),
  ].join(" / ");
}
// Sum the visit-based GeoIP buckets so the UI can expose an explicit check
// against the ad row total without confusing visits with unique visitors.
export function locationVisitTotal(locations = []) {
  return locations.reduce(
    (total, location) => total + Number(location?.visits || 0),
    0,
  );
}
export function validateLink(link) {
  // Zero disables TimeSpent; enabled values match the backend's safe whole-second range.
  if (
    link.time_spent_threshold !== undefined &&
    (!Number.isInteger(link.time_spent_threshold) ||
      (link.time_spent_threshold !== 0 &&
        (link.time_spent_threshold < 5 || link.time_spent_threshold > 3600)))
  )
    return "TimeSpent 阈值请输入 0，或 5–3600 的整数秒数";
  if (
    link.landing_delay !== undefined &&
    (!Number.isInteger(link.landing_delay) ||
      link.landing_delay < 0 ||
      link.landing_delay > 300)
  )
    return "定时跳转请输入 0–300 的整数秒数";
  if (
    link.mode === "landing" &&
    (!link.landing_brand?.trim() ||
      !link.landing_title?.trim() ||
      !link.landing_description?.trim())
  )
    return "请填写落地页品牌、标题和产品简介";
  if (!link.name?.trim()) return "请输入链接名称";
  // Every saved link targets exactly one platform so one visit can never be
  // delivered to both Meta and TikTok by accident.
  const platform = link.ad_platform || "meta";
  const hasMeta =
    Number.isInteger(Number(link.meta_pixel_id)) &&
    Number(link.meta_pixel_id) > 0;
  const hasTikTok =
    Number.isInteger(Number(link.tiktok_pixel_id)) &&
    Number(link.tiktok_pixel_id) > 0;
  if ((platform === "meta" && hasTikTok) || (platform === "tiktok" && hasMeta))
    return "一个短链接只能绑定一个广告平台";
  if (platform === "meta" && !hasMeta) return "请选择 Meta Pixel";
  if (platform === "tiktok" && !hasTikTok) return "请选择 TikTok Pixel";
  if (!["meta", "tiktok"].includes(platform)) return "请选择广告平台";
  if (!["bound", "dynamic"].includes(link.attribution_mode))
    return "请选择广告归因方式";
  if (platform === "tiktok" && link.attribution_mode !== "dynamic")
    return "TikTok 短链接仅支持动态归因";
  try {
    const url = new URL(link.target_url);
    if (
      url.protocol !== "https:" ||
      url.hostname !== "wa.me" ||
      url.port ||
      url.username ||
      url.password ||
      !/^\/[1-9]\d{5,14}$/.test(url.pathname)
    )
      return "请输入合法的 https://wa.me/国际号码 地址";
  } catch {
    return "请输入合法的 WhatsApp 地址";
  }
  return "";
}

// Normalize the admin form at the API boundary so stale hidden fields cannot
// create a mixed Meta/TikTok binding.
export function shortLinkPayload(form = {}) {
  const payload = { ...form };
  payload.name = String(payload.name || "").trim();
  payload.target_url = String(payload.target_url || "").trim();
  payload.ad_platform = payload.ad_platform === "tiktok" ? "tiktok" : "meta";
  if (payload.ad_platform === "tiktok") {
    payload.meta_connection_id = null;
    payload.meta_pixel_id = null;
    payload.attribution_mode = "dynamic";
  } else {
    payload.tiktok_pixel_id = null;
  }
  return payload;
}
export function fillTrend(rows, { start, end, tz }) {
  const found = new Map(rows.map((row) => [row.date, row]));
  let labels = [];
  if (start === end) {
    const format = new Intl.DateTimeFormat("en-CA", {
      timeZone: tz,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      hourCycle: "h23",
    });
    const anchor = Date.parse(start + "T00:00:00Z");
    for (let h = -15; h <= 39; h++) {
      const parts = Object.fromEntries(
        format
          .formatToParts(new Date(anchor + h * 3600000))
          .map((p) => [p.type, p.value]),
      );
      const date = parts.year + "-" + parts.month + "-" + parts.day;
      if (date === start) labels.push(date + " " + parts.hour + ":00");
    }
    labels = [...new Set(labels)].sort();
  } else {
    const cursor = new Date(start + "T00:00:00Z");
    for (let i = 0; i < 366 && cursor.toISOString().slice(0, 10) <= end; i++) {
      labels.push(cursor.toISOString().slice(0, 10));
      cursor.setUTCDate(cursor.getUTCDate() + 1);
    }
  }
  return labels.map(
    (date) => found.get(date) || { date, total: 0, filtered: 0, unique: 0 },
  );
}
