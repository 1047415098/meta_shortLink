<template>
  <section>
    <PageHeader :title="editing ? '编辑小说' : '新增小说'" context="免费小说内容" description="书籍资料与章节分开维护，章节 Markdown 由后端安全渲染。">
      <el-button @click="router.push({ name: 'novels' })">返回列表</el-button>
      <el-button type="primary" :loading="saving" @click="saveNovel">保存书籍</el-button>
    </PageHeader>
    <section class="panel" v-loading="loading">
      <el-form label-position="top">
        <div class="two"><el-form-item label="标题" required><el-input v-model="form.title" maxlength="160" /></el-form-item><el-form-item label="slug" required><el-input v-model="form.slug" @blur="form.slug = normalizeNovelSlug(form.slug)" /></el-form-item></div>
        <div class="three"><el-form-item label="作者"><el-input v-model="form.author" maxlength="100" placeholder="可选" /></el-form-item><el-form-item label="分类"><el-input v-model="form.category" maxlength="80" placeholder="可选" /></el-form-item><el-form-item label="发布日期" required><el-date-picker v-model="form.published_at" type="date" value-format="YYYY-MM-DD" style="width:100%" /></el-form-item></div>
        <el-form-item label="摘要" required><el-input v-model="form.excerpt" type="textarea" :rows="3" maxlength="500" show-word-limit /></el-form-item>
        <div class="two"><el-form-item label="封面"><div class="cover-row"><el-image v-if="form.cover_path" :src="form.cover_path" :preview-src-list="[form.cover_path]" preview-teleported fit="cover" class="cover-preview"><template #error><div class="cover-broken">封面加载失败</div></template></el-image><div v-else class="cover-placeholder">暂无封面</div><div class="cover-actions"><el-upload :show-file-list="false" :http-request="upload" accept="image/jpeg,image/png,image/webp"><el-button :loading="uploading">{{ form.cover_path ? '更换封面' : '上传封面' }}</el-button></el-upload><el-button v-if="form.cover_path" link type="danger" @click="form.cover_path = ''">移除</el-button><small v-if="form.cover_path" class="muted">点击封面可放大预览</small></div></div><div class="muted">JPEG、PNG 或 WebP，最大 5MB</div></el-form-item><el-form-item label="首页排序"><el-input-number v-model="form.sort_order" :min="0" :max="9999" /></el-form-item></div>
        <el-form-item><el-switch v-model="form.enabled" active-text="启用" /><el-switch v-model="form.featured" class="feature-switch" active-text="首页推荐" :disabled="!form.enabled" /></el-form-item>
      </el-form>
    </section>

    <section v-if="editing" class="panel chapters">
      <div class="section-heading"><div><h2>章节管理</h2><p class="muted">免费站全部章节可读，不包含登录、付费和章节锁。</p></div><el-button type="primary" :icon="Plus" @click="openChapter()">新增章节</el-button></div>
      <el-table :data="chapters" v-loading="chaptersLoading" empty-text="暂无章节">
        <el-table-column prop="chapter_number" label="章节" width="90" /><el-table-column prop="title" label="标题" min-width="240" /><el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
        <el-table-column label="操作" width="150"><template #default="{ row }"><el-button link type="primary" @click="openChapter(row)">编辑</el-button><el-button link type="danger" @click="removeChapter(row)">删除</el-button></template></el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="chapterDialog" :title="chapterForm.id ? '编辑章节' : '新增章节'" width="min(1040px, 94vw)" destroy-on-close>
      <div class="chapter-editor"><el-form label-position="top"><div class="two"><el-form-item label="章节序号" required><el-input-number v-model="chapterForm.chapter_number" :min="1" /></el-form-item><el-form-item label="章节标题" required><el-input v-model="chapterForm.title" maxlength="160" /></el-form-item></div><el-form-item label="Markdown 正文" required><el-input v-model="chapterForm.body_markdown" type="textarea" :rows="18" @input="schedulePreview" /></el-form-item><el-switch v-model="chapterForm.enabled" active-text="启用" /></el-form><aside class="chapter-preview"><h3>正文预览</h3><div v-if="previewing" class="muted">正在生成预览…</div><article v-else-if="bodyHTML" v-html="bodyHTML"></article><el-empty v-else description="输入正文后显示预览" /></aside></div>
      <template #footer><el-button @click="chapterDialog = false">取消</el-button><el-button type="primary" :loading="chapterSaving" @click="saveChapter">保存章节</el-button></template>
    </el-dialog>
  </section>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Plus } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import PageHeader from "../components/PageHeader.vue";
