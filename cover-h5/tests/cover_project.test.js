import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { COVER_WALL_STAGES, nextCoverWallStage, nextLuckyUserNumber } from "../src/lib/coverWallFlow.js";
import { coverWallPlaceholderTranslations } from "../src/lib/coverWallPlaceholderTranslations.js";
import { postForm } from "../src/lib/tracking.js";

test("standalone cover flow keeps the complete guide, questions, draw, and conversion stages", async () => {
  assert.equal(nextCoverWallStage(COVER_WALL_STAGES.INTRO), COVER_WALL_STAGES.QUESTIONS);
  assert.equal(nextCoverWallStage(COVER_WALL_STAGES.QUESTIONS), COVER_WALL_STAGES.REVEAL);
  assert.equal(nextCoverWallStage(COVER_WALL_STAGES.REVEAL), COVER_WALL_STAGES.CONVERSION);
  assert.equal(nextCoverWallStage(COVER_WALL_STAGES.CONVERSION), null);

  const [app, component] = await Promise.all([
    readFile(new URL("../src/App.vue", import.meta.url), "utf8"),
    readFile(new URL("../src/components/PhotoWallStartup.vue", import.meta.url), "utf8"),
  ]);
  // 独立封面项目没有小说 Router、小说 API、阅读按钮或搜索跳转。
  assert.match(app, /<PhotoWallStartup v-else \/>/);
  assert.doesNotMatch(app + component, /novel-api|RouterView|enterStory|openSearch|Start Reading/);
  assert.match(component, /COVER_WALL_STAGES\.INTRO/);
  assert.match(component, /COVER_WALL_STAGES\.QUESTIONS/);
  assert.match(component, /COVER_WALL_STAGES\.REVEAL/);
  assert.match(component, /COVER_WALL_STAGES\.CONVERSION/);
  assert.match(component, /const prizeKeys = \["coverWallPrizeOne"[\s\S]*"coverWallPrizeSix"\]/);
  assert.match(component, /<dialog[^>]*aria-labelledby="cover-wall-payment-title"/);
});

test("cover wall uses every local photo in alternating continuous rows", async () => {
  const [component, styles] = await Promise.all([
    readFile(new URL("../src/components/PhotoWallStartup.vue", import.meta.url), "utf8"),
    readFile(new URL("../src/styles.css", import.meta.url), "utf8"),
  ]);
  assert.match(component, /import\.meta\.glob\("\.\.\/assets\/photo\/\*\.\{jpg,jpeg,png,webp\}"/);
  assert.match(component, /const initialPhoto = photoUrls\[Math\.floor\(Math\.random\(\) \* photoUrls\.length\)\]/);
  assert.match(component, /v-for="\(row, rowIndex\) in wallRows"/);
  assert.match(component, /v-for="copy in 2"/);
  assert.match(styles, /animation:\s*photo-wall-scroll 96s linear infinite/);
  assert.match(styles, /animation-direction:\s*reverse/);
  assert.doesNotMatch(styles.match(/\.novel-photo-wall__fallback-cover\s*\{([^}]*)\}/)?.[1] || "", /filter:\s*blur/);
});

test("lucky display number stays between 280 and 359 and changes across refreshes", () => {
  assert.equal(nextLuckyUserNumber(null, () => 0), 280);
  assert.equal(nextLuckyUserNumber(null, () => 0.999999), 359);
  assert.equal(nextLuckyUserNumber(280, () => 0), 281);
  assert.equal(nextLuckyUserNumber(359, () => 0.999999), 358);
});

test("cover callback helper preserves the server status for threshold retry decisions", async () => {
  const result = await postForm("/cover/demo/time-spent", { ticket:"signed" }, {
    request:async () => ({ ok:false, status:409, headers:{ get:() => "" } }),
  });
  assert.deepEqual(result, { ok:false, status:409, data:null });
});

test("cover app confirms server thresholds before browser conversion events", async () => {
  const app = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");
  const serverCall = app.indexOf("/time-spent");
  const metaCall = app.indexOf("trackMetaTimeSpent", serverCall);
  assert.ok(serverCall >= 0 && metaCall > serverCall);
  assert.match(app, /result\.status === 409/);
  assert.match(app, /thresholdAttempts < 3/);
});

test("all ten cover-project languages include the guide, draw, and payment copy", () => {
  const locales = ["en", "id", "ja", "ko", "ms", "pt", "es", "fil", "th", "vi"];
  const englishKeys = Object.keys(coverWallPlaceholderTranslations.en).sort();
  for (const locale of locales) {
    const copy = coverWallPlaceholderTranslations[locale];
    assert.deepEqual(Object.keys(copy).sort(), englishKeys, locale);
    for (const key of ["coverWallIntroTitle", "coverWallQuestionFour", "coverWallPrizeOne", "coverWallPrizeSix", "coverWallPaymentTitle", "coverWallPaymentCopy"]) {
      assert.ok(copy[key]?.trim(), `${locale}:${key}`);
    }
    assert.match(copy.coverWallPaymentTitle, /0\.99 USDT/, locale);
  }
});
