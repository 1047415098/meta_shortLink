export const homePath = (code) => `/novel/${encodeURIComponent(code)}`;
export const searchPath = (code) => `${homePath(code)}/search`;
export const storiesPath = (code) => `${homePath(code)}/stories`;
export const storyPath = (code, slug) => `${storiesPath(code)}/${encodeURIComponent(slug)}`;
export const chapterPath = (code, slug, chapterNumber) => `${storyPath(code, slug)}?chapter=${Number(chapterNumber) || 1}`;
