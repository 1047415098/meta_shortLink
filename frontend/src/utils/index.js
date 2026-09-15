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
export function validateLink(link) {
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
