import { request } from "./http.js";

// Keep admin-only presentation fields out of the mutation contract.
export function novelLinkPayload(source = {}) {
  const requestedThreshold = Number(source.time_spent_threshold);
  const requestedStartupTail = Number(source.startup_tail_seconds);
  const platform = source.ad_platform === "tiktok" ? "tiktok" : "meta";
  // 小说投放只暴露停留阈值；隐藏的广告参数始终按所选平台动态读取。
  return {
    name: source.name || "",
    code: source.code || "",
    novel_id: Number(source.novel_id) || 0,
    // 入口章节属于投放配置；历史未绑定链接继续保留 null 兼容简介页。
    entry_chapter_id:
      source.entry_chapter_id === "" || source.entry_chapter_id == null
        ? null
        : Number(source.entry_chapter_id) || null,
    enabled: Boolean(source.enabled),
    ad_platform: platform,
    channel: platform === "tiktok" ? "tiktok" : "facebook",
    campaign_id: "",
    adset_id: "",
    ad_id: "",
    meta_connection_id:
      platform === "meta" ? source.meta_connection_id || null : null,
    meta_pixel_id: platform === "meta" ? source.meta_pixel_id || null : null,
    tiktok_pixel_id:
      platform === "tiktok" ? source.tiktok_pixel_id || null : null,
    attribution_mode: "dynamic",
    time_spent_threshold:
      source.time_spent_threshold === "" ||
      source.time_spent_threshold == null ||
      !Number.isFinite(requestedThreshold)
        ? 10
        : requestedThreshold,
    // 免费小说唯一使用倒计时主题；该固定值用于兼容后端历史字段。
    startup_tail_seconds:
      source.startup_tail_seconds === "" ||
      source.startup_tail_seconds == null ||
      !Number.isFinite(requestedStartupTail)
        ? 5
        : requestedStartupTail,
    startup_theme: "countdown",
  };
}

export function novelEntryChapterOptions(items = [], link = null) {
  // 未访问链接只允许选择当前可读章节；已访问链接额外保留冻结的历史章节。
  const options = items.filter(
    (chapter) => chapter.enabled && !chapter.deleted_at,
  );
  const sortOptions = () =>
    options.sort(
      (left, right) =>
        Number(left.chapter_number || 0) - Number(right.chapter_number || 0),
    );
  if (!link?.first_visited_at || !link.entry_chapter_id) return sortOptions();

  const entryID = Number(link.entry_chapter_id);
  if (options.some((chapter) => Number(chapter.id) === entryID))
    return sortOptions();

  const unavailable = items.find((chapter) => Number(chapter.id) === entryID);
  options.push(
    unavailable || {
      id: entryID,
      chapter_number: link.entry_chapter_number,
      title: link.entry_chapter_title || "原绑定章节",
      enabled: false,
      entry_unavailable: true,
    },
  );
  return sortOptions();
}
export function listNovelLinks(novelId) {
  // The selected novel is an admin query filter, not part of the URL.
  return request("/novel-links/query", {
    method: "POST",
    body: JSON.stringify({ novel_id: novelId || 0 }),
  });
}
export const createNovelLink = (data) =>
  request("/novel-links", {
    method: "POST",
    body: JSON.stringify(novelLinkPayload(data)),
  });
export const updateNovelLink = (id, data) =>
  request(`/novel-links/${id}`, {
    method: "PATCH",
    body: JSON.stringify(novelLinkPayload(data)),
  });
export const deleteNovelLink = (id) =>
  request(`/novel-links/${id}`, { method: "DELETE" });
export const getNovelLinkStats = (id, filters) =>
  request(`/novel-links/${id}/stats`, {
    method: "POST",
    body: JSON.stringify(filters),
  });
