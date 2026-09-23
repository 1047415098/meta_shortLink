<template>
  <section>
    <PageHeader
      :title="editing ? '编辑小说' : '新增小说'"
      context="免费小说内容"
      description="书籍资料与章节分开维护，章节 Markdown 由后端安全渲染。"
    >
      <el-button @click="router.push({ name: 'novels' })">返回列表</el-button>
      <el-button type="primary" :loading="saving" @click="saveNovel"
        >保存书籍</el-button
      >
    </PageHeader>
    <section class="panel" v-loading="loading">
      <el-form label-position="top">
        <div class="two">
          <el-form-item label="标题" required
            ><el-input v-model="form.title" maxlength="160" /></el-form-item
          ><el-form-item label="slug" required
            ><el-input
              v-model="form.slug"
              @blur="form.slug = normalizeNovelSlug(form.slug)"
          /></el-form-item>
        </div>
        <div class="three">
          <el-form-item label="作者"
            ><el-input
              v-model="form.author"
              maxlength="100"
              placeholder="可选" /></el-form-item
          ><el-form-item label="分类"
            ><el-input
              v-model="form.category"
              maxlength="80"
              placeholder="可选" /></el-form-item
          ><el-form-item label="发布日期" required
            ><el-date-picker
              v-model="form.published_at"
              type="date"
              value-format="YYYY-MM-DD"
              style="width: 100%"
          /></el-form-item>
        </div>
        <el-form-item label="摘要" required
          ><el-input
            v-model="form.excerpt"
            type="textarea"
            :rows="3"
            maxlength="500"
            show-word-limit
        /></el-form-item>
        <div class="two">
          <el-form-item label="封面"
            ><div class="cover-row">
              <el-image
                v-if="form.cover_path"
                :src="form.cover_path"
                :preview-src-list="[form.cover_path]"
                preview-teleported
                fit="cover"
                class="cover-preview"
                ><template #error
                  ><div class="cover-broken">封面加载失败</div></template
                ></el-image
              >
              <div v-else class="cover-placeholder">暂无封面</div>
              <div class="cover-actions">
                <el-upload
                  :show-file-list="false"
                  :http-request="upload"
                  accept="image/jpeg,image/png,image/webp"
                  ><el-button :loading="uploading">{{
                    form.cover_path ? "更换封面" : "上传封面"
                  }}</el-button></el-upload
                ><el-button
                  v-if="form.cover_path"
                  link
                  type="danger"
                  @click="form.cover_path = ''"
                  >移除</el-button
                ><small v-if="form.cover_path" class="muted"
                  >点击封面可放大预览</small
                >
              </div>
            </div>
            <div class="muted">JPEG、PNG 或 WebP，最大 5MB</div></el-form-item
          ><el-form-item label="首页排序"
            ><el-input-number v-model="form.sort_order" :min="0" :max="9999"
          /></el-form-item>
        </div>
        <el-form-item
          ><el-switch v-model="form.enabled" active-text="启用" /><el-switch
            v-model="form.featured"
            class="feature-switch"
            active-text="首页推荐"
            :disabled="!form.enabled"
        /></el-form-item>
      </el-form>
    </section>

    <section v-if="editing" class="panel translations">
      <div class="section-heading">
        <div>
          <h2>多语言翻译</h2>
          <p class="muted">
            英文为唯一原文。选择本次需要生成的语言，译本全部完成后才会发布。
          </p>
        </div>
        <el-button
          type="primary"
          :loading="generatingTranslations"
          :disabled="!translationSelection.length"
          @click="generateSelectedTranslations"
          >生成翻译</el-button
        >
      </div>
      <el-checkbox-group
        v-model="translationSelection"
        class="translation-options"
      >
        <el-checkbox
          v-for="option in NOVEL_TRANSLATION_LOCALES"
          :key="option.code"
          :value="option.code"
          >{{ option.name }}</el-checkbox
        >
      </el-checkbox-group>
      <el-table
        class="translation-status"
        :data="translationStatuses"
        v-loading="translationsLoading"
        empty-text="暂无翻译状态"
      >
        <el-table-column prop="name" label="语言" min-width="130" />
        <el-table-column label="状态" min-width="120"
          ><template #default="{ row }"
            ><el-tag :type="translationTagType(row.status)">{{
              translationStatusLabel(row.status)
            }}</el-tag></template
          ></el-table-column
        >
        <el-table-column label="进度" min-width="180"
          ><template #default="{ row }"
            ><el-progress
              v-if="row.total_items"
              :percentage="translationProgress(row)"
              :stroke-width="8"
            /><span v-else class="muted">—</span></template
          ></el-table-column
        >
        <el-table-column label="说明" min-width="220"
          ><template #default="{ row }"
            ><span v-if="row.error_message" class="translation-error">{{
              row.error_message
            }}</span
            ><span v-else-if="row.status === 'stale'" class="muted"
              >英文原文已更新，旧译文仍在线</span
            ><span v-else class="muted">{{
              row.published ? "已有完整译本" : "尚未生成"
            }}</span></template
          ></el-table-column
        >
        <el-table-column label="前台展示" width="110"
          ><template #default="{ row }"
            ><el-switch
              v-if="row.published"
              v-model="row.enabled"
              @change="toggleTranslation(row)"
            /><span v-else class="muted">—</span></template
          ></el-table-column
        >
        <!-- 单语言操作与顶部批量生成并列，只重试当前行对应的目标语言。 -->
        <el-table-column label="操作" width="120" fixed="right"
          ><template #default="{ row }"
            ><el-button
              v-if="canGenerateNovelTranslation(row.status)"
              link
              type="primary"
              :loading="retryingLocale === row.locale"
              :disabled="generatingTranslations || Boolean(retryingLocale)"
              @click="generateSingleTranslation(row)"
              >{{
                row.status === "failed" ? "重新翻译" : "生成翻译"
              }}</el-button
            ><el-button
              v-else-if="row.status === 'queued' || row.status === 'running'"
              link
              disabled
              >进行中</el-button
            ><span v-else class="muted">—</span></template
          ></el-table-column
        >
      </el-table>
    </section>

    <section v-if="editing" class="panel chapters">
      <div class="section-heading">
        <div>
          <h2>章节管理</h2>
          <p class="muted">免费站全部章节可读，不包含登录、付费和章节锁。</p>
        </div>
        <el-button type="primary" :icon="Plus" @click="openChapter()"
          >新增章节</el-button
        >
      </div>
      <el-table
        :data="chapters"
        v-loading="chaptersLoading"
        empty-text="暂无章节"
      >
        <el-table-column
          prop="chapter_number"
          label="章节"
          width="90"
        /><el-table-column
          prop="title"
          label="标题"
          min-width="240"
        /><el-table-column label="状态" width="90"
          ><template #default="{ row }"
            ><el-tag :type="row.enabled ? 'success' : 'info'">{{
              row.enabled ? "启用" : "停用"
            }}</el-tag></template
          ></el-table-column
        >
        <el-table-column label="操作" width="150"
          ><template #default="{ row }"
            ><el-button link type="primary" @click="openChapter(row)"
              >编辑</el-button
            ><el-button link type="danger" @click="removeChapter(row)"
              >删除</el-button
            ></template
          ></el-table-column
        >
      </el-table>
    </section>

    <el-dialog
      v-model="chapterDialog"
      :title="chapterForm.id ? '编辑章节' : '新增章节'"
      width="min(1040px, 94vw)"
      destroy-on-close
    >
      <div class="chapter-editor">
        <el-form label-position="top"
          ><div class="two">
            <el-form-item label="章节序号" required
              ><el-input-number
                v-model="chapterForm.chapter_number"
                :min="1" /></el-form-item
            ><el-form-item label="章节标题" required
              ><el-input v-model="chapterForm.title" maxlength="160"
            /></el-form-item>
          </div>
          <el-form-item label="Markdown 正文" required
            ><el-input
              v-model="chapterForm.body_markdown"
              type="textarea"
              :rows="18"
              @input="schedulePreview" /></el-form-item
          ><el-switch v-model="chapterForm.enabled" active-text="启用"
        /></el-form>
        <aside class="chapter-preview">
          <h3>正文预览</h3>
          <div v-if="previewing" class="muted">正在生成预览…</div>
          <article v-else-if="bodyHTML" v-html="bodyHTML"></article>
          <el-empty v-else description="输入正文后显示预览" />
        </aside>
      </div>
      <template #footer
        ><el-button @click="chapterDialog = false">取消</el-button
        ><el-button type="primary" :loading="chapterSaving" @click="saveChapter"
          >保存章节</el-button
        ></template
      >
    </el-dialog>
  </section>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Plus } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import PageHeader from "../components/PageHeader.vue";
