<template>
  <section>
    <PageHeader
      :title="data ? `${data.link.name} · 广告统计` : '短链接广告统计'"
      context="短链接管理 / 统计"
      :description="
        data
          ? `短链接 /${data.link.code} · 比较各广告带来的真实点击与咨询`
          : '按真实广告点击记录分别统计。'
      "
    >
      <el-button @click="router.push({ name: 'links' })">返回列表</el-button>
      <el-button @click="openOverview">数据总览</el-button>
    </PageHeader>

    <el-card shadow="never" class="filter-card">
      <el-form inline label-position="top" @submit.prevent="query()">
        <el-form-item label="统计日期">
          <div class="date-controls">
            <el-date-picker
              v-model="range"
              type="daterange"
              value-format="YYYY-MM-DD"
              format="YYYY/MM/DD"
              range-separator="至"
              :clearable="false"
              style="width: 280px"
            />
            <!-- Visible presets make the two most-used operating ranges one-click actions. -->
            <el-button-group>
              <el-button
                :type="activeDatePreset === 'today' ? 'primary' : undefined"
                :aria-pressed="activeDatePreset === 'today'"
                @click="applyDatePreset(1)"
                >今天</el-button
              >
              <el-button
                :type="
                  activeDatePreset === 'three-days' ? 'primary' : undefined
                "
                :aria-pressed="activeDatePreset === 'three-days'"
                @click="applyDatePreset(3)"
                >近 3 天</el-button
              >
            </el-button-group>
          </div>
        </el-form-item>
        <el-form-item label="统计时区">
          <el-select
            v-model="filters.tz"
            style="width: 190px"
            aria-label="统计时区"
          >
            <el-option label="上海 · UTC+8" value="Asia/Shanghai" />
            <el-option label="协调世界时 · UTC" value="UTC" />
            <el-option label="纽约" value="America/New_York" />
            <el-option label="洛杉矶" value="America/Los_Angeles" />
          </el-select>
        </el-form-item>
        <el-form-item label="广告 ID">
          <el-input
            v-model="filters.ad_id"
            clearable
            placeholder="输入完整广告 ID"
            aria-label="广告 ID"
          />
        </el-form-item>
        <el-form-item label="&nbsp;">
          <el-button type="primary" native-type="submit" :loading="busy"
            >查询</el-button
          >
          <el-button @click="reset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      class="notice"
    />
    <div v-loading="busy" class="stats-content">
      <template v-if="data">
        <el-alert
          v-if="data.link.attribution_mode === 'bound' && data.link.ad_id"
          type="warning"
          :closable="false"
          class="notice"
          title="此链接绑定了固定广告 ID，访问会归入该 ID。多条广告共用链接时，请在编辑链接中选择动态来源，并在投放网址中携带各自的 ad_id。"
        />
        <el-alert
          v-if="data.link.mode !== 'landing'"
          type="info"
          :closable="false"
          class="notice"
          title="此链接当前使用直接跳转模式。此表仅统计落地页访问与咨询，直接跳转请求可在数据总览中查看。"
        />
        <div class="stats-grid">
          <el-card v-for="card in cards" :key="card.key" shadow="never">
            <span class="metric-label">{{ card.label }}</span>
            <strong>{{ number(data.summary[card.key]) }}</strong>
            <span class="metric-help">{{ card.help }}</span>
          </el-card>
        </div>

        <el-card shadow="never" class="ad-table">
          <div class="table-heading">
            <div>
              <h2>真实广告表现</h2>
              <p>
                仅统计携带有效 fbclid，并能从 ad_id 或 utm_content 解析广告 ID
                的正常访客。
              </p>
            </div>
            <el-tag effect="plain">共 {{ number(data.total) }} 条广告</el-tag>
          </div>
          <!-- Every row uses one resolved advertising ID; ad_id takes priority over utm_content. -->
          <el-table
            :data="data.items"
            :row-key="rowKey"
            :default-sort="tableSort"
            @sort-change="sortChanged"
            empty-text="所选日期暂无真实广告点击"
          >
            <el-table-column
              label="广告"
              prop="source_value"
              min-width="300"
              sortable="custom"
            >
              <template #default="{ row }">
                <div class="ad-name">{{ rowTitle(row) }}</div>
                <!-- The normalized ID comes from ad_id or its utm_content fallback. -->
                <div class="source-value">广告 ID：{{ row.source_value }}</div>
                <div class="source-meta">
                  <el-tag
                    v-if="row.name_source === 'parameter'"
                    type="info"
                    size="small"
                    >名称来自访问参数</el-tag
                  >
                  <span v-if="row.account_id">账户 {{ row.account_id }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column
              prop="visits"
              label="访问"
              min-width="110"
              align="right"
              sortable="custom"
            >
              <template #default="{ row }">{{ number(row.visits) }}</template>
            </el-table-column>
            <el-table-column
              prop="unique_visitors"
              label="独立访客"
              min-width="130"
              align="right"
              sortable="custom"
            >
              <template #default="{ row }">{{
                number(row.unique_visitors)
              }}</template>
            </el-table-column>
            <el-table-column
              prop="manual_consultations"
              label="手动咨询"
              min-width="130"
              align="right"
              sortable="custom"
            >
              <template #default="{ row }"
                ><b class="manual-count">{{
                  number(row.manual_consultations)
                }}</b></template
              >
            </el-table-column>
            <el-table-column
              prop="auto_redirects"
              label="自动跳转"
              min-width="130"
              align="right"
              sortable="custom"
            >
              <template #default="{ row }">{{
                number(row.auto_redirects)
              }}</template>
            </el-table-column>
          </el-table>
          <el-pagination
            v-if="data.total > data.page_size"
            class="pagination"
            background
            layout="total, prev, pager, next"
            :total="data.total"
            :page-size="data.page_size"
            :current-page="page"
            @current-change="changePage"
          />
          <div class="counting-notes">
            <p>
              真实广告点击要求入口携带有效 fbclid，并且能从 ad_id 或 utm_content
              解析广告
              ID，同时访问被识别为正常访客；机器人预览、预取、可疑请求和普通帖子点击均不计入。
            </p>
            <p>
              真实广告去重访客按本站 Cookie
              在所选范围内去重；同一访客可能点击多条广告，各行人数不能直接相加。缺少
              Cookie 的
              {{ number(data.summary.no_cookie) }} 次真实广告点击不计入人数。
            </p>
            <p>
              手动咨询和自动跳转只统计由上述真实广告点击产生的行为，并分别按同一次访问去重。参数首尾空格会在保存和查询时自动清理。
            </p>
          </div>
        </el-card>
      </template>
      <el-empty v-else-if="!busy && !error" description="暂无统计数据" />
    </div>
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import PageHeader from "../components/PageHeader.vue";
import { getLinkStats } from "../api/analytics";
import { settings } from "../stores/settings";

