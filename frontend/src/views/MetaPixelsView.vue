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
        ><el-form-item label="账户"
          ><el-select
            v-model="connectionID"
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
              手动：{{ row.manual_enabled ? row.manual_event_name : "关闭" }}
            </div>
            <!-- 自动跳转沿用咨询回传开关，但使用独立事件名，避免与手动点击混算。 -->
            <div class="muted">
              自动：{{ row.manual_enabled ? "WhatsAppAutoRedirect" : "关闭" }}
            </div></template
          ></el-table-column
        ><el-table-column label="凭证" min-width="170"
          ><template #default="{ row }"
            >{{ row.has_capi_token ? "•••• 已保存" : "未配置" }}
            <div class="muted">
              {{ credentialStatus(row.credential_status).label }}
            </div></template
          ></el-table-column
        ><el-table-column label="操作" width="130"
          ><template #default="{ row }"
            ><el-button link type="primary" @click="open(row)">编辑</el-button
            ><el-button link @click="openTest(row)">测试</el-button></template
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
        ><el-form-item label="CAPI 令牌" :required="!editing"
          ><el-input
            v-model="form.capi_token"
            type="password"
            autocomplete="new-password"
            :placeholder="
              editing?.has_capi_token ? '已保存，留空保留' : '填写回传令牌'
            " />
          <small class="muted"
            >令牌只保存在服务端，并仅用于向当前 Pixel 回传事件。</small
          ><el-switch
            v-if="editing?.has_capi_token"
            v-model="form.clear_capi_token"
            aria-label="清除令牌"
            active-text="清除令牌" /></el-form-item
        ><el-form-item label="人工记录到期日"
          ><el-date-picker
            v-model="form.token_expires_at"
            value-format="YYYY-MM-DD"
            clearable
          /><small class="muted"
            >仅用于提醒，不代表 Meta 平台保证。按 UTC 当日 23:59:59
            保存；清空后保存会移除提醒日期。</small
          ></el-form-item
        ><el-form-item label="运行规则"
          ><el-switch
            v-model="form.enabled"
            active-text="启用目标"
            aria-label="启用目标" /><el-switch
            v-model="form.pageview_enabled"
            aria-label="回传 PageView"
            active-text="回传 PageView" /><el-switch
            v-model="form.manual_enabled"
            aria-label="回传咨询事件"
            active-text="回传手动与自动咨询" /></el-form-item
        ><el-form-item label="手动事件名"
          ><el-select v-model="form.manual_event_name"
            ><el-option value="WhatsAppConsultClick" /><el-option
              value="Contact" /></el-select></el-form-item></el-form
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
import { Refresh, Plus } from "@element-plus/icons-vue";
import PageHeader from "../components/PageHeader.vue";
import {
  listConnections,
  listPixels,
  savePixel,
  sendPixelTestEvent,
} from "../api/meta";
import { pixelForm, pixelPayload, credentialStatus } from "../utils/meta";
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
  testing = ref(false);
const form = reactive(pixelForm()),
  testForm = reactive({
    test_event_code: "",
    event_name: "WhatsAppConsultClick",
  }),
  // 测试事件列表与后端允许的真实事件名称保持一致。
  eventNames = [
    "PageView",
    "WhatsAppConsultClick",
    "WhatsAppAutoRedirect",
    "Contact",
  ];
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
function open(row) {
  editing.value = row || null;
  Object.assign(form, pixelForm(row));
  dialog.value = true;
}
async function save() {
  // A new target is enabled by default, so require its own token before the
  // request reaches the backend and can appear ready without credentials.
  if (!editing.value && !form.capi_token.trim()) {
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
.el-switch {
  margin-right: 18px;
}
.muted {
  margin-top: 6px;
}
</style>
