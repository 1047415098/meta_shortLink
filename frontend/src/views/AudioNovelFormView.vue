<template>
  <section>
    <PageHeader
      :title="editing ? '编辑语音小说' : '新增语音小说'"
      context="语音小说内容"
      description="正文使用 Markdown，预览内容由后端安全渲染。"
    >
      <el-button @click="router.push({ name: 'audio-novels' })"
        >返回列表</el-button
      ><el-button type="primary" :loading="saving" @click="save"
        >保存</el-button
      >
    </PageHeader>
    <div class="editor-layout" v-loading="loading">
      <section class="panel">
        <el-form label-position="top">
          <div class="two">
            <el-form-item label="标题" required
              ><el-input v-model="form.title" maxlength="160" /></el-form-item
            ><el-form-item label="slug" required
              ><el-input
                v-model="form.slug"
                @blur="form.slug = normalizeAudioNovelSlug(form.slug)"
            /></el-form-item>
          </div>
          <div class="two">
            <el-form-item label="分类" required
              ><el-input v-model="form.category" maxlength="80" /></el-form-item
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
          <el-form-item label="封面"
            ><div class="cover-row">
              <el-image
                v-if="form.cover_path"
                :src="form.cover_path"
                fit="cover"
                class="cover-preview"
              /><el-upload
                :show-file-list="false"
                :http-request="upload"
                accept="image/jpeg,image/png,image/webp"
                ><el-button :loading="uploading">上传封面</el-button></el-upload
              ><el-button
                v-if="form.cover_path"
                link
                type="danger"
                @click="form.cover_path = ''"
                >移除</el-button
              >
            </div>
            <div class="muted">JPEG、PNG 或 WebP，最大 5MB</div></el-form-item
          >
          <el-form-item label="Podcast MP3">
            <div class="audio-row">
              <audio
                v-if="editing && form.audio_path"
                :src="form.audio_path"
                controls
                preload="metadata"
              ></audio>
              <el-upload
                :show-file-list="false"
                :http-request="uploadAudio"
                accept="audio/mpeg,.mp3"
                ><el-button :loading="audioUploading">{{
                  form.audio_path ? "替换 MP3" : "上传 MP3"
                }}</el-button></el-upload
              >
              <el-button
                v-if="form.audio_path"
                link
                type="danger"
                :loading="audioRemoving"
                @click="removeAudio"
                >移除音频</el-button
              >
            </div>
            <div class="muted">
              MP3，最大 100MB；上传后请先试听，再保存文章。
              <span v-if="form.audio_duration"
                >当前时长 {{ form.audio_duration }}</span
              >
            </div>
          </el-form-item>
          <el-form-item label="Markdown 正文" required
            ><el-input
              v-model="form.body_markdown"
              type="textarea"
              :rows="20"
              @input="schedulePreview"
          /></el-form-item>
          <el-form-item
            ><el-switch v-model="form.enabled" active-text="启用" /><el-switch
              v-model="form.featured"
              class="feature-switch"
              active-text="首页推荐"
              :disabled="!form.enabled"
          /></el-form-item>
        </el-form>
      </section>
      <aside class="panel preview">
        <h2>正文预览</h2>
        <div v-if="previewing" class="muted">正在生成预览…</div>
        <article v-else-if="bodyHTML" v-html="bodyHTML"></article>
        <el-empty v-else description="输入 Markdown 后显示预览" />
      </aside>
    </div>
  </section>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import PageHeader from "../components/PageHeader.vue";
import {
  createAudioNovel,
  getAudioNovel,
  normalizeAudioNovelSlug,
  previewAudioNovelMarkdown,
  updateAudioNovel,
  uploadAudioNovelCover,
  uploadAudioNovelAudio,
  removeAudioNovelAudio,
  formatAudioDuration,
} from "../api/audioNovels.js";
const route = useRoute(),
  router = useRouter(),
  editing = Boolean(route.params.id),
  loading = ref(editing),
  saving = ref(false),
  uploading = ref(false),
  audioUploading = ref(false),
  audioRemoving = ref(false),
  previewing = ref(false),
  bodyHTML = ref("");
const today = new Date().toISOString().slice(0, 10),
  form = reactive({
    title: "",
    slug: "",
    category: "",
    excerpt: "",
    body_markdown: "",
    cover_path: "",
    audio_path: "",
    audio_duration: "",
    audio_duration_seconds: 0,
    audio_size_bytes: 0,
    published_at: today,
    enabled: true,
    featured: false,
  });
