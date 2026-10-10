// Development keeps the legacy home entry; production may additionally bind an entry chapter.
// Legacy server output has no loading-tail setting, so keep the campaign-safe five-second fallback.
// The established countdown remains the safe default when historical payloads lack a theme.
const linkDefaults={entry_story_slug:"",entry_chapter_number:null,time_spent_threshold:0,startup_tail_seconds:5,startup_theme:"countdown"};
const fallback = { link: import.meta.env?.DEV ? { code:"hello",...linkDefaults } : null, ticket:"", surface:"novel", ad_platform:"meta", cookie_enabled:false, meta_measurement:false, tiktok_enabled:false, tiktok_pixel_code:"", locale:"en", available_locales:["en"], error:null };
export function readBootstrap(documentRef = document) {
  const node = documentRef.getElementById("novel-h5-data");
  if (!node?.textContent) return fallback;
  try {
    const result={...fallback,...JSON.parse(node.textContent)};
    // 历史倒计时链接沿用五秒默认值；封面墙启动数据不携带倒计时尾段参数。
    if(result.link){
      result.link={...linkDefaults,...result.link};
      // 免费小说已恢复为唯一倒计时首屏，忽略历史封面墙主题快照。
      result.link.startup_theme="countdown";
    }
    return result;
  }
  catch { return { ...fallback, error:{ status:503, message:"The story archive could not be opened." } }; }
}
