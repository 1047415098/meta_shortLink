// Development keeps the legacy home entry; production may additionally bind an entry chapter.
// Legacy server output has no loading-tail setting, so keep the campaign-safe five-second fallback.
const linkDefaults={entry_story_slug:"",entry_chapter_number:null,time_spent_threshold:0,startup_tail_seconds:5};
const fallback = { link: import.meta.env?.DEV ? { code:"hello",...linkDefaults } : null, ticket:"", surface:"novel", ad_platform:"meta", cookie_enabled:false, meta_measurement:false, tiktok_enabled:false, tiktok_pixel_code:"", locale:"en", available_locales:["en"], error:null };
export function readBootstrap(documentRef = document) {
  const node = documentRef.getElementById("novel-h5-data");
  if (!node?.textContent) return fallback;
  try {
    const result={...fallback,...JSON.parse(node.textContent)};
    // 给历史服务端输出补齐新字段，避免旧链接被误判为绑定章节。
    if(result.link)result.link={...linkDefaults,...result.link};
    return result;
  }
  catch { return { ...fallback, error:{ status:503, message:"The story archive could not be opened." } }; }
}
