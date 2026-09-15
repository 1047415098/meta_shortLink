<template>
  <section class="ads-page">
    <PageHeader
      title="导入统计"
      context="CSV 广告花费"
      description="查看 CSV 导入的广告花费与站内点击表现。"
      ><el-button :icon="Refresh" :loading="busy" @click="load()"
        >刷新</el-button
      ><el-button :icon="Download" @click="exportCSV"
        >导出数据</el-button
      ></PageHeader
    ><el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      class="notice"
    /><AnalyticsFilter
      :filters="filters"
      :links="links"
      :busy="busy"
      @query="load()"
      @reset="resetFilters"
    />
    <section v-loading="busy">
      <div class="panel">
        <div class="panel-heading">
          <h2>CSV 花费与访问表现</h2>
          <span class="muted">花费按币种展示，不合并不同币种</span>
        </div>
        <el-table :data="analytics?.ads || []" empty-text="暂无广告访问数据"
          ><el-table-column label="广告 ID" min-width="150"
            ><template #default="{ row }">{{
              row.ad_id || "未绑定广告"
            }}</template></el-table-column
          ><el-table-column prop="total" label="总访问" /><el-table-column
            prop="filtered"
            label="过滤后点击"
          /><el-table-column prop="unique" label="估算访客" /><el-table-column
            label="花费"
            ><template #default="{ row }"
              >{{ row.cost == null ? "—" : Number(row.cost).toFixed(2) }}
              {{ row.currency }}</template
            ></el-table-column
          ><el-table-column label="每次点击成本" min-width="140"
            ><template #default="{ row }"
              >{{ row.cost == null ? "—" : unitCost(row.cost, row.filtered) }}
              {{ row.currency }}</template
            ></el-table-column
          ><el-table-column label="每位访客成本" min-width="140"
            ><template #default="{ row }"
              >{{ row.cost == null ? "—" : unitCost(row.cost, row.unique) }}
              {{ row.currency }}</template
            ></el-table-column
          ></el-table
        >
      </div>
      <div class="panel import-panel">
        <h2>导入广告花费</h2>
        <p class="muted">
          CSV 表头：date,ad_id,amount,currency,time_zone。日期使用
          YYYY-MM-DD，时区与广告账户保持一致。
        </p>
        <p class="muted">示例：2026-09-08,广告ID,25.50,USD,Asia/Shanghai</p>
        <el-upload
          drag
          accept=".csv,text/csv"
          :auto-upload="false"
          :limit="1"
          :on-change="chooseUpload"
          :on-remove="() => (file = null)"
          class="spend-upload"
          ><el-icon class="el-icon--upload"><UploadFilled /></el-icon>
          <div class="el-upload__text">
            将 CSV 文件拖到此处，或 <em>点击选择</em>
          </div>
          <template #tip
            ><div class="el-upload__tip">
              UTF-8 格式 · 最大 2 MB · 最多 10,000 行
            </div></template
          ></el-upload
        ><el-button type="primary" :loading="saving" @click="importSpend"
          >导入 CSV</el-button
        >
      </div>
      <div class="footnote">
        未导入完整日期花费、时区不匹配或筛选单链接时，不计算成本；零花费日期也需导入。花费仅用于点击成本分析，不代表咨询、成交或投资回报。
      </div>
    </section>
  </section>
</template>

<script setup>
import { ref } from "vue";

import { ElMessage } from "element-plus/es/components/message/index";
import PageHeader from "../components/PageHeader.vue";
import { Refresh, Download, UploadFilled } from "@element-plus/icons-vue";

import AnalyticsFilter from "../components/AnalyticsFilter.vue";
import { useReport } from "../composables/useReport";
import { getAnalytics } from "../api/analytics";
import { unitCost } from "../utils";

import { uploadSpend } from "../api/adspend";
const {
  filters,
  data: analytics,
  links,
  busy,
  error,
  load,
  resetFilters,
  exportCSV,
} = useReport(getAnalytics);
const saving = ref(false);
const file = ref(null);
function chooseUpload(upload) {
  file.value = upload.raw;
}
async function importSpend() {
  if (!file.value) {
    ElMessage.warning("请先选择 CSV 文件");
    return;
  }
  saving.value = true;
  try {
    const body = new FormData();
    body.append("file", file.value);
    await uploadSpend(body);
    ElMessage.success("花费已导入");
    file.value = null;
    await load();
  } catch (e) {
    ElMessage.error(e.message);
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.import-panel {
  margin-top: 20px;
}
.spend-upload {
  max-width: 560px;
  margin: 20px 0;
}
.spend-upload :deep(.el-upload-dragger) {
  padding: 25px;
}
.spend-upload :deep(.el-icon--upload) {
  font-size: 40px;
  margin-bottom: 8px;
  color: #a0cfff;
}
</style>