import { createChapter, createNovel, deleteChapter, getNovel, listChapters, normalizeNovelSlug, previewNovelMarkdown, updateChapter, updateNovel, uploadNovelCover } from "../api/novels.js";
const route = useRoute(), router = useRouter(), editing = Boolean(route.params.id);
const loading = ref(editing), saving = ref(false), uploading = ref(false), chaptersLoading = ref(false), chapterDialog = ref(false), chapterSaving = ref(false), previewing = ref(false), bodyHTML = ref("");
const form = reactive({ title:"", slug:"", author:"", category:"", excerpt:"", cover_path:"", published_at:new Date().toISOString().slice(0,10), enabled:true, featured:false, sort_order:0 });
const chapters = ref([]), chapterForm = reactive({ id:0, chapter_number:1, title:"", body_markdown:"", enabled:true });
let timer;
async function upload({ file }) { uploading.value = true; try { if (file.size > 5*1024*1024) throw new Error("封面不能超过 5MB"); form.cover_path = (await uploadNovelCover(file)).path; ElMessage.success("封面上传成功"); } catch (error) { ElMessage.error(error.message); } finally { uploading.value = false; } }
async function saveNovel() { if (!form.title || !form.slug || !form.excerpt || !form.published_at) { ElMessage.warning("请填写所有必填项"); return; } form.slug = normalizeNovelSlug(form.slug); saving.value = true; try { if (editing) await updateNovel(route.params.id, form); else { const item = await createNovel(form); ElMessage.success("小说已创建，请继续添加章节"); router.replace({ name:"novel-edit", params:{ id:item.id } }); return; } ElMessage.success("小说已保存"); } catch (error) { ElMessage.error(error.message); } finally { saving.value = false; } }
async function loadChapters() { chaptersLoading.value = true; try { chapters.value = (await listChapters(route.params.id)).items || []; } catch (error) { ElMessage.error(error.message); } finally { chaptersLoading.value = false; } }
function openChapter(row) { Object.assign(chapterForm, row ? { ...row } : { id:0, chapter_number:(chapters.value.at(-1)?.chapter_number || 0)+1, title:"", body_markdown:"", enabled:true }); bodyHTML.value = ""; chapterDialog.value = true; if (row?.body_markdown) renderPreview(); }
function schedulePreview() { clearTimeout(timer); timer = setTimeout(renderPreview, 300); }
async function renderPreview() { previewing.value = true; try { bodyHTML.value = (await previewNovelMarkdown(chapterForm.body_markdown)).body_html || ""; } catch (error) { ElMessage.error(error.message); } finally { previewing.value = false; } }
async function saveChapter() { if (!chapterForm.chapter_number || !chapterForm.title || !chapterForm.body_markdown) { ElMessage.warning("请填写章节序号、标题和正文"); return; } chapterSaving.value = true; try { if (chapterForm.id) await updateChapter(route.params.id, chapterForm.id, chapterForm); else await createChapter(route.params.id, chapterForm); chapterDialog.value = false; ElMessage.success("章节已保存"); loadChapters(); } catch (error) { ElMessage.error(error.message); } finally { chapterSaving.value = false; } }
async function removeChapter(row) { try { await ElMessageBox.confirm(`确认删除第 ${row.chapter_number} 章？`, "删除章节", { type:"warning" }); await deleteChapter(route.params.id, row.id); ElMessage.success("章节已删除"); loadChapters(); } catch (error) { if (error !== "cancel" && error !== "close") ElMessage.error(error.message || "删除失败"); } }
onMounted(async () => { if (!editing) return; try { Object.assign(form, await getNovel(route.params.id)); await loadChapters(); } catch (error) { ElMessage.error(error.message); } finally { loading.value = false; } });
onBeforeUnmount(() => clearTimeout(timer));
</script>

<style scoped>
.two{display:grid;grid-template-columns:1fr 1fr;gap:16px}.three{display:grid;grid-template-columns:1fr 1fr 1fr;gap:16px}.cover-row{display:flex;align-items:center;gap:14px;width:100%}.cover-preview,.cover-placeholder{width:96px;height:128px;border-radius:8px;flex:0 0 auto;background:#f2f3f5}.cover-placeholder,.cover-broken{display:grid;place-items:center;text-align:center;color:#909399;font-size:12px}.cover-broken{height:100%;padding:8px}.cover-actions{display:flex;align-items:flex-start;flex-direction:column;gap:8px}.feature-switch{margin-left:24px}.chapters{margin-top:20px}.section-heading{display:flex;justify-content:space-between;align-items:center;margin-bottom:16px}.section-heading h2,.section-heading p{margin:0 0 5px}.chapter-editor{display:grid;grid-template-columns:1fr 1fr;gap:24px}.chapter-preview{border-left:1px solid #ebeef5;padding-left:24px;max-height:650px;overflow:auto}.chapter-preview h3{margin-top:0}.chapter-preview article{font-family:Georgia,serif;font-size:17px;line-height:1.8;color:#30343b}@media(max-width:900px){.three,.chapter-editor{grid-template-columns:1fr}.chapter-preview{border-left:0;border-top:1px solid #ebeef5;padding:20px 0 0}}@media(max-width:620px){.two,.three{grid-template-columns:1fr}}
</style>
