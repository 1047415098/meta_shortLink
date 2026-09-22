export function homeSections(data = {}) { return { featured:data.featured || null, ranking:Array.isArray(data.ranking) ? data.ranking : [], items:Array.isArray(data.items) ? data.items : [] }; }
export const normalizedQuery = (value) => String(value || "").trim();
export function bannerStories(featured, items = [], limit = 5) {
  const candidates=[featured,...items].filter(Boolean),seen=new Set();
  // 推荐内容固定排在第一张，其余位置按首页列表顺序补足。
  return candidates.filter((story)=>{const key=story.id ?? story.slug;if(seen.has(key))return false;seen.add(key);return true;}).slice(0,Math.max(1,Number(limit)||5));
}
