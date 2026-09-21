<template>
  <section>
    <PageHeader
      title="Meta Pixel"
      context="回传目标管理"
      description="一个广告账户可配置多个 Pixel；Pixel ID 与 CAPI Token 决定回传目标。"
      ><el-button :icon="Refresh" :loading="busy" @click="load">刷新</el-button
      ><el-button type="primary" :icon="Plus" @click="open()"
        >添加 Pixel</el-button
      ></PageHeader
    ><el-alert
      v-if="error"
      class="notice"
      type="error"
      :closable="false"
      :title="error"
    />
    <!-- Pixel 页面包含完整的 CAPI 目标配置，不依赖账户读取权限。 -->
    <el-alert
      class="notice"
      type="info"
      :closable="false"
      title="添加 Pixel ID 与对应 CAPI Token，保存后即可按下方规则接收事件。"
    />
    <section class="panel" v-loading="busy">
      <el-form inline
        ><el-form-item label="账户" class="account-filter"
          ><el-select
            v-model="connectionID"
            class="account-filter__select"
            clearable
            placeholder="全部账户"
            @change="load"
            ><el-option
              v-for="c in connections"
              :key="c.id"
              :value="c.id"
              :label="
                c.name + ' · ' + c.account_id
              " /></el-select></el-form-item></el-form
      ><el-table :data="pixels" empty-text="尚未配置 Pixel"
        ><el-table-column label="Pixel" min-width="210"
          ><template #default="{ row }"
            ><b>{{ row.name }}</b>
            <div class="muted">{{ row.pixel_id }}</div></template
          ></el-table-column
        ><el-table-column label="规则" min-width="230"
          ><template #default="{ row }"
            ><el-tag :type="row.enabled ? 'success' : 'info'">{{
              row.enabled ? "启用" : "停用"
            }}</el-tag>
            <el-tag :type="row.pageview_enabled ? 'success' : 'info'"
              >PageView {{ row.pageview_enabled ? "开" : "关" }}</el-tag
            >
            <div class="muted">
              手动：{{ row.manual_enabled ? META_CONSULT_EVENT : "关闭" }}
            </div>
            <!-- 两类动作共享标准事件名，但仍由独立开关控制。 -->
            <div class="muted">
              自动：{{ row.auto_enabled ? META_CONSULT_EVENT : "关闭" }}
            </div></template
          ></el-table-column
        ><el-table-column label="凭证" min-width="170"
          ><template #default="{ row }"
            >{{ row.has_capi_token ? "•••• 已保存" : "未配置" }}
            <div class="muted">
              {{ credentialStatus(row.credential_status).label }}
            </div></template
          ></el-table-column
        ><el-table-column label="操作" width="235"
          ><template #default="{ row }"
            ><el-button link type="primary" @click="open(row)">编辑</el-button
            ><el-button link @click="openTest(row)">测试</el-button
            ><el-button
              link
              :type="row.enabled ? 'danger' : 'success'"
              :loading="toggling === row.id"
              @click="togglePixel(row)"
              >{{ row.enabled ? "停用" : "启用" }}</el-button
            ><el-button
              link
              type="danger"
              :loading="deleting === row.id"
              @click="removePixel(row)"
              >删除</el-button
            ></template
          ></el-table-column
        ></el-table
      >
    </section>
    <el-dialog
      v-model="dialog"
      :title="editing ? '编辑 Pixel' : '添加 Pixel'"
      width="560px"
      ><el-form label-position="top"
        ><el-form-item label="所属账户" required
          ><el-select
            v-model="form.connection_id"
            :disabled="!!editing"
            style="width: 100%"
            ><el-option
              v-for="c in connections"
              :key="c.id"
              :value="c.id"
              :label="
                c.name + ' · ' + c.account_id
              " /></el-select></el-form-item
        ><el-form-item label="名称" required
          ><el-input v-model="form.name" /></el-form-item
        ><el-form-item label="Pixel ID" required
          ><el-input
            v-model="form.pixel_id"
            :disabled="!!editing" /></el-form-item
        ><el-form-item label="CAPI 令牌" required
          ><el-input
            v-model="form.capi_token"
            type="password"
            show-password
            autocomplete="new-password"
            :disabled="credentialLoading"
            :placeholder="
              credentialLoading
                ? '正在读取已保存令牌'
                : editing?.has_capi_token
                  ? '已保存令牌'
                  : '填写回传令牌'
            "
          />
          <small class="muted"
            >令牌只保存在服务端，并仅用于向当前 Pixel 回传事件。</small
          ><!-- Operators pause delivery from the list and replace credentials directly in this field. --> </el-form-item
        ><el-form-item label="运行规则"
          ><!-- New targets are enabled by default; the list provides the explicit pause action. -->
          <el-switch
            v-model="form.pageview_enabled"
            aria-label="回传 PageView"
            active-text="回传 PageView" /><el-switch
            v-model="form.manual_enabled"
            aria-label="回传手动咨询"
            active-text="回传手动咨询 AddToCart" /><el-switch
            v-model="form.auto_enabled"
            aria-label="回传自动跳转"
            active-text="回传自动跳转 AddToCart" /></el-form-item></el-form
      ><template #footer
        ><el-button @click="dialog = false">取消</el-button
        ><el-button type="primary" :loading="saving" @click="save"
          >保存</el-button
        ></template
      ></el-dialog
    >
    <el-dialog v-model="testDialog" title="发送 Pixel 测试事件" width="480px"
      ><el-form label-position="top"
        ><el-form-item label="测试事件代码"
          ><el-input v-model="testForm.test_event_code" /></el-form-item
        ><el-form-item label="事件名"
          ><el-select v-model="testForm.event_name"
            ><el-option
              v-for="n in eventNames"
              :key="n"
              :value="n" /></el-select></el-form-item></el-form
      ><template #footer
        ><el-button @click="testDialog = false">取消</el-button
        ><el-button type="primary" :loading="testing" @click="runTest"
          >发送测试</el-button
        ></template
      ></el-dialog
    >
  </section>