const route = useRoute(),
  router = useRouter();
const data = ref(null),
  busy = ref(false),
  error = ref("");
const filters = reactive({ start: "", end: "", tz: "", ad_id: "" });
const page = ref(1),
  sort = ref("visits"),
  order = ref("desc");
let generation = 0,
  alive = true;
// Keep the summary aligned with the backend's single real-ad-click cohort.
const cards = [
  {
    key: "visits",
    label: "真实广告点击",
    help: "携带有效 fbclid，并有 ad_id 或 utm_content 广告 ID",
  },
  {
    key: "unique_visitors",
    label: "真实广告去重访客",
    help: "真实广告点击按本站访客 Cookie 去重",
  },
  {
    key: "manual_consultations",
    label: "手动咨询",
    help: "用户主动点击咨询按钮",
  },
  { key: "auto_redirects", label: "自动跳转", help: "倒计时结束后的咨询跳转" },
];
const range = computed({
  get: () => [filters.start, filters.end],
  set: (v) => {
    filters.start = v?.[0] || "";
    filters.end = v?.[1] || "";
  },
});
// Presets follow the selected report timezone and include the current calendar day.
function reportDate(dayOffset = 0) {
  const tz = filters.tz || settings.value.timezone || "Asia/Shanghai";
  const today = new Date().toLocaleDateString("en-CA", { timeZone: tz });
  const date = new Date(`${today}T12:00:00Z`);
  date.setUTCDate(date.getUTCDate() + dayOffset);
  return date.toISOString().slice(0, 10);
}
const activeDatePreset = computed(() => {
  if (filters.end !== reportDate()) return "";
  if (filters.start === filters.end) return "today";
  if (filters.start === reportDate(-2)) return "three-days";
  return "";
});
const tableSort = computed(() => ({
  prop: sort.value,
  order: order.value === "asc" ? "ascending" : "descending",
}));
const number = (v) => Number(v || 0).toLocaleString("zh-CN");
const rowKey = (r) =>
  JSON.stringify([
    r.connection_id,
    r.account_id,
    r.source_kind,
    r.source_value,
  ]);