import {
  canGenerateNovelTranslation,
  createChapter,
  createNovel,
  deleteChapter,
  generateNovelTranslations,
  getNovel,
  listChapters,
  listNovelTranslations,
  normalizeNovelSlug,
  NOVEL_TRANSLATION_LOCALES,
  NOVEL_TRANSLATION_LOCALE_CODES,
  previewNovelMarkdown,
  setNovelTranslationEnabled,
  updateChapter,
  updateNovel,
  uploadAndPersistNovelCover,
} from "../api/novels.js";
const route = useRoute(),
  router = useRouter(),
  editing = Boolean(route.params.id);
const loading = ref(editing),
  saving = ref(false),
  uploading = ref(false),
  chaptersLoading = ref(false),
  chapterDialog = ref(false),
  chapterSaving = ref(false),
  previewing = ref(false),
  bodyHTML = ref("");
// 默认选中全部目标语言，运营人员点击一次即可批量创建八种译文。
const translationsLoading = ref(false),
  generatingTranslations = ref(false),
  retryingLocale = ref(""),
  translationSelection = ref([...NOVEL_TRANSLATION_LOCALE_CODES]),
  translationStatuses = ref([]);
const form = reactive({
  title: "",
  slug: "",
  author: "",
  category: "",
  excerpt: "",
  cover_path: "",
  published_at: new Date().toISOString().slice(0, 10),
  enabled: true,
  featured: false,
  sort_order: 0,
});
const chapters = ref([]),
  chapterForm = reactive({
    id: 0,
    chapter_number: 1,
    title: "",
    body_markdown: "",
    enabled: true,
  });
