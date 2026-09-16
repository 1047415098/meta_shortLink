<template>
  <section class="links-page">
    <PageHeader
      title="短链接管理"
      context="链接与渠道管理"
      description="将每一条广告连接到 WhatsApp，让来源清晰可追踪。"
      ><el-button :icon="Refresh" :loading="busy" @click="load">刷新</el-button
      ><el-button type="primary" :icon="Plus" @click="openLink()"
        >创建短链接</el-button
      ></PageHeader
    ><el-alert v-if="error" :title="error" type="error" :closable="false" />
    <section class="panel" v-loading="busy">
      <div class="panel-heading">
        <h2>
          全部链接 <span class="count">{{ links.length }}</span>
        </h2>
        <span class="muted">每条广告建议分配一个独立链接</span>
      </div>
      <div class="table-toolbar">
        <div class="toolbar-actions">
          <el-radio-group v-model="linkStatus"
            ><el-radio-button value="all"
              >全部 {{ links.length }}</el-radio-button
            ><el-radio-button value="active"
              >使用中 {{ activeLinks }}</el-radio-button
            ><el-radio-button value="inactive"
              >已停用</el-radio-button
            ></el-radio-group
          ><!-- Selected rows are deleted through the same confirmation path as a single row. -->
          <el-button
            type="danger"
            plain
            :disabled="selectedLinks.length === 0 || deleting"
            :loading="deleting"
            @click="confirmDelete(selectedLinks)"
            >删除所选<span v-if="selectedLinks.length">
              ({{ selectedLinks.length }})</span
            ></el-button
          >
        </div>
        <el-input
          v-model="linkSearch"
          clearable
          :prefix-icon="Search"
          placeholder="搜索名称、短码或广告 ID"
          aria-label="搜索短链接"
          class="table-search"
        />
      </div>
      <el-table
        ref="linkTable"
        :data="visibleLinks"
        row-key="id"
        empty-text="当前筛选条件下没有短链接"
        @selection-change="selectionChanged"
        ><el-table-column
          type="selection"
          width="48"
          reserve-selection
        /><el-table-column label="名称 / 短链接" min-width="240"
          ><template #default="{ row }"
            ><div class="link-name">
              <span class="link-avatar"
                ><el-icon><Link /></el-icon
              ></span>
              <div>
                <b>{{ row.name }}</b>
                <div class="url">{{ shortURL(row) }}</div>
              </div>
            </div></template
          ></el-table-column
        ><el-table-column label="访问模式" width="120"
          ><template #default="{ row }"
            ><el-tag :type="row.mode === 'landing' ? 'success' : 'info'">{{
              row.mode === "landing" ? "网站落地页" : "直接跳转"
            }}</el-tag></template
          ></el-table-column
        ><el-table-column
          prop="channel"
          label="渠道"
          width="110"
        /><el-table-column
          prop="ad_id"
          label="广告 ID"
          min-width="140"
        /><el-table-column label="状态" width="95"
          ><template #default="{ row }"
            ><el-tag :type="row.enabled ? 'success' : 'info'">{{
              row.enabled ? "使用中" : "已停用"
            }}</el-tag></template
          ></el-table-column
        ><el-table-column label="操作" min-width="290" fixed="right"
          ><template #default="{ row }"
            ><el-button link type="primary" @click="copy(row)">复制</el-button
            ><el-button link type="primary" @click="detail(row)">统计</el-button
            ><el-button link @click="openLink(row)">编辑</el-button
            ><el-button
              link
              :type="row.enabled ? 'danger' : 'success'"
              @click="toggle(row)"
              >{{ row.enabled ? "停用" : "启用" }}</el-button
            ><el-button
              link
              type="danger"
              :disabled="deleting"
              @click="confirmDelete([row])"
              >删除</el-button
            ></template
          ></el-table-column
        ></el-table
      >
    </section>
    <el-dialog
      v-model="dialog"
      :title="editing ? '编辑短链接' : '创建短链接'"
      width="580px"
      class="link-dialog"
      :close-on-click-modal="false"
      ><div class="dialog-intro">
        <el-icon><Link /></el-icon
        ><span>选择直接跳转或网站落地页，分别统计访问与咨询按钮点击。</span>
      </div>
      <el-form label-position="top" @submit.prevent="saveLink"
        ><el-form-item label="链接名称" required
          ><el-input
            v-model="form.name"
            maxlength="120"
            placeholder="例如：秋季活动 · 广告 A" /></el-form-item
        ><el-form-item label="WhatsApp 目标地址" required
          ><el-input
            v-model="form.target_url"
            placeholder="https://wa.me/13365661092" /></el-form-item
        ><el-form-item label="访问模式">
          <el-radio-group v-model="form.mode"
            ><el-radio-button value="landing">网站落地页</el-radio-button
            ><el-radio-button value="redirect"
              >直接跳转</el-radio-button
            ></el-radio-group
          >
        </el-form-item>
        <template v-if="form.mode === 'landing'">
          <el-alert
            title="访客先浏览产品介绍，主动点击按钮后前往 WhatsApp。"
            type="info"
            :closable="false"
            style="margin-bottom: 16px"
          />
          <el-form-item label="品牌 / 展示名称" required
            ><el-input
              v-model="form.landing_brand"
              maxlength="40"
              show-word-limit
          /></el-form-item>
          <el-form-item label="页面标题" required
            ><el-input
              v-model="form.landing_title"
              maxlength="80"
              show-word-limit
          /></el-form-item>
          <el-form-item label="产品简介" required
            ><el-input
              v-model="form.landing_description"
              type="textarea"
              :rows="3"
              maxlength="800"
              show-word-limit
          /></el-form-item>
          <!-- Keep editable landing content together before redirect behavior settings. -->
          <el-form-item label="详细介绍"
            ><el-input
              v-model="form.landing_details"
              type="textarea"
              :rows="5"
              maxlength="2000"
              show-word-limit
          /></el-form-item>
          <el-form-item label="定时跳转（秒）"
            ><el-input
              v-model.number="form.landing_delay"
              type="number"
              min="0"
              max="300"
              step="1"
            /><small class="muted"
              >0 表示关闭；1–300
              秒后自动跳转。自动跳转单独记录，不计入咨询按钮点击。</small
            ></el-form-item
          >
        </template>
        <el-form-item v-if="!editing" label="自定义短码（留空自动生成）"
          ><el-input
            v-model="form.code"
            maxlength="32"
            placeholder="仅字母、数字、短横线或下划线" /></el-form-item
        ><el-divider content-position="left">广告归因</el-divider>
        <!-- 运营人员只选择最终回传 Pixel，所属账户由 Pixel 关系自动确定。 -->
        <el-form-item label="Meta Pixel" required>
          <el-select
            v-model="form.meta_pixel_id"
            placeholder="选择回传 Pixel"
            style="width: 100%"
            @change="selectMetaPixel"
          >
            <el-option
              v-for="pixel in metaPixels"
              :key="pixel.id"
              :value="pixel.id"
              :label="pixelOptionLabel(pixel)"
              :disabled="!pixelSelectable(pixel)"
            />
          </el-select>
          <small class="muted"
            >选择 Pixel 后自动绑定所属账户；CAPI Token 仅在 Pixel
            页面维护。</small
          >
          <small v-if="selectedMetaConnection" class="muted"
            >所属账户：{{ selectedMetaConnection.name }} ·
            {{ selectedMetaConnection.account_id }}</small
          >
          <el-button
            link
            type="primary"
            @click="router.push({ name: 'meta-pixels' })"
            >管理 Meta Pixel</el-button
          >
          <el-alert
            v-if="metaError"
            :title="metaError"
            type="error"
            :closable="false"
          />
        </el-form-item>
        <el-alert
          class="meta-binding-notice"
          :title="metaBindingNotice.title"
          :type="metaBindingNotice.type"
          :closable="false"
          show-icon
        />
        <el-form-item label="广告归因方式" required>
          <el-select v-model="form.attribution_mode" style="width: 100%">
            <el-option value="bound" label="固定绑定：使用下方填写的广告 ID" />
            <el-option
              value="dynamic"
              label="动态归因：读取访问 URL 的广告 ID（新 Meta 链接推荐）"
            />
          </el-select>
          <small class="muted"
            >已有链接保留固定绑定。动态模式适合多个广告共用短链接，投放 URL
            需携带 campaign_id、adset_id、ad_id。</small
          >
        </el-form-item>
        <el-form-item label="Meta 网址参数">
          <!-- Disable direct editing so every operator copies the same canonical Meta template. -->
          <el-input
            :model-value="META_URL_PARAMETERS"
            type="textarea"
            :rows="4"
            disabled
          />
          <div class="meta-parameter-actions">
            <el-button type="primary" plain @click="copyMetaURLParameters"
              >一键复制 Meta 网址参数</el-button
            >
            <small class="muted"
              >粘贴到每条广告的“网址参数”；fbclid 由 Meta 自动添加。</small
            >
          </div>
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="渠道"
            ><el-input v-model="form.channel" /></el-form-item
          ><el-form-item label="广告 ID"
            ><el-input v-model="form.ad_id" /></el-form-item
          ><el-form-item label="广告系列 ID"
            ><el-input v-model="form.campaign_id" /></el-form-item
          ><el-form-item label="广告组 ID"
            ><el-input v-model="form.adset_id"
          /></el-form-item>
        </div>
        <!-- Availability is intentionally absent here; operators use the list action to enable or disable a link. --> </el-form
      ><template #footer
        ><el-button @click="dialog = false">取消</el-button
        ><el-button type="primary" :loading="saving" @click="saveLink"
          >保存链接</el-button
        ></template
      ></el-dialog
    >
  </section>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus/es/components/message/index";