let timer;
let savedAudioPath = "";
function schedulePreview() {
  clearTimeout(timer);
  timer = setTimeout(renderPreview, 300);
}
async function renderPreview() {
  previewing.value = true;
  try {
    bodyHTML.value =
      (await previewAudioNovelMarkdown(form.body_markdown)).body_html || "";
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    previewing.value = false;
  }
}
async function upload({ file }) {
  uploading.value = true;
  try {
    if (file.size > 5 * 1024 * 1024) throw new Error("封面不能超过 5MB");
    form.cover_path = (await uploadAudioNovelCover(file)).path;
    ElMessage.success("封面上传成功");
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    uploading.value = false;
  }
}

function readAudioDuration(file) {
  return new Promise((resolve, reject) => {
    const objectURL = URL.createObjectURL(file);
    const audio = new Audio();
    // 浏览器只读取本地元数据来计算时长，不会自动播放或上传临时地址。
    audio.preload = "metadata";
    audio.onloadedmetadata = () => {
      URL.revokeObjectURL(objectURL);
      const seconds = Math.floor(audio.duration);
      Number.isFinite(audio.duration) && seconds >= 1 && seconds <= 86400
        ? resolve({
            display: formatAudioDuration(seconds),
            seconds,
          })
        : reject(new Error("MP3 时长必须在 1 秒到 24 小时之间"));
    };
    audio.onerror = () => {
      URL.revokeObjectURL(objectURL);
      reject(new Error("无法读取 MP3 时长"));
    };
    audio.src = objectURL;
  });
}

async function uploadAudio({ file }) {
  audioUploading.value = true;
  try {
    if (file.size > 100 * 1024 * 1024) throw new Error("音频不能超过 100MB");
    const duration = await readAudioDuration(file);
    // 新文件上传成功后才替换表单元数据；旧文件在文章保存成功后由后端清理。
    const uploaded = await uploadAudioNovelAudio(file);
    form.audio_path = uploaded.path;
    form.audio_duration = duration.display;
    form.audio_duration_seconds = duration.seconds;
    form.audio_size_bytes = uploaded.size_bytes;
    ElMessage.success("MP3 上传成功，请试听后保存");
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    audioUploading.value = false;
  }
}

async function removeAudio() {
  try {
    await ElMessageBox.confirm("确认移除当前 Podcast 音频？", "移除音频", {
      type: "warning",
    });
    audioRemoving.value = true;
    if (editing && form.audio_path === savedAudioPath) {
      await removeAudioNovelAudio(route.params.id);
      savedAudioPath = "";
    }
    form.audio_path = "";
    form.audio_duration = "";
    form.audio_duration_seconds = 0;
    form.audio_size_bytes = 0;
    ElMessage.success("音频已移除");
  } catch (error) {
    if (error !== "cancel" && error !== "close")
      ElMessage.error(error.message || "移除失败");
  } finally {
    audioRemoving.value = false;
  }
}
async function save() {
  if (
    !form.title ||
    !form.slug ||
    !form.category ||
    !form.excerpt ||
    !form.body_markdown ||
    !form.published_at
  ) {
    ElMessage.warning("请填写所有必填项");
    return;
  }
  form.slug = normalizeAudioNovelSlug(form.slug);
  saving.value = true;
  try {
    if (editing) await updateAudioNovel(route.params.id, form);
    else await createAudioNovel(form);
    ElMessage.success("语音小说已保存");
    router.push({ name: "audio-novels" });
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    saving.value = false;
  }
}
onMounted(async () => {
  if (!editing) return;
  try {
    Object.assign(form, await getAudioNovel(route.params.id));
    savedAudioPath = form.audio_path;
    renderPreview();
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    loading.value = false;
  }
});
onBeforeUnmount(() => clearTimeout(timer));
</script>

<style scoped>
.editor-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(360px, 0.85fr);
  gap: 20px;
  align-items: start;
}
.two {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.cover-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}
.cover-preview {
  width: 110px;
  height: 80px;
  border-radius: 6px;
}
.audio-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  width: 100%;
}
.audio-row audio {
  width: min(100%, 420px);
}
.feature-switch {
  margin-left: 24px;
}
.preview {
  position: sticky;
  top: 84px;
  max-height: calc(100vh - 110px);
  overflow: auto;
}
.preview h2 {
  margin-top: 0;
}
.preview article {
  font-family: Georgia, serif;
  font-size: 17px;
  line-height: 1.75;
  color: #31343a;
}
.preview :deep(img) {
  display: none;
}
@media (max-width: 980px) {
  .editor-layout {
    grid-template-columns: 1fr;
  }
  .preview {
    position: static;
    max-height: none;
  }
}
@media (max-width: 620px) {
  .two {
    grid-template-columns: 1fr;
  }
}
</style>
