<template>
  <section>
    <PageHeader
      title="TikTok Pixel"
      context="Events API 回传目标"
      description="为小说投放链接集中配置 Pixel Code、凭证和测试事件代码。"
    >
      <el-button :icon="Refresh" :loading="busy" @click="load">刷新</el-button>
      <el-button type="primary" :icon="Plus" @click="open()"
        >添加 Pixel</el-button
      >
    </PageHeader>
    <el-alert
      class="notice"
      type="info"
      :closable="false"
      title="Pixel Code 和所属凭证保存后不可修改；发送测试事件只验证 TikTok Events API 是否接收。"
    />
    <el-alert
      v-if="error"
      class="notice"
      type="error"
      :closable="false"
      :title="error"
    />
    <section class="panel" v-loading="busy">
      <el-form inline>
        <el-form-item label="凭证">
          <el-select
            v-model="connectionID"
            clearable
            placeholder="全部凭证"
            style="width: 240px"
            @change="load"
          >
            <el-option
              v-for="connection in connections"
              :key="connection.id"
              :value="connection.id"
              :label="connection.name"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <el-table :data="pixels" empty-text="尚未配置 TikTok Pixel">
        <el-table-column label="Pixel" min-width="230">
          <template #default="{ row }">
            <b>{{ row.name }}</b>
            <div class="muted">{{ row.pixel_code }}</div>
          </template>
        </el-table-column>
        <el-table-column label="凭证" min-width="180">
          <template #default="{ row }">
            {{ connectionName(row.connection_id) }}
          </template>
        </el-table-column>
        <el-table-column label="测试配置" min-width="190">
          <template #default="{ row }">
            {{ row.test_event_code ? "Test Event Code 已配置" : "未配置" }}
            <div v-if="row.last_error" class="muted error-line">
              {{ row.last_error }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'">
              {{ row.enabled ? "启用" : "停用" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="open(row)">编辑</el-button>
            <el-button
              link
              :disabled="!row.test_event_code"
              :loading="testing === row.id"
              @click="runTest(row)"
              >发送测试事件</el-button
            >
            <el-button
              link
              :type="row.enabled ? 'danger' : 'success'"
              @click="toggle(row)"
              >{{ row.enabled ? "停用" : "启用" }}</el-button
            >
            <el-button
              link
              type="danger"
              :loading="deleting === row.id"
              @click="remove(row)"
              >删除</el-button
            >
          </template>
        </el-table-column>
      </el-table>
    </section>

    <el-dialog
      v-model="dialog"
      :title="editing ? '编辑 TikTok Pixel' : '添加 TikTok Pixel'"
      width="min(580px, 94vw)"
      :close-on-click-modal="false"
      @closed="reset"
    >
      <el-form label-position="top" @submit.prevent="save">
        <el-form-item label="所属凭证" required>
          <el-select
            v-model="form.connection_id"
            :disabled="Boolean(editing)"
            style="width: 100%"
            placeholder="选择 TikTok 凭证"
          >
            <el-option
              v-for="connection in connections"
              :key="connection.id"
              :value="connection.id"
              :label="connection.name"
              :disabled="!connection.enabled"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Pixel 名称" required>
          <el-input
            v-model="form.name"
            maxlength="120"
            placeholder="例如：小说站主 Pixel"
          />
        </el-form-item>
        <el-form-item label="Pixel Code" required>
          <el-input
            v-model="form.pixel_code"
            :disabled="Boolean(editing)"
            placeholder="填写 TikTok Pixel Code"
          />
          <small v-if="editing" class="muted">
            Pixel Code 创建后不可修改，需要更换时请新建 Pixel。
          </small>
        </el-form-item>
        <el-form-item label="Test Event Code">
          <el-input
            v-model="form.test_event_code"
            maxlength="120"
            placeholder="可选；发送测试事件前需要填写"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.enabled" active-text="启用事件发送" />
        </el-form-item>
        <el-alert
          v-if="formError"
          type="error"
          :closable="false"
          :title="formError"
        />
      </el-form>
      <template #footer>
        <el-button :disabled="saving" @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save"
          >保存 Pixel</el-button
        >
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus/es/components/message/index";
import { ElMessageBox } from "element-plus/es/components/message-box/index";
import { Plus, Refresh } from "@element-plus/icons-vue";

import PageHeader from "../components/PageHeader.vue";
import {
  deleteTikTokPixel,
  listTikTokConnections,
  listTikTokPixels,
  saveTikTokPixel,
  testTikTokPixel,
} from "../api/tiktok.js";
import { tiktokPixelForm } from "../utils/tiktok.js";

const connections = ref([]);
const pixels = ref([]);
const connectionID = ref(null);
const busy = ref(false);
const error = ref("");
const dialog = ref(false);
const editing = ref(null);
const saving = ref(false);
const testing = ref(null);
const deleting = ref(null);
const formError = ref("");
const form = reactive(tiktokPixelForm());

function connectionName(id) {
  return (
    connections.value.find((item) => Number(item.id) === Number(id))?.name ||
    "未知凭证"
  );
}

async function load() {
  busy.value = true;
  error.value = "";
  try {
    const [connectionResult, pixelResult] = await Promise.all([
      listTikTokConnections(),
      listTikTokPixels(connectionID.value),
    ]);
    connections.value = connectionResult;
    pixels.value = pixelResult;
  } catch (loadError) {
    error.value = loadError.message;
  } finally {
    busy.value = false;
  }
}

function open(row) {
  editing.value = row || null;
  Object.assign(form, tiktokPixelForm(row));
  formError.value = "";
  dialog.value = true;
}

function reset() {
  editing.value = null;
  Object.assign(form, tiktokPixelForm());
  formError.value = "";
}

async function save() {
  if (!form.connection_id || !form.name.trim() || !form.pixel_code.trim()) {
    formError.value = "请选择凭证并填写 Pixel 名称和 Pixel Code";
    return;
  }
  saving.value = true;
  formError.value = "";
  try {
    await saveTikTokPixel(editing.value?.id, form);
    dialog.value = false;
    ElMessage.success("TikTok Pixel 已保存");
    await load();
  } catch (saveError) {
    formError.value = saveError.message;
  } finally {
    saving.value = false;
  }
}

async function toggle(row) {
  try {
    await saveTikTokPixel(row.id, { ...row, enabled: !row.enabled });
    ElMessage.success(
      row.enabled ? "TikTok Pixel 已停用" : "TikTok Pixel 已启用",
    );
    await load();
  } catch (toggleError) {
    ElMessage.error(toggleError.message);
  }
}

async function runTest(row) {
  testing.value = row.id;
  try {
    await testTikTokPixel(row.id);
    // API accepted 只说明 TikTok 接口收到了测试事件，不声明广告归因结果。
    ElMessage.success(
      "TikTok Events API 已接收测试事件，请到 Events Manager 核对",
    );
    await load();
  } catch (testError) {
    ElMessage.error(testError.message);
  } finally {
    testing.value = null;
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(
      `确定删除 TikTok Pixel“${row.name}”吗？存在投放链接或历史事件时请改为停用。`,
      "删除 TikTok Pixel",
      { type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" },
    );
  } catch {
    return;
  }
  deleting.value = row.id;
  try {
    await deleteTikTokPixel(row.id);
    ElMessage.success("TikTok Pixel 已删除");
    await load();
  } catch (deleteError) {
    ElMessage.error(deleteError.message);
  } finally {
    deleting.value = null;
  }
}

onMounted(load);
</script>

<style scoped>
.error-line {
  max-width: 360px;
  overflow-wrap: anywhere;
}
</style>
