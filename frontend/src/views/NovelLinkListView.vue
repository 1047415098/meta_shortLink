<template>
  <section>
    <PageHeader
      :title="novel ? `《${novel.title}》投放链接` : '小说投放链接'"
      context="免费小说项目 / 独立归因"
      description="同一本小说可创建多条链接；每条链接单独统计访问人数与可见停留时长。"
    >
      <el-button @click="router.push({ name: 'novels' })">返回小说</el-button
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
      title="建议每位投手或每次投放创建一条独立链接，并用链接名称标记归属。链接产生访问后，绑定小说将被锁定。"
    />
    <section class="panel" v-loading="loading">
      <el-table :data="links" empty-text="这本小说还没有投放链接">
        <el-table-column label="名称 / 地址" min-width="280"
          ><template #default="{ row }"
            ><b>{{ row.name }}</b>
            <div class="url">{{ row.public_url }}</div></template
          ></el-table-column
        >
        <el-table-column label="访问" prop="visit_count" width="90" />
        <el-table-column label="广告 ID" prop="ad_id" min-width="140"
          ><template #default="{ row }">{{
            row.ad_id || "动态读取"
          }}</template></el-table-column
        >
        <el-table-column label="状态" width="90"
          ><template #default="{ row }"
            ><el-tag :type="row.enabled ? 'success' : 'info'">{{
              row.enabled ? "使用中" : "已停用"
            }}</el-tag></template
          ></el-table-column
        >
        <el-table-column
          label="操作"
          min-width="280"
          :fixed="isMobile ? false : 'right'"
          ><template #default="{ row }">
            <el-button link type="primary" @click="copy(row)">复制</el-button
            ><el-button
              link
              type="primary"
              @click="
                router.push({
                  name: 'novel-link-stats',
                  params: { id: row.id },
                })
              "
              >统计</el-button
            ><el-button link @click="open(row)">编辑</el-button
            ><el-button
              link
              :type="row.enabled ? 'danger' : 'success'"
              @click="toggle(row)"
              >{{ row.enabled ? "停用" : "启用" }}</el-button
            ><el-button
              link
              type="danger"
              :disabled="Boolean(row.first_visited_at)"
              @click="remove(row)"
              >删除</el-button
            >
          </template></el-table-column
        >
      </el-table>
    </section>
    <el-dialog
      v-model="dialog"
      :title="editing ? '编辑投放链接' : '创建投放链接'"
      width="620px"
      class="novel-link-dialog"
      :close-on-click-modal="false"
    >
      <el-form label-position="top" @submit.prevent="save">
        <div class="form-grid">
          <el-form-item label="链接名称" required
            ><el-input
              v-model="form.name"
              maxlength="120"
              placeholder="例如：投手 A · 广告组 1" /></el-form-item
          ><el-form-item v-if="!editing" label="自定义短码（留空自动生成）"
            ><el-input v-model="form.code" placeholder="wife-a"
          /></el-form-item>
        </div>
        <el-form-item label="绑定小说" required
          ><el-select
            v-model="form.novel_id"
            style="width: 100%"
            :disabled="Boolean(editing?.first_visited_at)"
            ><el-option
              v-for="item in novels"
              :key="item.id"
              :label="item.title"
              :value="item.id" /></el-select
          ><small v-if="editing?.first_visited_at" class="muted"
            >已有访问，小说绑定不可再修改；需要换小说时请新建链接。</small
          ></el-form-item
        >
        <el-form-item label="Meta Pixel" required
          ><el-select
            v-model="form.meta_pixel_id"
            style="width: 100%"
            placeholder="选择回传 Pixel"
            @change="selectPixel"
            ><el-option
              v-for="pixel in pixels"
              :key="pixel.id"
              :value="pixel.id"
              :label="pixelLabel(pixel)"
              :disabled="!pixelSelectable(pixel)" /></el-select
        ></el-form-item>
        <el-form-item label="停留时长回传（秒）"
          ><el-input-number
            v-model="form.time_spent_threshold"
            :min="0"
            :max="3600"
            :step="1"
          /><small class="muted"
            >达到设置的前台可见时长后才回传一次 Meta TimeSpent；0 表示关闭此回传。</small
          ></el-form-item
        >
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
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Plus, Refresh } from "@element-plus/icons-vue";
import PageHeader from "../components/PageHeader.vue";
import { listNovels } from "../api/novels.js";
import {
  createNovelLink,
  deleteNovelLink,
  listNovelLinks,
  updateNovelLink,
} from "../api/novelLinks.js";
import { listConnections, listPixels } from "../api/meta.js";
import { connectionIDForPixel, pixelSelectable } from "../utils/meta.js";
const route = useRoute(),
  router = useRouter(),
  loading = ref(false),
  saving = ref(false),
  dialog = ref(false),
  editing = ref(null),
  links = ref([]),
  novels = ref([]),
  connections = ref([]),
  pixels = ref([]);
