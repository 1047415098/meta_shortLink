<template>
  <section class="visits-page">
    <PageHeader title="访问明细"
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
    <section class="panel" v-loading="busy">
      <div class="panel-heading">
        <h2>访问记录</h2>
        <span class="muted"
          >共 {{ fmt(clicks.total) }} 条 · 时间按所选时区显示</span
        >
      </div>
      <el-table
        :data="clicks.items || []"
        empty-text="当前筛选条件下没有访问记录"
        ><el-table-column label="访问时间" width="170"
          ><template #default="{ row }">{{
            displayTime(row.occurred_at)
          }}</template></el-table-column
        ><el-table-column
          prop="code"
          label="短码"
          width="100"
        /><el-table-column label="访问 / 咨询" min-width="160"
          ><template #default="{ row }"
            ><div>
              {{ row.event_type === "landing" ? "落地页访问" : "直接跳转" }}
            </div>
            <small class="muted"
              ><span v-if="row.whatsapp_clicked_at"
                >手动咨询：{{ displayTime(row.whatsapp_clicked_at) }}</span
              ><br
                v-if="row.whatsapp_clicked_at && row.auto_redirected_at"
              /><span v-if="row.auto_redirected_at"
                >自动跳转：{{ displayTime(row.auto_redirected_at) }}</span
              ><span v-if="!row.whatsapp_clicked_at && !row.auto_redirected_at"
                >—</span
              ></small
            ></template
          ></el-table-column
        ><el-table-column label="入口" width="90"
          ><template #default="{ row }"
            ><el-tag
              :type="row.surface === 'audio_novel' ? 'warning' : 'info'"
              effect="plain"
            >
              {{ row.surface === "audio_novel" ? "语音小说站" : "短链接" }}
            </el-tag></template
          ></el-table-column
        ><el-table-column label="设备" min-width="135"
          ><template #default="{ row }"
            >{{ deviceLabel[row.device] || row.device || "未知" }}
            <div class="muted">{{ row.os }} · {{ row.browser }}</div></template
          ></el-table-column
        ><el-table-column label="地区" min-width="130"
          ><template #default="{ row }">{{
            [row.country, row.region, row.city].filter(Boolean).join(" / ") ||
            "未知"
          }}</template></el-table-column
        ><el-table-column
          prop="source"
          label="来源"
          min-width="100"
        /><el-table-column
          prop="ad_id"
          label="广告 ID"
          min-width="130"
        /><el-table-column label="访客标识" width="115"
          ><template #default="{ row }"
            ><el-tag type="info" effect="plain">{{
              {
                recognized: "已识别",
                issued: "首次下发",
                disabled: "未启用",
              }[row.cookie_status] || row.cookie_status
            }}</el-tag></template
          ></el-table-column
        ><el-table-column label="流量分类" width="130"
          ><template #default="{ row }"
            ><el-tooltip :content="row.reason || '未记录分类原因'"
              ><el-tag
                :type="
                  row.classification === 'bot'
                    ? 'info'
                    : row.classification === 'suspicious'
                      ? 'warning'
                      : 'success'
                "
                >{{
                  className[row.classification] || row.classification
                }}</el-tag
              ></el-tooltip
            ><span v-if="row.attribution_conflict" class="conflict"
              >归因冲突</span
            ></template
          ></el-table-column
        ></el-table
      ><el-pagination
        class="pagination"
        layout="prev, pager, next, total"
        :total="clicks.total"
        :page-size="50"
        v-model:current-page="clickPage"
        @current-change="load(false)"
      />
    </section>
  </section>
</template>

<script setup>
import { computed } from "vue";

import PageHeader from "../components/PageHeader.vue";
import { Refresh, Download } from "@element-plus/icons-vue";

import AnalyticsFilter from "../components/AnalyticsFilter.vue";
import { useReport } from "../composables/useReport";
import { getVisits } from "../api/analytics";

const {
  filters,
  data,
  links,
  busy,
  error,
  load,
  resetFilters,
  exportCSV,
  clickPage,
} = useReport(getVisits);
const clicks = computed(() => data.value || { items: [], total: 0 });
const fmt = (value) => new Intl.NumberFormat("zh-CN").format(value || 0);
const deviceLabel = {
  mobile: "手机",
  tablet: "平板",
  desktop: "电脑",
  unknown: "未知设备",
};
const className = {
  normal: "过滤后",
  filtered: "过滤后",
  bot: "机器人",
  suspicious: "疑似异常",
  unclassified: "未分类",
  head: "HEAD 请求",
  prefetch: "预取",
};
function displayTime(v) {
  try {
    return new Intl.DateTimeFormat("zh-CN", {
      dateStyle: "short",
      timeStyle: "medium",
      timeZone: filters.tz,
    }).format(new Date(v));
  } catch {
    return v;
  }
}
</script>

<style scoped>
.visits-page {
  min-width: 0;
}
</style>
