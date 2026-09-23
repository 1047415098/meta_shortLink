# 小说 H5 Vue I18n 与语言切换 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让运营人员默认一次生成全部八种小说译文，并让 H5 使用 Vue I18n 在已发布语言之间同步切换界面与小说内容，新用户按 IP 国家获得默认语言。

**Architecture:** 保留后端现有预生成译文、原子发布、`available_locales` 和 GeoIP 解析机制。运营后台只调整默认勾选行为；H5 用 `vue-i18n` 替换自定义文案格式化器，同时保留一个轻量 locale controller 负责可用语言过滤、Cookie/localStorage 持久化与 Router 查询参数同步。

**Tech Stack:** Vue 3.5、Vue Router 4、Vue I18n 11 Composition API、Vite 6、Node test runner、Go 1.27、Gin、PostgreSQL 17、Docker Compose

**Spec:** `docs/superpowers/specs/2026-09-22-novel-vue-i18n-language-switching-design.md`

## Global Constraints

- 英文仍是唯一原文；H5 不调用翻译 API，也不实时翻译小说。
- 目标语言固定为 `id`、`ja`、`ko`、`ms`、`pt`、`fil`、`th`、`vi`。
- H5 菜单只展示 `en` 和当前入口小说已经完整发布并启用的译文，不展示禁用项。
- 语言优先级固定为 URL `lang`、已保存选择、IP 国家、英文。
- 用户手动选择写入 `novel_lang` Cookie 和 localStorage，保存一年。
- Vue I18n 必须使用 `legacy: false` 和 `fallbackLocale: "en"`。
- 每次代码修改保留简洁中文注释，不做无关重构或过度封装。
- 按项目要求不创建 Git commit，也不推送 GitHub；每个任务以测试结果和 diff 检查作为检查点。

## Review Focus

- Cookie 或 URL 指向未发布语言时必须回退到可用语言，不能出现空白页；Task 2 的 locale controller 测试覆盖。
- 只完成部分翻译任务时，H5 只能显示成功发布的语言，不能显示失败或排队语言；Task 2 的可用语言测试覆盖。
- 用户快速连续切换语言时，旧请求不能覆盖最后选择；Task 2 运行现有 request gate 回归测试覆盖。
- API 返回错误时，错误文案必须使用当前 Vue I18n 语言并能回退英文；Task 2 的翻译器测试覆盖。
- 320–390px 阅读页同时存在返回、语言、搜索和目录按钮时不能重叠；Task 2 的移动端浏览器验收覆盖。

---

### Task 1: 运营后台默认选择全部八种语言

**Files:**
- Modify: `frontend/src/api/novels.js`
- Modify: `frontend/src/views/NovelFormView.vue`
- Test: `frontend/tests/novels.test.js`

**Interfaces:**
- Consumes: `NOVEL_TRANSLATION_LOCALES: Array<{code:string,name:string}>`
- Produces: `NOVEL_TRANSLATION_LOCALE_CODES: string[]`，供编辑页初始化和任务完成后重置选择。

- [ ] **Step 1: 写失败测试，固定默认语言集合**

在 `frontend/tests/novels.test.js` 的 API 导入中加入 `NOVEL_TRANSLATION_LOCALE_CODES`，并在翻译控制测试中加入：

```js
assert.deepEqual(NOVEL_TRANSLATION_LOCALE_CODES, ["id", "ja", "ko", "ms", "pt", "fil", "th", "vi"]);
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd frontend && npm test -- --test-name-pattern="novel translation controls"`

Expected: FAIL，提示 `NOVEL_TRANSLATION_LOCALE_CODES` 尚未导出或值为 `undefined`。

- [ ] **Step 3: 添加共享语言代码并设置默认选择**

在 `frontend/src/api/novels.js` 中紧接语言定义添加：

```js
// 编辑页与 API 共用同一份语言代码，避免默认选择和提交白名单不一致。
export const NOVEL_TRANSLATION_LOCALE_CODES = NOVEL_TRANSLATION_LOCALES.map(({ code }) => code);
const translationLocaleCodes = new Set(NOVEL_TRANSLATION_LOCALE_CODES);
```