let timer, translationPoll;
async function upload({ file }) {
  uploading.value = true;
  try {
    if (file.size > 5 * 1024 * 1024) throw new Error("封面不能超过 5MB");
    // 编辑已有小说时立即保存封面，新建小说仍随首次保存一起写入。
    form.cover_path = await uploadAndPersistNovelCover(
      file,
      editing ? route.params.id : 0,
    );
    ElMessage.success(editing ? "封面上传并保存成功" : "封面上传成功");
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    uploading.value = false;
  }
}
async function saveNovel() {
  if (!form.title || !form.slug || !form.excerpt || !form.published_at) {
    ElMessage.warning("请填写所有必填项");
    return;
  }
  form.slug = normalizeNovelSlug(form.slug);
  saving.value = true;
  try {
    if (editing) {
      await updateNovel(route.params.id, form);
      await loadTranslations();
    } else {
      const item = await createNovel(form);
      ElMessage.success("小说已创建，请继续添加章节");
      router.replace({ name: "novel-edit", params: { id: item.id } });
      return;
    }
    ElMessage.success("小说已保存");
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    saving.value = false;
  }
}
async function loadChapters() {
  chaptersLoading.value = true;
  try {
    chapters.value = (await listChapters(route.params.id)).items || [];
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    chaptersLoading.value = false;
  }
}
function openChapter(row) {
  Object.assign(
    chapterForm,
    row
      ? { ...row }
      : {
          id: 0,
          chapter_number: (chapters.value.at(-1)?.chapter_number || 0) + 1,
          title: "",
          body_markdown: "",
          enabled: true,
        },
  );
  bodyHTML.value = "";
  chapterDialog.value = true;
  if (row?.body_markdown) renderPreview();
}
function schedulePreview() {
  clearTimeout(timer);
  timer = setTimeout(renderPreview, 300);
}
async function renderPreview() {
  previewing.value = true;
  try {
    bodyHTML.value =
      (await previewNovelMarkdown(chapterForm.body_markdown)).body_html || "";
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    previewing.value = false;
  }
}
async function saveChapter() {
  if (
    !chapterForm.chapter_number ||
    !chapterForm.title ||
    !chapterForm.body_markdown
  ) {
    ElMessage.warning("请填写章节序号、标题和正文");
    return;
  }
  chapterSaving.value = true;
  try {
    if (chapterForm.id)
      await updateChapter(route.params.id, chapterForm.id, chapterForm);
    else await createChapter(route.params.id, chapterForm);
    chapterDialog.value = false;
    ElMessage.success("章节已保存");
    await Promise.all([loadChapters(), loadTranslations()]);
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    chapterSaving.value = false;
  }
}
async function removeChapter(row) {
  try {
    await ElMessageBox.confirm(
      `确认删除第 ${row.chapter_number} 章？`,
      "删除章节",
      { type: "warning" },
    );
    await deleteChapter(route.params.id, row.id);
    ElMessage.success("章节已删除");
    await Promise.all([loadChapters(), loadTranslations()]);
  } catch (error) {
    if (error !== "cancel" && error !== "close")
      ElMessage.error(error.message || "删除失败");
  }
}
const translationStatusLabel = (status) =>
  ({
    not_generated: "未生成",
    queued: "排队中",
    running: "翻译中",
    published: "已发布",
    stale: "原文已更新",
    failed: "失败",
  })[status] || status;