import { ElMessageBox } from "element-plus/es/components/message-box/index";
import PageHeader from "../components/PageHeader.vue";
import { Link, Refresh, Plus, Search } from "@element-plus/icons-vue";

import { settings } from "../stores/settings";
import { validateLink } from "../utils";
import {
  deleteLinks,
  listLinks,
  saveLink as persistLink,
  updateLink,
} from "../api/links";
import { listConnections, listPixels } from "../api/meta";
import {
  pixelSelectable,
  connectionIDForPixel,
  pixelUnavailableReason,
  META_URL_PARAMETERS,
} from "../utils/meta";
const router = useRouter();
const links = ref([]);
const metaConnections = ref([]);
const metaPixels = ref([]);
const selectedMetaConnection = computed(() =>
  metaConnections.value.find(
    (connection) => connection.id === form.meta_connection_id,
  ),
);
const selectedMetaPixel = computed(() =>
  metaPixels.value.find(
    (pixel) => Number(pixel.id) === Number(form.meta_pixel_id),
  ),
);
// Keep the operator informed whether the saved link can actually create CAPI events.
const metaBindingNotice = computed(() => {
  if (!form.meta_pixel_id)
    return {
      type: "error",
      title: "请选择 Meta Pixel，链接保存后才能统一统计并回传事件。",
    };
  if (!selectedMetaConnection.value)
    return { type: "error", title: "当前 Pixel 的所属账户不存在。" };
  if (!pixelSelectable(selectedMetaPixel.value))
    return { type: "error", title: "当前 Pixel 不可用于 CAPI 回传。" };
  return {
    type: "success",
    title: `事件将回传至 ${selectedMetaConnection.value.name} 的 Pixel ${selectedMetaPixel.value.pixel_id}。`,
  };
});
const metaError = ref("");
const busy = ref(false);
const saving = ref(false);
const deleting = ref(false);
const error = ref("");
const dialog = ref(false);
const editing = ref(null);
const form = reactive({
  name: "",
  code: "",
  target_url: "https://wa.me/13365661092",
  mode: "landing",
  landing_delay: 3,
  landing_brand: "PEPLYRA Research",
  landing_title: "Research materials. Clearer possibilities.",
  landing_description:
    "Explore selected peptide materials and discuss specifications, batch documentation and availability for your laboratory research.",
  landing_details:
    "Product specifications\nAsk about the material, format and available pack sizes.\n\nBatch documentation\nRequest the relevant COA and analytical information.\n\nAvailability and delivery\nShare the product name and destination to discuss current options.",
  // Availability remains internal to the form payload: new links start enabled,
  // while edits inherit the existing row state without exposing another control.
  enabled: true,
  campaign_id: "",
  adset_id: "",
  ad_id: "",
  channel: "facebook",
  meta_connection_id: null,
  // New links never inherit a default Pixel; the operator chooses the CAPI target explicitly.
  meta_pixel_id: null,
  // New links always use the URL's current ad IDs so one short link can serve
  // multiple ads without inheriting a fixed ID from the creation form.
  attribution_mode: "dynamic",
});
const linkSearch = ref("");
const linkStatus = ref("all");
const linkTable = ref(null);
const selectedLinks = ref([]);
const visibleLinks = computed(() =>
  links.value.filter(
    (l) =>
      (!linkSearch.value ||
        [l.name, l.code, l.ad_id].some((v) =>
          String(v || "")
            .toLowerCase()
            .includes(linkSearch.value.toLowerCase()),
        )) &&
      (linkStatus.value === "all" ||
        (linkStatus.value === "active" ? l.enabled : !l.enabled)),
  ),
);
const activeLinks = computed(
  // Automatic expiry was removed; only the explicit enabled flag controls availability.
  () => links.value.filter((l) => l.enabled).length,
);
// Element Plus supplies the full selected rows, preserving names for confirmation copy.
function selectionChanged(rows) {
  selectedLinks.value = rows;
}