在 `NovelFormView.vue` 中导入该常量，并修改状态初始化：

```js
const translationSelection = ref([...NOVEL_TRANSLATION_LOCALE_CODES]);
```

任务创建成功后恢复全部默认选择：

```js
translationSelection.value = [...NOVEL_TRANSLATION_LOCALE_CODES];
```

- [ ] **Step 4: 运行运营后台测试和构建**

Run: `cd frontend && npm test && npm run build`

Expected: 全部测试通过，生产构建成功。

- [ ] **Step 5: 检查点（不提交 Git）**

Run: `git diff --check -- frontend/src/api/novels.js frontend/src/views/NovelFormView.vue frontend/tests/novels.test.js`

Expected: 无输出。

---

### Task 2: 安装 Vue I18n 并建立全局语言控制器

**Files:**
- Modify: `novel-h5/package.json`
- Modify: `novel-h5/package-lock.json`
- Modify: `novel-h5/src/lib/i18n.js`
- Modify: `novel-h5/src/main.js`
- Modify: `novel-h5/src/components/AppHeader.vue`
- Modify: `novel-h5/src/components/BottomNav.vue`
- Modify: `novel-h5/src/components/ChapterDrawer.vue`
- Modify: `novel-h5/src/components/StoryCard.vue`
- Modify: `novel-h5/src/styles.css`
- Modify: `novel-h5/src/views/HomeView.vue`
- Modify: `novel-h5/src/views/SearchView.vue`
- Modify: `novel-h5/src/views/StoryListView.vue`
- Modify: `novel-h5/src/views/StoryView.vue`
- Modify: `novel-h5/src/views/UnavailableView.vue`
- Test: `novel-h5/tests/i18n.test.js`
- Test: `novel-h5/tests/api.test.js`
- Test: `novel-h5/tests/tracking.test.js`

**Interfaces:**
- Consumes: `bootstrap.locale`、`bootstrap.available_locales`、Vue Router 当前查询参数。
- Produces: `createNovelI18n(options) -> { plugin, locale, availableLocales, setLocale, t }`。
- Produces: `availableLocales: ComputedRef<Array<{code:string,name:string}>>`，只包含真正可阅读的语言。
- Produces: `translate(locale,key,parameters)`，内部委托 Vue I18n，供 API 错误处理使用。
- Produces: Vue 插件安装与 `localeController` 注入；所有组件使用 `useI18n({ useScope:"global" })`。

- [ ] **Step 1: 修改测试，描述 Vue I18n 和可用语言的新行为**

在 `novel-h5/tests/i18n.test.js` 中把可用语言断言改为只返回启动数据中的语言：

```js
assert.deepEqual(
  i18n.availableLocales.value.map(({ code }) => code),
  ["en", "ja", "th"],
);
assert.ok(i18n.plugin);
assert.equal(i18n.plugin.global.t("chapterCount", { count:3 }, { locale:"ja" }), "3章");
```

补充无效语言不会改变当前值、不会改 URL 的断言：

```js
await i18n.setLocale("ko");
assert.equal(i18n.locale.value, "ja");
assert.deepEqual(calls, [{ query:{ chapter:"2", lang:"ja" } }]);
```

同时引入 `createSSRApp`、`h`、`renderToString` 和 `useI18n`，加入真实全局 Composer 测试：

```js
test("Vue I18n global composer updates component text", async () => {
  const router = { currentRoute:{ value:{ query:{} } }, replace:async()=>{} };
  const manager = createNovelI18n({
    bootstrap:{ locale:"ja", available_locales:["en","ja"] },
    router,
    storage:{ getItem:()=>null, setItem:()=>{} },
    documentRef:{ cookie:"", documentElement:{ lang:"en" }, title:"" },
    secure:false,
  });
  const View = { setup(){ const { t } = useI18n({ useScope:"global" }); return () => h("p", t("home")); } };
  const app = createSSRApp(View).use(manager.plugin);
  assert.equal(await renderToString(app), "<p>ホーム</p>");
});
```

- [ ] **Step 2: 运行测试并确认失败原因正确**

Run: `cd novel-h5 && npm test -- --test-name-pattern="language|dictionary"`