</template>
<script setup>
import { ref, reactive, onMounted } from "vue";
import { ElMessage } from "element-plus/es/components/message/index";
import { ElMessageBox } from "element-plus/es/components/message-box/index";
import { Refresh, Plus } from "@element-plus/icons-vue";
import PageHeader from "../components/PageHeader.vue";
import {
  listConnections,
  listPixels,
  getPixelCredential,
  savePixel,
  deletePixel,
  sendPixelTestEvent,
} from "../api/meta";
import {
  pixelForm,
  pixelPayload,
  pixelTogglePayload,
  credentialStatus,
  META_CONSULT_EVENT,
} from "../utils/meta";
const connections = ref([]),
  pixels = ref([]),
  connectionID = ref(null),
  busy = ref(false),
  error = ref(""),
  dialog = ref(false),
  editing = ref(null),
  saving = ref(false),
  testDialog = ref(false),
  testPixel = ref(null),
  testing = ref(false),
  credentialLoading = ref(false),
  toggling = ref(null),
  deleting = ref(null);
const form = reactive(pixelForm()),
  testForm = reactive({
    test_event_code: "",
    event_name: META_CONSULT_EVENT,
  }),
  // 测试事件列表与后端允许的真实事件名称保持一致。
  eventNames = ["PageView", META_CONSULT_EVENT];
async function load() {
  busy.value = true;
  error.value = "";
  try {
    pixels.value = await listPixels(connectionID.value);
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function open(row) {
  editing.value = row || null;
  Object.assign(form, pixelForm(row));
  dialog.value = true;
  if (!row?.has_capi_token) return;
  credentialLoading.value = true;
  try {
    // Fetch one credential only on explicit edit; Pixel lists never receive plaintext tokens.
    const credential = await getPixelCredential(row.id);
    Object.assign(form, pixelForm(row, credential.capi_token || ""));
  } catch (e) {
    ElMessage.error(`读取 CAPI Token 失败：${e.message}`);
  } finally {
    credentialLoading.value = false;
  }
}
async function save() {
  // Editing uses the credential fetched for this Pixel. Block saving while it
  // is unavailable so every saved target always retains or replaces a token.
  if (credentialLoading.value) {
    ElMessage.warning("正在读取当前 Pixel 的 CAPI Token，请稍候");
    return;
  }
  if (!form.capi_token.trim()) {
    ElMessage.warning("请填写当前 Pixel 的 CAPI Token");
    return;
  }
  saving.value = true;
  try {
    await savePixel(editing.value?.id, pixelPayload(form, !!editing.value));
    dialog.value = false;
    ElMessage.success("Pixel 已保存");
    await load();
  } catch (e) {
    ElMessage.error(e.message);
  } finally {
    saving.value = false;
  }
}
async function togglePixel(row) {
  toggling.value = row.id;
  try {
    // PATCH overlays this single state on the stored Pixel without clearing its token or rules.
    await savePixel(row.id, pixelTogglePayload(row));
    ElMessage.success(row.enabled ? "Pixel 已停用" : "Pixel 已启用");
    await load();
  } catch (e) {
    ElMessage.error(e.message);
  } finally {
    toggling.value = null;
  }
}
async function removePixel(row) {
  try {
    // The confirmation states both irreversible credential deletion and the
    // backend reference guard before the administrator makes the final choice.
    await ElMessageBox.confirm(
      `确定删除 Pixel“${row.name}”吗？对应 CAPI Token 会永久删除。已被短链接或历史记录使用时，系统会拒绝删除。`,
      "删除 Meta Pixel",
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
    await deletePixel(row.id);
    ElMessage.success("Meta Pixel 已删除");
    await load();
  } catch (e) {
    ElMessage.error(e.message);
  } finally {
    deleting.value = null;
  }
}
function openTest(row) {
  testPixel.value = row;
  testDialog.value = true;
}
async function runTest() {
  testing.value = true;
  try {
    await sendPixelTestEvent(testPixel.value.id, { ...testForm });
    testDialog.value = false;
    ElMessage.success("测试事件已加入队列");
  } catch (e) {
    ElMessage.error(e.message);
  } finally {
    testing.value = false;
  }
}
onMounted(async () => {
  try {
    connections.value = await listConnections();
  } catch (e) {
    error.value = e.message;
  }
  await load();
});
</script>
<style scoped>
/* Keep the account name and long Meta account ID readable in the filter. */
.account-filter__select {
  width: 360px;
  max-width: calc(100vw - 150px);
}
.el-switch {
  margin-right: 18px;
}
.muted {
  margin-top: 6px;
}
@media (max-width: 600px) {
  /* Let the filter consume the available row on narrow admin screens. */
  .account-filter {
    width: 100%;
    margin-right: 0;
  }
  .account-filter :deep(.el-form-item__content) {
    flex: 1;
    min-width: 0;
  }
  .account-filter__select {
    width: 100%;
    max-width: none;
  }
}
</style>
