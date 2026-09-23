export const homePath = (code) => `/novel/${encodeURIComponent(code)}`;
export const searchPath = (code) => `${homePath(code)}/search`;
export const storiesPath = (code) => `${homePath(code)}/stories`;
export const storyPath = (code, slug) => `${storiesPath(code)}/${encodeURIComponent(slug)}`;
// 章节使用独立路径，简介页和阅读页可以各自维护清晰的页面状态。
export const chapterPath = (code, slug, chapterNumber) => `${storyPath(code, slug)}/chapters/${Number(chapterNumber) || 1}`;