Expected: FAIL，因为旧实现没有 `plugin`，无法安装全局 Composer，且仍返回九个含禁用标记的菜单项。

- [ ] **Step 3: 安装 Vue I18n 11**

Run: `cd novel-h5 && npm install vue-i18n@^11.0.0`

Expected: `package.json` 和 `package-lock.json` 记录 `vue-i18n`，不增加其他生产依赖。

- [ ] **Step 4: 用 Vue I18n 重写文案引擎，保留业务控制器**

在 `novel-h5/src/lib/i18n.js` 中保留现有九套 `dictionaries` 和 `localeOptions`，引入：

```js
import { computed } from "vue";
import { createI18n } from "vue-i18n";

const plugin = createI18n({
  legacy:false,
  fallbackLocale:"en",
  locale:"en",
  messages:dictionaries,
});
```

`createNovelI18n` 使用以下核心逻辑：

```js
const availableCodes = [...new Set(["en", ...(bootstrap.available_locales || [])])]
  .filter((code) => dictionaries[code]);
const selected = normalizeLocale(routeLocale || remembered || bootstrap.locale, availableCodes);
plugin.global.locale.value = selected;
const locale = plugin.global.locale;
const availableLocales = computed(() => localeOptions.filter(({ code }) => availableCodes.includes(code)));
const t = (...args) => plugin.global.t(...args);

async function setLocale(next) {
  if (!availableCodes.includes(next)) return false;
  locale.value = next;
  storage?.setItem?.("novel-language", next);
  if (documentRef) documentRef.cookie = `${LanguageCookieName}=${encodeURIComponent(next)}; Max-Age=31536000; Path=/; SameSite=Lax${secure ? "; Secure" : ""}`;
  applyDocumentLanguage();
  await router.replace({ query:{ ...router.currentRoute.value.query, lang:next } });
  return true;
}

return { plugin, locale, availableLocales, setLocale, t };
```

`translate` 不再手动替换占位符，改为委托 Vue I18n：

```js
export function translate(locale, key, parameters = {}) {
  return plugin.global.t(key, parameters, { locale:normalizeLocale(locale, localeOptions.map(({ code }) => code)) });
}
```

- [ ] **Step 5: 在应用入口安装插件并单独注入业务控制器**

将 `novel-h5/src/main.js` 的挂载改为：

```js
const localeController = createNovelI18n({ bootstrap, router });
createApp(App)
  .provide("bootstrap", bootstrap)
  .provide("localeController", localeController)
  .use(router)
  .use(localeController.plugin)
  .mount("#app");
```

- [ ] **Step 6: 组件改用全局 Composer**

所有组件和页面移除 `inject("i18n")`，统一引入：

```js
import { useI18n } from "vue-i18n";
const { locale, t } = useI18n({ useScope:"global" });
```

仅 `AppHeader.vue` 继续注入业务能力：

```js
const { locale, t } = useI18n({ useScope:"global" });
const { availableLocales, setLocale } = inject("localeController");
```

保留现有 `watch(locale, ...)`、请求版本门和 `lang` 查询参数传递，不复制新的语言状态。

同一步把 `AppHeader.vue` 的语言项改为只渲染已发布语言：删除 `:disabled="!option.enabled"`、锁图标和 `option.enabled` 分支，保留当前语言勾选：

```vue
<button
  v-for="option in availableLocales"
  :key="option.code"
  class="language-option"
  :class="{ selected:option.code===locale }"
  type="button"
  role="menuitemradio"
  :aria-checked="option.code===locale"
  popovertarget="language-popover"
  popovertargetaction="hide"
  @click="setLocale(option.code)"
>
  <span class="language-code">{{ option.code.toUpperCase() }}</span>
  <span class="language-name">{{ option.name }}</span>
  <i v-if="option.code===locale" class="fa-solid fa-check" aria-hidden="true" />
</button>
```

从 `styles.css` 删除 `.language-option:disabled`，保留当前胶囊入口、浮层、选中高亮和 `max-width:380px` 响应式样式。

- [ ] **Step 7: 运行核心语言测试、完整 H5 测试和生产构建**

