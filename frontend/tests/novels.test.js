import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  listNovels,
  createNovel,
  listChapters,
  createChapter,
  updateChapter,
  novelPayload,
  chapterPayload,
  NOVEL_TRANSLATION_LOCALES,
  translationLocalesPayload,
  listNovelTranslations,
  generateNovelTranslations,
  setNovelTranslationEnabled,
  canGenerateNovelTranslation,
  uploadAndPersistNovelCover,
} from "../src/api/novels.js";
import * as novelsAPI from "../src/api/novels.js";
import viteConfig from "../vite.config.js";

test("novel clients whitelist payloads and use chapter endpoints", async (t) => {
  const calls = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    return new Response(JSON.stringify({ items: [] }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });
  assert.deepEqual(
    novelPayload({
      title: "Story",
      author: "Nine",
      sort_order: 3,
      ignored: true,
    }).ignored,
    undefined,
  );
  assert.equal(
    novelPayload({ cover_path: "/novel-uploads/cover.png" }).cover_path,
    "/novel-uploads/cover.png",
  );
  assert.deepEqual(
    chapterPayload({
      chapter_number: 2,
      title: "Two",
      body_markdown: "Body",
      enabled: true,
      ignored: true,
    }).ignored,
    undefined,
  );
  await listNovels({ q: "glass" });
  await createNovel({ title: "Story" });
  await listChapters(7);
  await createChapter(7, { chapter_number: 1 });
  await updateChapter(7, 3, { chapter_number: 2 });
  assert.deepEqual(
    calls.map(({ url }) => url),
    [
      "/api/v1/novels?q=glass",
      "/api/v1/novels",
      "/api/v1/novels/7/chapters",
      "/api/v1/novels/7/chapters",
      "/api/v1/novels/7/chapters/3",
    ],
  );
});

test("admin development server proxies persisted novel covers", () => {
  assert.equal(
    viteConfig.server.proxy["/novel-uploads"],
    "http://127.0.0.1:8080",
  );
});

test("admin registers the image component used to render novel covers", () => {
  // 本项目按需注册 Element Plus；未注册时 el-image 只会变成空的自定义标签。
  const source = readFileSync(
    new URL("../src/main.js", import.meta.url),
    "utf8",
  );
  assert.match(
    source,
    /import \{ ElImage \} from "element-plus\/es\/components\/image\/index"/,
  );
  assert.match(source, /ElImage,\s*\n\s*ElTable/);
});

test("existing novel cover upload is persisted before the new path is returned", async (t) => {
  const calls = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    const body =
      url === "/api/v1/novel-covers"
        ? { path: "/novel-uploads/0123456789abcdef0123456789abcdef.webp" }
        : {
            id: 7,
            cover_path: "/novel-uploads/0123456789abcdef0123456789abcdef.webp",
          };
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  // 返回路径前必须完成专用 PATCH，避免列表刷新时仍读取旧封面。
  const path = await uploadAndPersistNovelCover(
    new Blob(["cover"], { type: "image/webp" }),
    7,
  );
  assert.equal(path, "/novel-uploads/0123456789abcdef0123456789abcdef.webp");
  assert.deepEqual(
    calls.map(({ url }) => url),
    ["/api/v1/novel-covers", "/api/v1/novels/7/cover"],
  );
  assert.equal(calls[1].options.method, "PATCH");
  assert.equal(calls[1].options.body, JSON.stringify({ cover_path: path }));
});

test("novel translation controls expose eight targets and dedicated actions", async (t) => {
  assert.deepEqual(
    NOVEL_TRANSLATION_LOCALES.map(({ code }) => code),
    ["id", "ja", "ko", "ms", "pt", "fil", "th", "vi"],
  );
  assert.deepEqual(novelsAPI.NOVEL_TRANSLATION_LOCALE_CODES, [
    "id",
    "ja",
    "ko",
    "ms",
    "pt",
    "fil",
    "th",
    "vi",
  ]);
  assert.deepEqual(translationLocalesPayload(["ja", "ko", "ja", "en"]), {
    locales: ["ja", "ko"],
  });
  const calls = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    return new Response(JSON.stringify({ items: [] }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });
  await listNovelTranslations(6);
  await generateNovelTranslations(6, ["ja", "th"]);
  // 单行按钮复用批量接口，但请求中只能包含当前行对应的一种语言。
  await generateNovelTranslations(6, ["th"]);
  await setNovelTranslationEnabled(6, "ja", false);
  assert.deepEqual(
    calls.map(({ url }) => url),
    [
      "/api/v1/novels/6/translations",
      "/api/v1/novels/6/translations",
      "/api/v1/novels/6/translations",
      "/api/v1/novels/6/translations/ja/status",
    ],
  );
  assert.equal(
    calls[1].options.body,
    JSON.stringify({ locales: ["ja", "th"] }),
  );
  assert.equal(calls[2].options.body, JSON.stringify({ locales: ["th"] }));
  assert.equal(calls[3].options.body, JSON.stringify({ enabled: false }));
});

test("single-language translation is only offered for actionable states", () => {
  // 失败、未生成和原文更新需要允许运营人员单独生成；活动任务和已发布译文不可重复提交。
  assert.equal(canGenerateNovelTranslation("failed"), true);
  assert.equal(canGenerateNovelTranslation("not_generated"), true);
  assert.equal(canGenerateNovelTranslation("stale"), true);
  assert.equal(canGenerateNovelTranslation("queued"), false);
  assert.equal(canGenerateNovelTranslation("running"), false);
  assert.equal(canGenerateNovelTranslation("published"), false);
});

test("novel edit page keeps translation selection separate from saving English", () => {
  const page = readFileSync(
    new URL("../src/views/NovelFormView.vue", import.meta.url),
    "utf8",
  );
  assert.match(page, /多语言翻译/);
  assert.match(page, /生成翻译/);
  assert.match(page, /translationSelection/);
  assert.match(page, /translation-status/);
  assert.match(page, /await loadTranslations\(\)/);
  assert.match(page, /loadTranslations\(true\)/);
});
