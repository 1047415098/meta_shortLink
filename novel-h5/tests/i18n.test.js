import test from "node:test";
import assert from "node:assert/strict";
import { createSSRApp, h } from "vue";
import { renderToString } from "@vue/server-renderer";
import { useI18n } from "vue-i18n";
import { createNovelI18n, dictionaries, localeOptions, normalizeLocale, translate } from "../src/lib/i18n.js";
import { createRequestGate } from "../src/lib/request.js";

test("all nine UI dictionaries expose the same complete keys", () => {
  assert.deepEqual(localeOptions.map(({ code }) => code), ["en", "id", "ja", "ko", "ms", "pt", "fil", "th", "vi"]);
  const englishKeys = Object.keys(dictionaries.en).sort();
  assert.ok(englishKeys.length >= 30);
  for (const { code } of localeOptions) assert.deepEqual(Object.keys(dictionaries[code]).sort(), englishKeys, code);
  assert.equal(translate("ja", "chapterCount", { count:3 }), "3章");
  assert.equal(normalizeLocale("ko", ["en", "ja"]), "en");
});

test("server bootstrap wins over stale local storage so IP detection remains authoritative", () => {
  const router = { currentRoute:{ value:{ query:{} } }, replace:async()=>{} };
  const i18n = createNovelI18n({
    bootstrap:{ locale:"vi", available_locales:["en","ja","vi"] },
    router,
    storage:{ getItem:()=>"ja", setItem:()=>{} },
    documentRef:{ cookie:"", documentElement:{ lang:"en" }, title:"" },
    secure:false,
  });
  assert.equal(i18n.locale.value, "vi");
});

test("manual language selection is remembered without adding a URL language parameter", async () => {
  const values = new Map();
  const storage = { getItem:(key)=>values.get(key)||null, setItem:(key,value)=>values.set(key,value) };
  const documentRef = { cookie:"", documentElement:{ lang:"en" }, title:"" };
  const calls = [];
  const router = { currentRoute:{ value:{ query:{ chapter:"2" } } }, replace:async(value)=>calls.push(value) };
  const i18n = createNovelI18n({ bootstrap:{ locale:"en", available_locales:["en","ja","th"] }, router, storage, documentRef, secure:false });
  await i18n.setLocale("ja");
  assert.equal(i18n.locale.value, "ja");
  assert.equal(values.get("novel-language"), "ja");
  assert.match(documentRef.cookie, /novel_lang=ja/);
  assert.equal(documentRef.documentElement.lang, "ja");
  assert.deepEqual(calls, []);
  await i18n.setLocale("ko");
  assert.equal(i18n.locale.value, "ja");
  assert.deepEqual(calls, []);
});

test("language menu only exposes translations published for the current novel", () => {
  const router = { currentRoute:{ value:{ query:{} } }, replace:async()=>{} };
  const i18n = createNovelI18n({
    bootstrap:{ locale:"en", available_locales:["en","ja","th"] },
    router,
    storage:{ getItem:()=>null, setItem:()=>{} },
    documentRef:{ cookie:"", documentElement:{ lang:"en" }, title:"" },
    secure:false,
  });
  assert.deepEqual(i18n.availableLocales.value.map(({ code }) => code), ["en", "ja", "th"]);
  assert.equal(Object.hasOwn(i18n.availableLocales.value[0], "enabled"), false);
});

test("Vue I18n global composer updates component text", async () => {
  const router = { currentRoute:{ value:{ query:{} } }, replace:async()=>{} };
  const manager = createNovelI18n({
    bootstrap:{ locale:"ja", available_locales:["en", "ja"] },
    router,
    storage:{ getItem:()=>null, setItem:()=>{} },
    documentRef:{ cookie:"", documentElement:{ lang:"en" }, title:"" },
    secure:false,
  });
  assert.ok(manager.plugin);
  const View = {
    setup() {
      const { t } = useI18n({ useScope:"global" });
      return () => h("p", t("home"));
    },
  };
  const app = createSSRApp(View).use(manager.plugin);
  assert.equal(await renderToString(app), "<p>ホーム</p>");
});

test("request gate rejects an older language response after a newer request starts", () => {
  const gate = createRequestGate();
  const english = gate.next();
  const japanese = gate.next();
  assert.equal(gate.isCurrent(english), false);
  assert.equal(gate.isCurrent(japanese), true);
  gate.cancel();
  assert.equal(gate.isCurrent(japanese), false);
});