Run: `cd novel-h5 && npm test && npm run build`

Expected: 字典、错误文案、可用语言、持久化、Composition API、请求版本门测试全部通过，Vite 构建无注入或模板错误。

- [ ] **Step 8: 内置浏览器移动端验收**

重建容器后检查首页和阅读页：

1. 仅英文译本时菜单只有 English，不显示锁定语言。
2. 有日语和泰语已发布时菜单显示 English、日本語、ไทย。
3. 切换后页面按钮和小说请求同时使用目标语言。
4. 320–390px 下返回、语言、搜索、目录按钮不重叠。
5. 点击外部和按 Esc 均能关闭菜单。

- [ ] **Step 9: 检查点（不提交 Git）**

Run: `git diff --check -- novel-h5/package.json novel-h5/package-lock.json novel-h5/src/lib/i18n.js novel-h5/src/main.js novel-h5/src/components novel-h5/src/views novel-h5/src/styles.css novel-h5/tests`

Expected: 无输出。

---

### Task 3: 验证 IP 默认语言、发布过滤和完整回归

**Files:**
- Verify: `backend/internal/modules/novel/locale.go`
- Test: `backend/internal/modules/novel/locale_test.go`
- Test: `backend/internal/app/novel_locale_test.go`
- Verify: `backend/internal/modules/novel/render.go`
- Verify: `backend/internal/modules/novel/repository.go`

**Interfaces:**
- Consumes: 后端现有 `ResolveLocale(explicit, remembered, country, available)`。
- Produces: 无新接口；确认优先级和 `available_locales` 发布过滤满足设计。

- [ ] **Step 1: 扩展语言解析表格测试**

在 `locale_test.go` 的现有表格中确保包含以下案例：

```go
{"th", "ja", "JP", "th"}, // URL 优先
{"", "ja", "TH", "ja"},  // Cookie 优先于 IP
{"", "", "TH", "th"},    // 新用户按 IP
{"ko", "ko", "KR", "en"}, // 未发布语言回退英文
{"xx", "xx", "JP", "ja"}, // 无效输入后仍可按 IP
```

- [ ] **Step 2: 运行后端 locale 单元测试**

Run: `cd backend && go test ./internal/modules/novel -run 'TestCountryLocale|TestResolveLocale' -count=1`

Expected: PASS；如测试已覆盖且通过，不修改生产代码。

- [ ] **Step 3: 运行小说本地化集成测试**

Run: `cd backend && TEST_DATABASE_URL='postgresql://level@127.0.0.1:5432/whatsapp_analytics_test?sslmode=disable' go test ./internal/app -run 'TestPublishedNovelTranslationLocalizesContentAndFallsBackToEnglish|TestNovelTranslation' -count=1`

Expected: 已发布译文返回目标语言，未发布语言回退英文，启动数据只包含 `en` 与已发布语言。

- [ ] **Step 4: 运行所有前端测试与构建**

Run:

```bash
(cd frontend && npm test && npm run build)
(cd novel-h5 && npm test && npm run build)
(cd landing && npm test && npm run build)
(cd audio-novel && npm test && npm run build)
```

Expected: 四个前端项目全部测试和构建通过。

- [ ] **Step 5: 运行完整后端验证**

Run: `cd backend && TEST_DATABASE_URL='postgresql://level@127.0.0.1:5432/whatsapp_analytics_test?sslmode=disable' go test -race -p=1 ./... -count=1 && go vet ./...`

Expected: 所有包通过，race detector 和 vet 无输出。

- [ ] **Step 6: 重建本地服务并检查健康状态**

Run:

```bash
docker compose -p linkscope up -d --build app
curl -fsS http://127.0.0.1:8080/healthz
```

Expected: 返回 `{"status":"ok"}`。

- [ ] **Step 7: 最终安全与仓库检查**

Run:

```bash
git diff --check
git status --short
```

确认：

- APIHZ 凭证只在本地忽略的 `.env` 中。
- 没有新增提交或推送。
- `vue-i18n` 仅存在于小说 H5 依赖中。
- 内置浏览器停留在可供用户检查的 H5 语言菜单。