async function confirmDelete(rows) {
  const chosen = [...new Map(rows.map((row) => [row.id, row])).values()];
  if (!chosen.length || deleting.value) return;
  const preview = chosen
    .slice(0, 3)
    .map((row) => row.name)
    .join("、");
  const suffix = chosen.length > 3 ? ` 等 ${chosen.length} 条` : "";
  try {
    await ElMessageBox.confirm(
      `即将永久删除 ${preview}${suffix}，对应访问统计、CAPI 回传记录和访客日志也会删除。此操作无法撤销。`,
      `确认删除 ${chosen.length} 条短链接？`,
      {
        type: "warning",
        confirmButtonText: "永久删除",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
      },
    );
  } catch {
    return;
  }
  deleting.value = true;
  error.value = "";
  try {
    const result = await deleteLinks(chosen.map((row) => row.id));
    // Clear stale selections before loading the authoritative list again.
    linkTable.value?.clearSelection();
    selectedLinks.value = [];
    await load();
    ElMessage.success(`已删除 ${result.deleted} 条短链接`);
  } catch (e) {
    error.value = e.message;
  } finally {
    deleting.value = false;
  }
}
function selectMetaPixel(value) {
  form.meta_pixel_id = value || null;
  // Persist the owning account automatically so the backend's composite
  // relationship remains valid without making the operator choose twice.
  form.meta_connection_id = connectionIDForPixel(metaPixels.value, value);
  if (!editing.value && value) form.attribution_mode = "dynamic";
}
// Disabled options remain visible so administrators can understand what to fix.
function pixelOptionLabel(pixel) {
  const reason = pixelUnavailableReason(pixel);
  const account = metaConnections.value.find(
    (connection) => Number(connection.id) === Number(pixel.connection_id),
  );
  return `${account?.name || "未知账户"} · ${pixel.name} · ${pixel.pixel_id}${reason ? ` · ${reason}` : ""}`;
}
function openLink(row) {
  editing.value = row?.id || null;
  Object.assign(
    form,
    {
      name: "",
      code: "",
      target_url: "https://wa.me/13365661092",
      mode: "landing",
      landing_delay: 3,
      landing_brand: "PEPLYRA Research",
      landing_title: "Research materials. Clearer possibilities.",
      landing_description:
        "Explore selected peptide materials and discuss specifications, batch documentation and availability for your laboratory research.",
      landing_details:
        "Product specifications\nAsk about the material, format and available pack sizes.\n\nBatch documentation\nRequest the relevant COA and analytical information.\n\nAvailability and delivery\nShare the product name and destination to discuss current options.",
      // The reset default applies only to creation; Object.assign below keeps
      // an edited link's existing status until the list action changes it.
      enabled: true,
      campaign_id: "",
      adset_id: "",
      ad_id: "",
      channel: "facebook",
      meta_connection_id: null,
      meta_pixel_id: null,
      // Creation resets to dynamic; an edited row below keeps its stored mode.
      attribution_mode: "dynamic",
    },
    row || {},
  );
  form.meta_connection_id = row?.meta_connection_id || null;
  form.meta_pixel_id = row?.meta_pixel_id || null;
  form.attribution_mode = row?.attribution_mode || "dynamic";
  dialog.value = true;
}
async function saveLink() {
  const invalid = validateLink(form);
  if (invalid) {
    ElMessage.warning(invalid);
    return;
  }
  // Re-derive the pair at save time so stale UI state cannot bind a Pixel to
  // the wrong account or silently depend on the legacy account CAPI switch.
  form.meta_connection_id = connectionIDForPixel(
    metaPixels.value,
    form.meta_pixel_id,
  );
  if (form.meta_pixel_id && !pixelSelectable(selectedMetaPixel.value)) {
    ElMessage.warning("当前 Pixel 无法用于 CAPI 回传");
    return;
  }
  if (!form.meta_pixel_id) form.meta_connection_id = null;
  saving.value = true;
  try {
    const payload = { ...form };
    delete payload.id;
    delete payload.created_at;
    await persistLink(editing.value, payload);
    links.value = await listLinks();
    dialog.value = false;
    ElMessage.success("链接已保存");
  } catch (e) {
    ElMessage.error(e.message);
  } finally {
    saving.value = false;
  }
}
async function toggle(row) {
  try {
    await updateLink(row.id, { enabled: !row.enabled });
    row.enabled = !row.enabled;
    ElMessage.success(row.enabled ? "链接已启用" : "链接已停用");
  } catch (e) {
    ElMessage.error(e.message);
  }
}
function shortURL(row) {
  return (
    (settings.value.public_base_url || window.location.origin).replace(
      /\/$/,
      "",
    ) +
    "/" +
    row.code
  );
}
async function copy(row) {
  try {
    await navigator.clipboard.writeText(shortURL(row));
    ElMessage.success("短链接已复制");
  } catch {
    ElMessage.warning("无法访问剪贴板，请手动复制链接");
  }
}
async function copyMetaURLParameters() {
  // Copy only the public tracking template; credentials never belong in an ad URL.
  try {
    await navigator.clipboard.writeText(META_URL_PARAMETERS);
    ElMessage.success("Meta 网址参数已复制");
  } catch {
    ElMessage.warning("无法访问剪贴板，请手动复制网址参数");
  }
}
async function load() {
  busy.value = true;
  error.value = "";
  try {
    links.value = await listLinks();
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
function detail(row) {
  // Open only this link's ad comparison; the overview remains a separate report.
  router.push({ name: "link-stats", params: { id: String(row.id) } });
}
onMounted(() => {
  load();
  listConnections()
    .then((rows) => {
      metaConnections.value = rows;
    })
    .catch((e) => {
      metaError.value = "Meta 帐号读取失败：" + e.message;
    });
  listPixels()
    .then((rows) => {
      metaPixels.value = rows;
      // Existing links retain their selected Pixel while its owning account is
      // normalized after asynchronous Pixel data arrives.
      if (dialog.value && form.meta_pixel_id)
        form.meta_connection_id = connectionIDForPixel(
          rows,
          form.meta_pixel_id,
        );
    })
    .catch((e) => {
      metaError.value = "Pixel 读取失败：" + e.message;
    });
});
</script>

<style scoped>
.table-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 22px;
}
.toolbar-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}
.table-search {
  max-width: 280px;
}
.link-name {
  display: flex;
  align-items: center;
  gap: 12px;
}
.link-avatar {
  height: 34px;
  width: 34px;
  background: #ecf5ff;
  color: #409eff;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-size: 18px;
}
.url {
  font-size: 12px;
  color: #909399;
  margin-top: 5px;
  word-break: break-all;
}
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 18px;
}
.link-dialog {
  max-width: calc(100vw - 30px);
  border-radius: 12px;
  padding: 24px;
}
.link-dialog :deep(.el-dialog__title) {
  font-size: 18px;
  font-weight: 600;
}
.link-dialog :deep(.el-dialog__header) {
  padding-bottom: 20px;
  border-bottom: 1px solid #ebeef5;
  margin-bottom: 18px;
}
.link-dialog :deep(.el-dialog__footer) {
  border-top: 1px solid #ebeef5;
  padding-top: 20px;
  margin-top: 10px;
}
.dialog-intro {
  display: flex;
  align-items: center;
  gap: 9px;
  color: #689acf;
  background: #ecf5ff;
  padding: 12px 14px;
  border-radius: 6px;
  font-size: 12px;
  margin-bottom: 22px;
}
.link-dialog :deep(.el-form-item__label) {
  font-size: 13px;
}
.link-dialog :deep(.el-divider__text) {
  font-size: 12px;
  color: #909399;
}
.link-dialog :deep(.el-divider) {
  margin: 30px 0 25px;
}
.link-dialog small {
  margin-top: 7px;
  font-size: 11px;
}
.meta-binding-notice {
  margin-bottom: 18px;
}
.meta-parameter-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 10px;
}
.meta-parameter-actions small {
  margin-top: 0;
}
@media (max-width: 1200px) {
  .table-toolbar {
    flex-wrap: wrap;
  }
}
@media (max-width: 1200px) {
  .table-search {
    max-width: 100%;
  }
}
@media (max-width: 800px) {
  .table-toolbar :deep(.el-radio-button__inner) {
    font-size: 11px;
    padding: 8px;
  }
}
@media (max-width: 800px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
