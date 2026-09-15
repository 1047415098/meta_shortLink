<template>
  <section>
    <PageHeader
      title="Meta 凭证"
      context="安全与审计"
      description="集中查看 Pixel CAPI 回传凭证状态。页面不会展示已保存的令牌。"
      ><el-button :icon="Refresh" :loading="busy" @click="load">刷新</el-button
      ><el-button type="warning" plain @click="rewrap"
        >使用活动密钥重加密</el-button
      ></PageHeader
    ><el-alert
      v-if="error"
      class="notice"
      type="error"
      :closable="false"
      :title="error"
    /><el-alert
      class="notice"
      type="info"
      :closable="false"
      :title="`当前加密密钥：${keyID || '未报告'}。凭证状态以最近一次 Meta 回传结果为准。`"
    />
    <section class="panel" v-loading="busy">
      <el-table :data="items"
        ><el-table-column
          prop="name"
          label="Pixel"
          min-width="180" /><el-table-column label="类型" width="120"
          ><template #default>Pixel 回传</template></el-table-column
        ><el-table-column label="编号" min-width="180"
          ><template #default="{ row }"
            ><div>账户 {{ row.account_id || "—" }}</div>
            <div class="muted">Pixel {{ row.pixel_id || "—" }}</div></template
          ></el-table-column
        ><el-table-column label="状态" width="140"
          ><template #default="{ row }"
            ><el-tag :type="credentialState(row).type">{{
              credentialState(row).label
            }}</el-tag></template
          ></el-table-column
        ><el-table-column label="最近验证" min-width="190"
          ><template #default="{ row }"
            ><!-- Token 不维护人工到期日，只显示最近一次实际回传验证时间。 -->
            <div>{{ timestamp(row.validated_at) }}</div></template
          ></el-table-column
        ><el-table-column
          prop="last_error"
          label="最近错误"
          min-width="220"
          show-overflow-tooltip
      /></el-table>
    </section>
    <section class="panel">
      <div class="panel-heading">
        <h2>最近审计</h2>
        <span class="muted">最多 100 条，敏感信息已脱敏</span>
      </div>
      <el-table :data="audit"
        ><el-table-column label="时间" width="185"
          ><template #default="{ row }">{{
            timestamp(row.created_at)
          }}</template></el-table-column
        ><el-table-column
          prop="actor"
          label="操作人"
          width="150"
        /><el-table-column
          prop="action"
          label="动作"
          width="180"
        /><el-table-column label="详情" min-width="260"
          ><template #default="{ row }"
            ><span class="audit-detail">{{
              auditDetail(row.detail)
            }}</span></template
          ></el-table-column
        ></el-table
      >
    </section>
  </section>
</template>
<script setup>
import { ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh } from "@element-plus/icons-vue";
import PageHeader from "../components/PageHeader.vue";
import { listCredentials, listMetaAudit, rewrapCredentials } from "../api/meta";
import { timestamp, credentialStatus } from "../utils/meta";
const items = ref([]),
  audit = ref([]),
  keyID = ref(""),
  busy = ref(false),
  error = ref("");
async function load() {
  busy.value = true;
  error.value = "";
  try {
    const [c, a] = await Promise.all([listCredentials(), listMetaAudit()]);
    items.value = c.items || [];
    keyID.value = c.encryption_key_id;
    audit.value = a.items || [];
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function rewrap() {
  try {
    await ElMessageBox.confirm(
      "仅当服务端已配置新的活动密钥时执行。每批最多处理各 500 条 Pixel 凭证与待发送事件，页面会自动重复直到全部处理完毕。继续吗？",
      "确认重加密",
      { type: "warning" },
    );
    let total = 0;
    for (let batch = 0; batch < 100; batch++) {
      const r = await rewrapCredentials();
      total += Number(r.updated) || 0;
      if (!r.updated) break;
      if (batch === 99)
        throw new Error("分批处理次数超过安全上限，请刷新后继续");
    }
    ElMessage.success(`重加密完成，共更新 ${total} 条凭证`);
    await load();
  } catch (e) {
    if (e !== "cancel") ElMessage.error(e.message);
  }
}
function credentialState(row) {
  return row.configured
    ? credentialStatus(row.status)
    : { label: "未配置", type: "info" };
}
function auditDetail(detail) {
  if (detail == null || detail === "") return "—";
  if (typeof detail === "string") {
    try {
      return JSON.stringify(JSON.parse(detail), null, 2);
    } catch {
      return detail;
    }
  }
  return JSON.stringify(detail, null, 2);
}
onMounted(load);
</script>
<style scoped>
.panel + .panel {
  margin-top: 18px;
}
.audit-detail {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