const selectedNovelID = computed(() => Number(route.params.id)),
  novel = computed(() =>
    novels.value.find((item) => item.id === selectedNovelID.value),
  );
// Fixed table columns obscure the first column on narrow screens, so follow the breakpoint live.
const mobileQuery = window.matchMedia("(max-width: 700px)"),
  isMobile = ref(mobileQuery.matches),
  syncMobile = (event) => (isMobile.value = event.matches);
const blank = () => ({
  name: "",
  code: "",
  novel_id: selectedNovelID.value,
  enabled: true,
  channel: "facebook",
  campaign_id: "",
  adset_id: "",
  ad_id: "",
  meta_connection_id: null,
  meta_pixel_id: null,
  attribution_mode: "dynamic",
  // 新投放默认以 10 秒前台可见时间作为 Meta 回传门槛。
  time_spent_threshold: 10,
});
const form = reactive(blank());
function open(row) {
  editing.value = row || null;
  Object.assign(form, blank(), row || {});
  dialog.value = true;
}
function selectPixel(id) {
  form.meta_connection_id = connectionIDForPixel(pixels.value, id);
}
function pixelLabel(pixel) {
  const account = connections.value.find(
    (item) => Number(item.id) === Number(pixel.connection_id),
  );
  return `${account?.name || "未知账户"} · ${pixel.name} · ${pixel.pixel_id}`;
}
async function load() {
  loading.value = true;
  try {
    const [linkResult, novelResult, accountResult, pixelResult] =
      await Promise.all([
        listNovelLinks(selectedNovelID.value),
        listNovels({ page_size: 100 }),
        listConnections(),
        listPixels(),
      ]);
    links.value = linkResult.items;
    novels.value = novelResult.items;
    connections.value = accountResult;
    pixels.value = pixelResult;
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    loading.value = false;
  }
}
async function save() {
  if (!form.name.trim() || !form.novel_id || !form.meta_pixel_id) {
    ElMessage.warning("请填写链接名称、绑定小说并选择 Meta Pixel");
    return;
  }
  selectPixel(form.meta_pixel_id);
  saving.value = true;
  try {
    if (editing.value) await updateNovelLink(editing.value.id, form);
    else await createNovelLink(form);
    dialog.value = false;
    ElMessage.success("投放链接已保存");
    await load();
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    saving.value = false;
  }
}
async function toggle(row) {
  try {
    await updateNovelLink(row.id, { ...row, enabled: !row.enabled });
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
    await deleteNovelLink(row.id);
    ElMessage.success("链接已删除");
    await load();
  } catch (error) {
    if (error !== "cancel" && error !== "close")
      ElMessage.error(error.message || "删除失败");
  }
}
async function copy(row) {
  try {
    await navigator.clipboard.writeText(row.public_url);
    ElMessage.success("投放链接已复制");
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
  font-size: 12px;
  color: #409eff;
  margin-top: 5px;
  overflow-wrap: anywhere;
}
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}
/* 长表单在桌面小窗口和手机上仅滚动正文，始终保留标题和保存操作。 */
:deep(.novel-link-dialog) {
  max-width: calc(100vw - 24px);
}
:deep(.novel-link-dialog .el-dialog__body) {
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
