<template>
  <section>
    <PageHeader
      title="TikTok 凭证"
      context="Events API 凭证管理"
      description="集中保存 TikTok Access Token，供一个或多个 Pixel 的服务端事件回传使用。"
    >
      <el-button :icon="Refresh" :loading="busy" @click="load">刷新</el-button>
      <el-button type="primary" :icon="Plus" @click="open()"
        >添加凭证</el-button
      >
    </PageHeader>
    <el-alert
      class="notice"
      type="info"
      :closable="false"
      title="Access Token 只在服务端加密保存，列表和浏览器不会读取已保存的明文。"
    />
    <el-alert
      v-if="error"
      class="notice"
      type="error"
      :closable="false"
      :title="error"
    />
    <section class="panel" v-loading="busy">
      <div class="panel-heading">
        <h2>
          TikTok 凭证 <span class="count">{{ connections.length }}</span>
        </h2>
        <span class="muted">一个凭证可以管理多个 Pixel</span>
      </div>
      <el-empty
        v-if="!busy && !connections.length"
        description="尚未配置 TikTok 凭证"
      >
        <el-button type="primary" @click="open()">添加第一个凭证</el-button>
      </el-empty>
      <el-table v-else :data="connections" empty-text="暂无 TikTok 凭证">
        <el-table-column label="名称" min-width="200">
          <template #default="{ row }">
            <b>{{ row.name }}</b>
            <div class="muted">
              {{
                row.has_access_token ? "Access Token 已保存" : "未保存 Token"
              }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="凭证状态" min-width="180">
          <template #default="{ row }">
            <el-tag :type="credentialType(row.credential_status)">
              {{ credentialLabel(row.credential_status) }}
            </el-tag>
            <div v-if="row.last_error" class="muted error-line">
              {{ row.last_error }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="运行状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'">
              {{ row.enabled ? "启用" : "停用" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="open(row)">编辑</el-button>
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
      :title="editing ? '编辑 TikTok 凭证' : '添加 TikTok 凭证'"
      width="min(560px, 94vw)"
      :close-on-click-modal="false"
      @closed="reset"
    >
      <el-form label-position="top" @submit.prevent="save">
        <el-form-item label="凭证名称" required>
          <el-input
            v-model="form.name"
            maxlength="120"
            placeholder="例如：TikTok 全球投放"
          />
        </el-form-item>
        <el-form-item label="Access Token" :required="!editing">
          <el-input
            v-model="form.access_token"
            type="password"
            show-password
            autocomplete="new-password"
            :placeholder="
              editing ? '留空保留现有 Access Token' : '填写 Access Token'
            "
          />
          <small class="muted">
            {{ editing ? "留空不会覆盖现有凭证。" : "Token 将加密后保存。" }}
          </small>
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
          >保存凭证</el-button
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
  deleteTikTokConnection,
  listTikTokConnections,
  saveTikTokConnection,
} from "../api/tiktok.js";
import { tiktokConnectionForm } from "../utils/tiktok.js";

const connections = ref([]);
const busy = ref(false);
const error = ref("");
const dialog = ref(false);
const editing = ref(null);
const saving = ref(false);
const deleting = ref(null);
const formError = ref("");
const form = reactive(tiktokConnectionForm());

function credentialLabel(status) {
  return (
    { valid: "有效", invalid: "无效", unverified: "未验证" }[status] || "未验证"
  );
}

function credentialType(status) {
  if (status === "valid") return "success";
  if (status === "invalid") return "danger";
  return "info";
}

async function load() {
  busy.value = true;
  error.value = "";
  try {
    connections.value = await listTikTokConnections();
  } catch (loadError) {
    error.value = loadError.message;
  } finally {
    busy.value = false;
  }
}

function open(row) {
  editing.value = row || null;
  Object.assign(form, tiktokConnectionForm(row));
  formError.value = "";
  dialog.value = true;
}

function reset() {
  Object.assign(form, tiktokConnectionForm());
  editing.value = null;
  formError.value = "";
}

async function save() {
  if (!form.name.trim() || (!editing.value && !form.access_token.trim())) {
    formError.value = "请填写凭证名称和 Access Token";
    return;
  }
  saving.value = true;
  formError.value = "";
  try {
    await saveTikTokConnection(editing.value?.id, form);
    dialog.value = false;
    ElMessage.success("TikTok 凭证已保存");
    await load();
  } catch (saveError) {
    formError.value = saveError.message;
  } finally {
    saving.value = false;
  }
}

async function remove(row) {
  try {
    // 服务端会拒绝删除仍有关联 Pixel 或历史事件的凭证。
    await ElMessageBox.confirm(
      `确定删除 TikTok 凭证“${row.name}”吗？存在 Pixel 或历史事件时请改为停用。`,
      "删除 TikTok 凭证",
      { type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" },
    );
  } catch {
    return;
  }
  deleting.value = row.id;
  try {
    await deleteTikTokConnection(row.id);
    ElMessage.success("TikTok 凭证已删除");
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
  max-width: 420px;
  margin-top: 5px;
  overflow-wrap: anywhere;
}
</style>
