<template>
  <section>
    <PageHeader
      title="Meta 帐号"
      context="广告账户分组"
      description="账户用于归类多个 Pixel；Pixel ID、CAPI Token 和回传规则统一在 Meta Pixel 页面管理。"
    >
      <el-button :icon="Refresh" :loading="busy" @click="load">刷新</el-button>
      <el-button type="primary" :icon="Plus" @click="open()"
        >添加账户</el-button
      >
    </PageHeader>
    <el-alert
      v-if="error"
      class="notice"
      type="error"
      :closable="false"
      :title="error"
    />
    <el-alert
      class="notice"
      type="info"
      :closable="false"
      title="CAPI 回传不读取广告报告，也不需要 ads_read 或 ads_management 权限。"
    />
    <section class="panel" v-loading="busy">
      <div class="panel-heading">
        <h2>
          广告账户 <span class="count">{{ connections.length }}</span>
        </h2>
        <span class="muted">一个账户可以添加多个 Pixel</span>
      </div>
      <el-empty
        v-if="!busy && !connections.length"
        description="尚未配置 Meta 广告账户"
      >
        <el-button type="primary" @click="open()">添加第一个账户</el-button>
      </el-empty>
      <el-table v-else :data="connections" empty-text="暂无账户">
        <el-table-column label="名称" min-width="200">
          <template #default="{ row }"
            ><b>{{ row.name }}</b></template
          >
        </el-table-column>
        <el-table-column
          prop="account_id"
          label="广告账户 ID"
          min-width="210"
        />
        <el-table-column
          prop="api_version"
          label="Graph API 版本"
          width="150"
        />
        <el-table-column label="操作" width="235" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="open(row)">编辑</el-button>
            <el-button
              link
              @click="
                router.push({
                  name: 'meta-events',
                  query: { connection_id: String(row.id) },
                })
              "
              >事件记录</el-button
            >
            <el-button
              link
              type="danger"
              :loading="deleting === row.id"
              @click="removeConnection(row)"
              >删除</el-button
            >
          </template>
        </el-table-column>
      </el-table>
    </section>
    <p class="footnote">
      添加账户后，请到 Meta Pixel 页面添加 Pixel ID 与 CAPI
      Token，再把短链接绑定到具体 Pixel。
    </p>
    <el-dialog
      v-model="dialog"
      :title="editing ? '编辑 Meta 帐号' : '添加 Meta 帐号'"
      width="min(620px, 94vw)"
      :close-on-click-modal="false"
      :close-on-press-escape="!saving"
      :show-close="!saving"
      destroy-on-close
      @closed="resetForm"
    >
      <el-form label-position="top" @submit.prevent="save">
        <el-form-item label="账户名称" required>
          <el-input
            v-model="form.name"
            maxlength="120"
            placeholder="例如：美国主账户"
          />
        </el-form-item>
        <el-form-item label="广告账户 ID" required>
          <el-input
            v-model="form.account_id"
            :disabled="!!editing"
            placeholder="只填写数字，不包含 act_"
          />
        </el-form-item>
        <el-form-item label="Graph API 版本" required>
          <!-- The Graph version is application-owned so every account uses the
          same tested CAPI contract. -->
          <el-input :model-value="GRAPH_API_VERSION" disabled />
          <small>由系统统一维护；账户 ID 保存后不可更换。</small>
        </el-form-item>
        <el-alert
          v-if="formError"
          :title="formError"
          type="error"
          :closable="false"
        />
      </el-form>
      <template #footer>
        <el-button :disabled="saving" @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save"
          >保存账户</el-button
        >
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus/es/components/message/index";
import { ElMessageBox } from "element-plus/es/components/message-box/index";
import { Plus, Refresh } from "@element-plus/icons-vue";

import PageHeader from "../components/PageHeader.vue";
import { deleteConnection, listConnections, saveConnection } from "../api/meta";
import {
  GRAPH_API_VERSION,
  connectionForm,
  connectionPayload,
} from "../utils/meta";

const router = useRouter();
const connections = ref([]);
const busy = ref(false);
const error = ref("");
const dialog = ref(false);
const editing = ref(null);
const saving = ref(false);
const deleting = ref(null);
const formError = ref("");
const form = reactive(connectionForm());

async function load() {
  busy.value = true;
  error.value = "";
  try {
    connections.value = await listConnections();
  } catch (loadError) {
    error.value = loadError.message;
  } finally {
    busy.value = false;
  }
}

function open(row) {
  editing.value = row || null;
  Object.assign(form, connectionForm(row));
  formError.value = "";
  dialog.value = true;
}

function resetForm() {
  // Reset every visible field when the dialog closes so old account metadata
  // cannot leak into the next create operation.
  Object.assign(form, connectionForm());
  editing.value = null;
  formError.value = "";
}

async function save() {
  if (saving.value) return;
  formError.value = "";
  if (!form.name.trim() || !/^\d{1,32}$/.test(form.account_id.trim())) {
    formError.value = "请填写账户名称和纯数字广告账户 ID";
    return;
  }
  saving.value = true;
  try {
    await saveConnection(
      editing.value?.id,
      connectionPayload(form, !!editing.value),
    );
    dialog.value = false;
    await load();
    ElMessage.success("Meta 帐号已保存");
  } catch (saveError) {
    formError.value = saveError.message;
  } finally {
    saving.value = false;
  }
}

async function removeConnection(row) {
  try {
    // Accounts own Pixel groups, so the operator must explicitly clear those
    // dependencies before this confirmation can result in a deletion.
    await ElMessageBox.confirm(
      `确定删除 Meta 帐号“${row.name}”吗？帐号下仍有 Pixel、短链接或历史数据时，系统会拒绝删除。`,
      "删除 Meta 帐号",
      {
        type: "warning",
        confirmButtonText: "删除",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
      },
    );
  } catch {
    return;
  }
  deleting.value = row.id;
  try {
    await deleteConnection(row.id);
    ElMessage.success("Meta 帐号已删除");
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
.count {
  color: #909399;
  font-size: 14px;
  font-weight: 500;
}
</style>