// The backend excludes unresolved sources, leaving one stable display fallback.
const rowTitle = (r) => r.ad_name || `广告 ${r.source_value}`;
function defaults() {
  const tz = settings.value.timezone || "Asia/Shanghai";
  const end = new Date().toLocaleDateString("en-CA", { timeZone: tz });
  const start = new Date(end + "T12:00:00Z");
  start.setUTCDate(start.getUTCDate() - 6);
  return { start: start.toISOString().slice(0, 10), end, tz, ad_id: "" };
}
// Keep report filters inside the page because the API receives the complete POST body.
async function query(nextPage = 1) {
  page.value = Number(nextPage);
  await load();
}
function reset() {
  Object.assign(filters, defaults());
  sort.value = "visits";
  order.value = "desc";
  query();
}
function applyDatePreset(days) {
  filters.end = reportDate();
  filters.start = reportDate(1 - days);
  // Query immediately so the shortcut behaves as an action rather than a form draft.
  query();
}
function changePage(value) {
  query(value);
}
function sortChanged(value) {
  sort.value = value.order ? value.prop : "visits";
  order.value = value.order === "ascending" ? "asc" : "desc";
  query();
}
function openOverview() {
  router.push({
    name: "overview",
    query: { ...filters, ad_id: "", link_id: String(route.params.id) },
  });
}
async function load() {
  const run = ++generation;
  data.value = null;
  error.value = "";
  busy.value = true;
  try {
    if (!filters.start || !filters.end || filters.start > filters.end)
      throw new Error("请选择有效日期范围");
    const result = await getLinkStats(route.params.id, {
      ...filters,
      page: page.value,
      sort: sort.value,
      order: order.value,
    });
    // Ignore stale responses when users change links, dates or sorting quickly.
    if (alive && run === generation) data.value = result;
  } catch (e) {
    if (alive && run === generation) error.value = e.message;
  } finally {
    if (alive && run === generation) busy.value = false;
  }
}
watch(
  () => route.params.id,
  async () => {
    Object.assign(filters, defaults());
    page.value = 1;
    sort.value = "visits";
    order.value = "desc";
    // Remove old bookmarked query strings left by versions that mirrored POST filters.
    if (Object.keys(route.query).length) {
      await router.replace({
        name: "link-stats",
        params: { id: route.params.id },
      });
    }
    await load();
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  alive = false;
  generation++;
});
</script>

<style scoped>
/* Keep the comparison readable on desktop and let the table scroll on narrow screens. */
.filter-card {
  margin-bottom: 20px;
}
.filter-card :deep(.el-form-item) {
  margin-bottom: 10px;
}
.date-controls {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.notice {
  margin-bottom: 18px;
}
.stats-content {
  min-height: 260px;
}
.stats-grid {
  display: grid;
  /* Four real-click funnel cards wrap naturally without squeezing values on medium screens. */
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 18px;
  margin-bottom: 22px;
}
.stats-grid strong {
  display: block;
  font-size: 30px;
  color: #253044;
  margin: 17px 0;
  font-variant-numeric: tabular-nums;
}
.metric-label {
  font-size: 13px;
  color: #606266;
}
.metric-help {
  font-size: 12px;
  color: #909399;
}
.table-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
}
.table-heading h2 {
  margin: 0 0 8px;
  font-size: 18px;
}
.table-heading p,
.source-meta,
.counting-notes {
  font-size: 12px;
  color: #909399;
}
.table-heading p {
  margin: 0;
}
.ad-name {
  font-weight: 600;
  color: #303133;
  overflow-wrap: anywhere;
}
.source-value {
  color: #606266;
  font-size: 12px;
  overflow-wrap: anywhere;
}
.source-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  margin-top: 5px;
}
.manual-count {
  color: #337ecc;
}
.pagination {
  margin-top: 20px;
  justify-content: flex-end;
}
.counting-notes {
  line-height: 1.8;
  margin-top: 20px;
  border-top: 1px solid #ebeef5;
  padding-top: 12px;
}
.counting-notes p {
  margin: 4px 0;
}
@media (max-width: 1000px) {
  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }
}
@media (max-width: 600px) {
  .filter-card :deep(.el-form-item),
  .filter-card :deep(.el-date-editor) {
    max-width: 100%;
  }
  .stats-grid strong {
    font-size: 25px;
  }
}
</style>
