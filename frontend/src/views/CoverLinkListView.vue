<template>
  <section>
    <PageHeader
      title="封面项目"
      context="封面项目 / 投放链接"
      description="独立管理封面墙、问答与抽奖引导；每条链接单独统计并绑定一个广告平台。"
    >
      <el-button :icon="Refresh" :loading="loading" @click="load"
        >刷新</el-button
      >
      <el-button type="primary" :icon="Plus" @click="open()"
        >创建封面链接</el-button
      >
    </PageHeader>
    <el-alert
      class="notice"
      type="info"
      :closable="false"
      title="封面项目不绑定小说内容，也不提供新增小说、编辑小说或设置推荐；访问完成后停留在封面互动流程。"
    />
    <section class="panel" v-loading="loading">
      <div class="table-toolbar">
        <span class="muted">共 {{ links.length }} 条封面链接</span>
        <!-- 批量删除只归档链接，历史统计和回传记录由后端保留。 -->
        <el-button
          type="danger"
          plain
          :disabled="selectedLinks.length === 0 || deleting"
          :loading="deleting"
          @click="remove(selectedLinks)"
        >
          删除所选<span v-if="selectedLinks.length">
            ({{ selectedLinks.length }})</span
          >
        </el-button>
      </div>
      <el-table
        ref="linkTable"
        :data="links"
        row-key="id"
        empty-text="还没有封面投放链接"
        @selection-change="selectionChanged"
      >
        <el-table-column type="selection" width="48" />
        <el-table-column label="名称 / 地址" min-width="280"
          ><template #default="{ row }"
            ><b>{{ row.name }}</b>
            <div class="url">{{ row.public_url }}</div></template
          ></el-table-column
        >
        <el-table-column label="平台 / Pixel" min-width="210"
          ><template #default="{ row }"
            ><el-tag
              :type="row.ad_platform === 'tiktok' ? 'danger' : 'primary'"
              >{{ row.ad_platform === "tiktok" ? "TikTok" : "Meta" }}</el-tag
            >
            <div class="muted pixel-name">
              {{ selectedPixelName(row) }}
            </div></template
          ></el-table-column
        >
        <el-table-column label="回传门槛" width="120"
          ><template #default="{ row }">{{
            row.time_spent_threshold ? `${row.time_spent_threshold} 秒` : "关闭"
          }}</template></el-table-column
        >
        <el-table-column label="访问" prop="visit_count" width="90" />
        <el-table-column label="状态" width="90"
          ><template #default="{ row }"
            ><el-tag :type="row.enabled ? 'success' : 'info'">{{
              row.enabled ? "使用中" : "已停用"
            }}</el-tag></template
          ></el-table-column
        >
        <el-table-column
          label="操作"
          min-width="380"
          :fixed="isMobile ? false : 'right'"
          ><template #default="{ row }">
            <el-button link type="primary" @click="copyURL(row.public_url)"
              >复制普通短链</el-button
            >
            <el-button
              v-if="row.ad_platform === 'tiktok'"
              link
              type="primary"
              @click="copyURL(row.tiktok_template_url)"
              >复制 TikTok 投放模板</el-button
            >
            <el-button
              link
              type="primary"
              @click="
                router.push({
                  name: 'cover-link-stats',
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
              :disabled="deleting"
              @click="remove([row])"
              >删除</el-button
            >
          </template></el-table-column
        >
      </el-table>
    </section>

    <el-dialog
      v-model="dialog"
      :title="editing ? '编辑封面链接' : '创建封面链接'"
      width="620px"
      class="cover-link-dialog"
      :close-on-click-modal="false"
    >
      <el-form label-position="top" @submit.prevent="save">
        <div class="form-grid">
          <el-form-item label="链接名称" required
            ><el-input
              v-model="form.name"
              maxlength="120"
              placeholder="例如：投手 A · 封面素材 1"
          /></el-form-item>
          <el-form-item v-if="!editing" label="自定义短码（留空自动生成）"
            ><el-input v-model="form.code" placeholder="cover-a"
          /></el-form-item>
        </div>
        <el-form-item label="广告平台" required>
          <el-radio-group
            v-model="form.ad_platform"
            :disabled="Boolean(editing?.first_visited_at)"
            @change="switchPlatform"
          >
            <el-radio-button value="meta">Meta</el-radio-button
            ><el-radio-button value="tiktok">TikTok</el-radio-button>
          </el-radio-group>
          <small v-if="editing?.first_visited_at" class="muted"
            >已有访问，广告平台和 Pixel 不可修改；需要更换时请新建链接。</small
          >
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
            :disabled="Boolean(editing?.first_visited_at)"
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
            :disabled="Boolean(editing?.first_visited_at)"
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
        <el-form-item label="停留时长回传（秒）">
          <el-input-number
            v-model="form.time_spent_threshold"
            :min="0"
            :max="3600"
            :step="1"
          />
          <small class="muted"
            >页面在前台达到此时长后，Meta 回传 TimeSpent，TikTok 回传
            ViewContent；0 表示关闭。</small
          >
        </el-form-item>
      </el-form>
      <template #footer
        ><el-button @click="dialog = false">取消</el-button
        ><el-button type="primary" :loading="saving" @click="save"
          >保存</el-button
        ></template
      >
    </el-dialog>
  </section>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Plus, Refresh } from "@element-plus/icons-vue";
import PageHeader from "../components/PageHeader.vue";
import {
  createCoverLink,
  deleteCoverLink,
  deleteCoverLinks,
  listCoverLinks,
  updateCoverLink,
} from "../api/coverLinks.js";
import { listConnections, listPixels } from "../api/meta.js";
import { listTikTokConnections, listTikTokPixels } from "../api/tiktok.js";
import { connectionIDForPixel, pixelSelectable } from "../utils/meta.js";

const router = useRouter();
const loading = ref(false),
  saving = ref(false),
  deleting = ref(false),
  dialog = ref(false),
  editing = ref(null);
const linkTable = ref(null),
  selectedLinks = ref([]);
const links = ref([]),
  connections = ref([]),
  pixels = ref([]),
  tiktokConnections = ref([]),
  tiktokPixels = ref([]);
const blank = () => ({
  name: "",
  code: "",
  enabled: true,
  ad_platform: "meta",
  attribution_mode: "dynamic",
  meta_connection_id: null,
  meta_pixel_id: null,
  tiktok_pixel_id: null,
  time_spent_threshold: 10,
});
const form = reactive(blank());
const mobileQuery = window.matchMedia("(max-width:700px)");
const isMobile = ref(mobileQuery.matches);
const syncMobile = (event) => {
  isMobile.value = event.matches;
};

async function load() {
  loading.value = true;
  try {
    const [
      linkResult,
      accountResult,
      pixelResult,
      ttAccountResult,
      ttPixelResult,
    ] = await Promise.all([
      listCoverLinks(),
      listConnections(),
      listPixels(),
      listTikTokConnections(),
      listTikTokPixels(),
    ]);
    links.value = linkResult.items;
    // Refreshing the list must also clear stale selections that may already be archived.
    selectedLinks.value = [];
    linkTable.value?.clearSelection();
    connections.value = accountResult;
    pixels.value = pixelResult;
    tiktokConnections.value = ttAccountResult;
    tiktokPixels.value = ttPixelResult;
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    loading.value = false;
  }
}
function open(row) {
  editing.value = row || null;
  Object.assign(form, blank(), row || {});
  dialog.value = true;
}
function switchPlatform(platform) {
  if (platform === "tiktok") {
    form.meta_connection_id = null;
    form.meta_pixel_id = null;
  } else form.tiktok_pixel_id = null;
}
function selectMetaPixel(id) {
  form.meta_connection_id = connectionIDForPixel(pixels.value, id);
}
function pixelLabel(pixel) {
  return `${connections.value.find((item) => Number(item.id) === Number(pixel.connection_id))?.name || "未知账户"} · ${pixel.name} · ${pixel.pixel_id}`;
}
function tiktokPixelLabel(pixel) {
  return `${tiktokConnections.value.find((item) => Number(item.id) === Number(pixel.connection_id))?.name || "未知凭证"} · ${pixel.name} · ${pixel.pixel_code}`;
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
async function save() {
  const pixel =
    form.ad_platform === "tiktok" ? form.tiktok_pixel_id : form.meta_pixel_id;
  if (!form.name.trim() || !pixel) {
    ElMessage.warning(
      `请填写链接名称并选择 ${form.ad_platform === "tiktok" ? "TikTok" : "Meta"} Pixel`,
    );
    return;
  }
  if (form.ad_platform === "meta") selectMetaPixel(form.meta_pixel_id);
  saving.value = true;
  try {
    if (editing.value) await updateCoverLink(editing.value.id, form);
    else await createCoverLink(form);
    dialog.value = false;
    ElMessage.success("封面链接已保存");
    await load();
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    saving.value = false;
  }
}
async function toggle(row) {
  try {
    await updateCoverLink(row.id, { ...row, enabled: !row.enabled });
    ElMessage.success(row.enabled ? "链接已停用" : "链接已启用");
    await load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
function selectionChanged(rows) {
  selectedLinks.value = rows;
}
async function remove(rows) {
  if (!rows.length || deleting.value) return;
  const label =
    rows.length === 1
      ? `“${rows[0].name}”`
      : `选中的 ${rows.length} 条封面链接`;
  try {
    await ElMessageBox.confirm(
      `确认删除${label}？删除后链接将失效并从列表隐藏，历史统计会保留。`,
      "删除封面链接",
      { type: "warning", confirmButtonText: "确认删除" },
    );
    deleting.value = true;
    if (rows.length === 1) await deleteCoverLink(rows[0].id);
    else await deleteCoverLinks(rows.map((row) => row.id));
    ElMessage.success(`已删除 ${rows.length} 条链接，历史统计已保留`);
    await load();
  } catch (error) {
    if (error !== "cancel" && error !== "close")
      ElMessage.error(error.message || "删除失败");
  } finally {
    deleting.value = false;
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
.table-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}
:deep(.cover-link-dialog) {
  max-width: calc(100vw - 24px);
}
:deep(.cover-link-dialog .el-dialog__body) {
  max-height: calc(100vh - 280px);
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