const translationTagType = (status) =>
  ({
    published: "success",
    stale: "warning",
    failed: "danger",
    queued: "info",
    running: "primary",
  })[status] || "info";
const translationProgress = (row) =>
  row.total_items
    ? Math.min(
        100,
        Math.round(((row.completed_items || 0) * 100) / row.total_items),
      )
    : 0;
function scheduleTranslationRefresh() {
  clearTimeout(translationPoll);
  if (
    translationStatuses.value.some(
      ({ status }) => status === "queued" || status === "running",
    )
  )
    translationPoll = setTimeout(() => loadTranslations(true), 2000);
}
async function loadTranslations(silent = false) {
  if (!silent) translationsLoading.value = true;
  try {
    translationStatuses.value =
      (await listNovelTranslations(route.params.id)).items || [];
  } catch (error) {
    if (!silent) ElMessage.error(error.message);
  } finally {
    if (!silent) translationsLoading.value = false;
    /* 活动任务即使一次轮询失败也继续重试。 */ scheduleTranslationRefresh();
  }
}
async function generateSelectedTranslations() {
  if (!translationSelection.value.length) {
    ElMessage.warning("请至少选择一种目标语言");
    return;
  }
  generatingTranslations.value = true;
  try {
    translationStatuses.value =
      (
        await generateNovelTranslations(
          route.params.id,
          translationSelection.value,
        )
      ).items || [];
    translationSelection.value = [...NOVEL_TRANSLATION_LOCALE_CODES];
    ElMessage.success("翻译任务已创建");
    scheduleTranslationRefresh();
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    generatingTranslations.value = false;
  }
}
// 单语言重试沿用批量接口，但请求体只携带当前行语言，避免牵连其他已成功的译文。
async function generateSingleTranslation(row) {
  if (!canGenerateNovelTranslation(row.status) || retryingLocale.value) return;
  retryingLocale.value = row.locale;
  try {
    translationStatuses.value =
      (await generateNovelTranslations(route.params.id, [row.locale])).items ||
      [];
    ElMessage.success(`${row.name}翻译任务已创建`);
    scheduleTranslationRefresh();
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    retryingLocale.value = "";
  }
}
async function toggleTranslation(row) {
  try {
    await setNovelTranslationEnabled(route.params.id, row.locale, row.enabled);
    ElMessage.success(row.enabled ? "译文已启用" : "译文已停用");
  } catch (error) {
    row.enabled = !row.enabled;
    ElMessage.error(error.message);
  }
}
onMounted(async () => {
  if (!editing) return;
  try {
    Object.assign(form, await getNovel(route.params.id));
    await Promise.all([loadChapters(), loadTranslations()]);
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    loading.value = false;
  }
});
onBeforeUnmount(() => {
  clearTimeout(timer);
  clearTimeout(translationPoll);
});
</script>

<style scoped>
.two {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.three {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 16px;
}
.cover-row {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
}
.cover-preview,
.cover-placeholder {
  width: 96px;
  height: 128px;
  border-radius: 8px;
  flex: 0 0 auto;
  background: #f2f3f5;
}
.cover-placeholder,
.cover-broken {
  display: grid;
  place-items: center;
  text-align: center;
  color: #909399;
  font-size: 12px;
}
.cover-broken {
  height: 100%;
  padding: 8px;
}
.cover-actions {
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  gap: 8px;
}
.feature-switch {
  margin-left: 24px;
}
.translations,
.chapters {
  margin-top: 20px;
}
.translation-options {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 18px;
  margin-bottom: 18px;
}
.translation-error {
  color: #f56c6c;
}
.section-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.section-heading h2,
.section-heading p {
  margin: 0 0 5px;
}
.chapter-editor {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}
.chapter-preview {
  border-left: 1px solid #ebeef5;
  padding-left: 24px;
  max-height: 650px;
  overflow: auto;
}
.chapter-preview h3 {
  margin-top: 0;
}
.chapter-preview article {
  font-family: Georgia, serif;
  font-size: 17px;
  line-height: 1.8;
  color: #30343b;
}
@media (max-width: 900px) {
  .three,
  .chapter-editor {
    grid-template-columns: 1fr;
  }
  .chapter-preview {
    border-left: 0;
    border-top: 1px solid #ebeef5;
    padding: 20px 0 0;
  }
}
@media (max-width: 620px) {
  .two,
  .three {
    grid-template-columns: 1fr;
  }
  .section-heading {
    align-items: flex-start;
    gap: 12px;
  }
  .translation-options {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
}
</style>
