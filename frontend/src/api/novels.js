import { request } from "./http.js";

export function normalizeNovelSlug(value = "") {
  return value.toLowerCase().trim().replace(/[^\p{L}\p{N}]+/gu, "-").replace(/^-|-$/g, "");
}

// 只提交后端允许的字段，避免把表格和弹窗的临时状态写入内容数据。
export function novelPayload(source) {
  return {
    title: source.title || "", slug: source.slug || "", author: source.author || "",
    category: source.category || "", excerpt: source.excerpt || "", cover_path: source.cover_path || "",
    published_at: source.published_at || "", enabled: Boolean(source.enabled),
    featured: Boolean(source.featured), sort_order: Number(source.sort_order) || 0,
  };
}
export function chapterPayload(source) {
  return { chapter_number: Number(source.chapter_number) || 0, title: source.title || "", body_markdown: source.body_markdown || "", enabled: Boolean(source.enabled) };
}
export function listNovels(filters={}) { const query=new URLSearchParams(); for(const [key,value] of Object.entries(filters)) if(value!==""&&value!=null) query.set(key,value); return request(`/novels?${query}`); }
export const getNovel=(id)=>request(`/novels/${id}`);
export const createNovel=(data)=>request("/novels",{method:"POST",body:JSON.stringify(novelPayload(data))});
export const updateNovel=(id,data)=>request(`/novels/${id}`,{method:"PATCH",body:JSON.stringify(novelPayload(data))});
export const deleteNovel=(id)=>request(`/novels/${id}`,{method:"DELETE"});
export const setNovelEnabled=(id,enabled)=>request(`/novels/${id}/status`,{method:"PATCH",body:JSON.stringify({enabled})});
export const setNovelFeatured=(id,featured)=>request(`/novels/${id}/featured`,{method:"PATCH",body:JSON.stringify({featured})});
export const listChapters=(id)=>request(`/novels/${id}/chapters`);
export const createChapter=(id,data)=>request(`/novels/${id}/chapters`,{method:"POST",body:JSON.stringify(chapterPayload(data))});
export const updateChapter=(id,chapterId,data)=>request(`/novels/${id}/chapters/${chapterId}`,{method:"PATCH",body:JSON.stringify(chapterPayload(data))});
export const deleteChapter=(id,chapterId)=>request(`/novels/${id}/chapters/${chapterId}`,{method:"DELETE"});
export const previewNovelMarkdown=(body_markdown)=>request("/novels/preview",{method:"POST",body:JSON.stringify({body_markdown})});
export function uploadNovelCover(file){const body=new FormData();body.append("file",file);return request("/novel-covers",{method:"POST",body});}
