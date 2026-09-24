<template>
  <section>
    <PageHeader
      :title="
        audioNovel
          ? `《${audioNovel.title}》语音小说投放链接`
          : '语音小说投放链接'
      "
      context="语音小说项目 / 独立归因"
      description="同一段音频可为不同投手创建独立链接，分别统计访问、实际播放与广告回传。"
    >
      <el-button @click="router.push({ name: 'audio-novels' })"
        >返回语音小说</el-button
      ><el-button :icon="Refresh" :loading="loading" @click="load"
        >刷新</el-button
      ><el-button type="primary" :icon="Plus" @click="open()"
        >创建投放链接</el-button
      >
    </PageHeader>
    <el-alert
      class="notice"
      type="info"
      :closable="false"
      title="建议每位投手或每次投放创建一条独立链接。链接产生首次访问后，语音内容、广告平台和 Pixel 会锁定。"
    />
    <section class="panel" v-loading="loading">
      <el-table :data="links" empty-text="这部语音小说还没有投放链接">
        <el-table-column label="名称 / 地址" min-width="280">
          <template #default="{ row }">
            <b>{{ row.name }}</b>
            <div class="url">{{ row.public_url }}</div>
          </template>
        </el-table-column>
        <el-table-column label="平台 / Pixel" min-width="200">
          <template #default="{ row }">
            <el-tag :type="row.ad_platform === 'tiktok' ? 'danger' : 'primary'">
              {{ row.ad_platform === "tiktok" ? "TikTok" : "Meta" }}
            </el-tag>
            <div class="muted pixel-name">{{ selectedPixelName(row) }}</div>
          </template>
        </el-table-column>
        <el-table-column label="达标播放" width="110">
          <template #default="{ row }">
            {{
              row.time_spent_threshold
                ? `${row.time_spent_threshold} 秒`
                : "关闭"
            }}
          </template>
        </el-table-column>
        <el-table-column label="访问" prop="visit_count" width="80" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'">
              {{ row.enabled ? "使用中" : "已停用" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="首次访问" min-width="170">
          <template #default="{ row }">
            {{
              row.first_visited_at ? dateTime(row.first_visited_at) : "尚无访问"
            }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          min-width="420"
          :fixed="isMobile ? false : 'right'"
        >
          <template #default="{ row }">
            <el-button link type="primary" @click="copyURL(row.public_url)"
              >复制普通短链</el-button
            >
            <el-button
              v-if="row.ad_platform === 'tiktok'"
              link
              type="primary"
              @click="copyTikTokTemplate(row)"
              >复制 TikTok 投放模板</el-button
            >
            <el-button
              link
              type="primary"
              @click="
                router.push({
                  name: 'audio-novel-link-stats',
                  params: { id: row.id },
                })
              "
              >统计</el-button
            >
            <el-button link @click="open(row)">编辑</el-button>
            <el-button
              link
              :type="row.enabled ? 'danger' : 'success'"
              @click="toggle(row)"
              >{{ row.enabled ? "停用" : "启用" }}</el-button
            >
            <el-button
              link
              type="danger"
              :disabled="
                Number(row.visit_count) > 0 || Boolean(row.first_visited_at)
              "
              @click="remove(row)"
              >删除</el-button
            >
          </template>
        </el-table-column>
      </el-table>
    </section>

    <el-dialog
      v-model="dialog"
      :title="editing ? '编辑语音小说投放链接' : '创建语音小说投放链接'"
      width="620px"
      class="audio-link-dialog"
      :close-on-click-modal="false"
    >
      <el-form label-position="top" @submit.prevent="save">
        <div class="form-grid">
          <el-form-item label="链接名称" required>
            <el-input
              v-model="form.name"
              maxlength="120"
              placeholder="例如：投手 A · 广告组 1"
            />
          </el-form-item>
          <el-form-item v-if="!editing" label="自定义短码（留空自动生成）">
            <el-input v-model="form.code" placeholder="audio-buyer-a" />
          </el-form-item>
        </div>
        <el-form-item label="绑定语音小说" required>
          <el-select
            v-model="form.audio_novel_id"
            style="width: 100%"
            :disabled="bindingLocked"
          >
            <el-option
              v-for="item in audioNovels"
              :key="item.id"
              :label="item.title"
              :value="item.id"
              :disabled="!item.enabled || !item.audio_path"
            />
          </el-select>
          <small v-if="bindingLocked" class="muted">
            已有访问，内容绑定不可修改；需要更换时请新建链接。
          </small>
        </el-form-item>
        <el-form-item label="广告平台" required>
          <el-radio-group
            v-model="form.ad_platform"
            :disabled="bindingLocked"
            @change="switchPlatform"
          >
            <el-radio-button value="meta">Meta</el-radio-button>
            <el-radio-button value="tiktok">TikTok</el-radio-button>
          </el-radio-group>
          <small v-if="bindingLocked" class="muted">
            已有访问，广告平台和 Pixel 不可修改；需要更换时请新建链接。
          </small>
        </el-form-item>
        <el-form-item
          v-if="form.ad_platform === 'meta'"
          label="Meta Pixel"
          required
        >
          <el-select
            v-model="form.meta_pixel_id"
            style="width: 100%"
            placeholder="选择回传 Pixel"
            :disabled="bindingLocked"
            @change="selectMetaPixel"
          >
            <el-option
              v-for="pixel in pixels"
              :key="pixel.id"
              :value="pixel.id"
              :label="pixelLabel(pixel)"
              :disabled="!pixelSelectable(pixel)"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-else label="TikTok Pixel" required>
          <el-select
            v-model="form.tiktok_pixel_id"
            style="width: 100%"
            placeholder="选择回传 TikTok Pixel"
            :disabled="bindingLocked"
          >
            <el-option
              v-for="pixel in tiktokPixels"
              :key="pixel.id"
              :value="pixel.id"
              :label="tiktokPixelLabel(pixel)"
              :disabled="!tiktokPixelSelectable(pixel)"
            />
          </el-select>
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="达标播放时长（秒）">
            <el-input-number
              v-model="form.time_spent_threshold"
              :min="0"
              :max="3600"
              :step="1"
            />
            <small class="muted">
              用户实际播放达到该时长后回传一次 ViewContent；默认 10 秒，0
              表示关闭。
            </small>
          </el-form-item>
          <el-form-item label="链接状态">
            <el-switch
              v-model="form.enabled"
              active-text="启用"
              inactive-text="停用"
            />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save"
          >保存</el-button
        >
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Plus, Refresh } from "@element-plus/icons-vue";
import PageHeader from "../components/PageHeader.vue";
import { listAudioNovels } from "../api/audioNovels.js";
import {
  createAudioNovelLink,
  deleteAudioNovelLink,
  listAudioNovelLinks,
  updateAudioNovelLink,
} from "../api/audioNovelLinks.js";
import { listConnections, listPixels } from "../api/meta.js";
import { listTikTokConnections, listTikTokPixels } from "../api/tiktok.js";
import { connectionIDForPixel, pixelSelectable } from "../utils/meta.js";
import { tiktokTemplateURL } from "../utils/tiktok.js";

const route = useRoute();
const router = useRouter();
const loading = ref(false);
const saving = ref(false);
const dialog = ref(false);
const editing = ref(null);
const links = ref([]);
const audioNovels = ref([]);
const connections = ref([]);
const pixels = ref([]);
const tiktokConnections = ref([]);
const tiktokPixels = ref([]);
const selectedAudioNovelID = computed(() => Number(route.params.id));
const audioNovel = computed(() =>
  audioNovels.value.find((item) => item.id === selectedAudioNovelID.value),
);
const bindingLocked = computed(() => Boolean(editing.value?.first_visited_at));

// Fixed action columns obscure content on phones, so disable them at the same
// breakpoint used by the existing text-novel campaign page.
const mobileQuery = window.matchMedia("(max-width: 700px)");
const isMobile = ref(mobileQuery.matches);
const syncMobile = (event) => (isMobile.value = event.matches);

const blank = () => ({
  name: "",
  code: "",
  audio_novel_id: selectedAudioNovelID.value,
  enabled: true,
  ad_platform: "meta",
  meta_connection_id: null,
  meta_pixel_id: null,
  tiktok_pixel_id: null,
  // The threshold is actual native audio playback, not page dwell time.
  time_spent_threshold: 10,
});
const form = reactive(blank());

function open(row) {
  editing.value = row || null;
  Object.assign(form, blank(), row || {}, {
    ad_platform: row?.ad_platform === "tiktok" ? "tiktok" : "meta",
  });
  dialog.value = true;
}

function switchPlatform(platform) {
  if (platform === "tiktok") {
    form.meta_connection_id = null;
    form.meta_pixel_id = null;
  } else {
    form.tiktok_pixel_id = null;
  }
}

function selectMetaPixel(id) {
  form.meta_connection_id = connectionIDForPixel(pixels.value, id);
}

function pixelLabel(pixel) {
  const account = connections.value.find(
    (item) => Number(item.id) === Number(pixel.connection_id),
  );
  return `${account?.name || "未知账户"} · ${pixel.name} · ${pixel.pixel_id}`;
}

function tiktokPixelLabel(pixel) {
  const connection = tiktokConnections.value.find(
    (item) => Number(item.id) === Number(pixel.connection_id),
  );
  return `${connection?.name || "未知凭证"} · ${pixel.name} · ${pixel.pixel_code}`;
}

function tiktokPixelSelectable(pixel) {
  const connection = tiktokConnections.value.find(
    (item) => Number(item.id) === Number(pixel.connection_id),
  );
  return Boolean(
    pixel?.enabled &&
    connection?.enabled &&
    connection?.has_access_token &&
    connection?.credential_status !== "invalid",
  );
}

function selectedPixelName(row) {
  if (row.ad_platform === "tiktok")
    return row.tiktok_pixel_name || row.tiktok_pixel_code || "未绑定";
  return (
    pixels.value.find((pixel) => Number(pixel.id) === Number(row.meta_pixel_id))
      ?.name || "Meta Pixel"
  );
}

function dateTime(value) {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

async function load() {
  loading.value = true;
  try {
    const [
      linkResult,
      novelResult,
      accountResult,
      pixelResult,
      tiktokConnectionResult,
      tiktokPixelResult,
    ] = await Promise.all([
      listAudioNovelLinks(selectedAudioNovelID.value),
      listAudioNovels({ page_size: 100 }),
      listConnections(),
      listPixels(),
      listTikTokConnections(),
      listTikTokPixels(),
    ]);
    links.value = linkResult.items;
    audioNovels.value = novelResult.items;
    connections.value = accountResult;
    pixels.value = pixelResult;
    tiktokConnections.value = tiktokConnectionResult;
    tiktokPixels.value = tiktokPixelResult;
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    loading.value = false;
  }
}

async function save() {
  const selectedPixel =
    form.ad_platform === "tiktok" ? form.tiktok_pixel_id : form.meta_pixel_id;
  if (!form.name.trim() || !form.audio_novel_id || !selectedPixel) {
    ElMessage.warning(
      `请填写链接名称、绑定语音小说并选择 ${form.ad_platform === "tiktok" ? "TikTok" : "Meta"} Pixel`,
    );
    return;
  }
  if (form.ad_platform === "meta") selectMetaPixel(form.meta_pixel_id);
  saving.value = true;
  try {
    if (editing.value) await updateAudioNovelLink(editing.value.id, form);
    else await createAudioNovelLink(form);
    dialog.value = false;
    ElMessage.success("语音小说投放链接已保存");
    await load();
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    saving.value = false;
  }
}

async function toggle(row) {
  try {
    await updateAudioNovelLink(row.id, { ...row, enabled: !row.enabled });
    ElMessage.success(row.enabled ? "链接已停用" : "链接已启用");
    await load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`确认删除“${row.name}”？`, "删除投放链接", {
      type: "warning",
    });
    await deleteAudioNovelLink(row.id);
    ElMessage.success("链接已删除");
    await load();
  } catch (error) {
    if (error !== "cancel" && error !== "close")
      ElMessage.error(error.message || "删除失败");
  }
}

async function copyURL(value) {
  try {
    await navigator.clipboard.writeText(value);
    ElMessage.success("链接已复制");
  } catch {
    ElMessage.warning("无法访问剪贴板，请手动复制");
  }
}

function copyTikTokTemplate(row) {
  // Prefer the server template; older cached responses use the same macro helper.
  const origin = new URL(row.public_url).origin;
  return copyURL(
    row.tiktok_template_url ||
      tiktokTemplateURL(origin, row.code, "audio-novel"),
  );
}

onMounted(() => {
  mobileQuery.addEventListener("change", syncMobile);
  load();
});
onBeforeUnmount(() => mobileQuery.removeEventListener("change", syncMobile));
</script>

<style scoped>
.url {
  margin-top: 5px;
  color: #409eff;
  font-size: 12px;
  overflow-wrap: anywhere;
}
.pixel-name {
  margin-top: 5px;
}
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}
:deep(.audio-link-dialog) {
  max-width: calc(100vw - 24px);
}
:deep(.audio-link-dialog .el-dialog__body) {
  max-height: calc(100vh - 300px);
  overflow-y: auto;
}
@media (max-width: 700px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
  .panel {
    overflow-x: auto;
  }
}
</style>
