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
      <el-button
        v-if="data?.link.ad_platform === 'tiktok'"
        @click="router.push({ name: 'tiktok-events' })"
        >TikTok 事件记录</el-button
      >
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
              :shortcuts="dateShortcuts"
              style="width: 280px"
            />
          </div>
        </el-form-item>
        <el-form-item label="统计时区">
          <el-select
            v-model="filters.tz"
            style="width: 190px"
            aria-label="统计时区"
          >
            <el-option
              v-for="timezone in REPORT_TIMEZONES"
              :key="timezone.value"
              :label="timezone.label"
              :value="timezone.value"
            />
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
        <!-- Direct mode contributes server-side visits and automatic handoffs, but never manual button clicks. -->
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
          title="此链接当前使用直接跳转模式。访问计入真实广告点击，跳转计入自动跳转；该模式不会产生手动咨询。"
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
              <p>{{ realClickDescription }}</p>
            </div>
            <el-tag effect="plain">共 {{ number(data.total) }} 条广告</el-tag>
          </div>
          <!-- Every row uses the canonical explicit ad_id from the visit URL. -->
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
                <!-- The normalized ID comes only from the explicit ad_id. -->
                <div class="source-value">
                  {{ isTikTok ? "TikTok 广告 ID" : "广告 ID" }}：{{
                    row.source_value
                  }}
                </div>
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
            <!-- Region counts use the same strict real-ad-click visits as the row total. -->
            <el-table-column label="访问地区" min-width="280">
              <template #default="{ row }">
                <div v-if="row.locations?.length" class="location-list">
                  <div
                    v-for="location in row.locations.slice(0, 3)"
                    :key="locationKey(location)"
                    class="location-row"
                  >
                    <span>{{ locationLabel(location) }}</span>
                    <b>{{ number(location.visits) }}</b>
                  </div>
                  <el-popover
                    v-if="row.locations.length > 3"
                    placement="bottom-start"
                    :width="380"
                    trigger="click"
                  >
                    <template #reference>
                      <el-button link type="primary" class="location-more"
                        >另外 {{ number(row.locations.length - 3) }} 个地区 ·
                        {{ number(locationVisitTotal(row.locations.slice(3))) }}
                        次</el-button
                      >
                    </template>
                    <div class="location-popover">
                      <div
                        v-for="location in row.locations.slice(3)"
                        :key="locationKey(location)"
                        class="location-row"
                      >
                        <span>{{ locationLabel(location) }}</span>
                        <b>{{ number(location.visits) }}</b>
                      </div>
                    </div>
                  </el-popover>
                  <!-- This total uses every bucket once, including unresolved GeoIP. -->
                  <div class="location-total">
                    地区合计
                    {{ number(locationVisitTotal(row.locations)) }} 次
                  </div>
                </div>
                <span v-else class="muted">暂无地区数据</span>
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
            <p v-if="isTikTok">
              TikTok 真实广告点击要求入口携带有效 ttclid 和明确的
              ad_id_v2，同时访问被识别为正常访客；机器人预览、预取、可疑请求和普通帖子点击均不计入。
            </p>
            <p v-else>
              真实广告点击要求入口携带有效 fbclid 和明确的
              ad_id，同时访问被识别为正常访客；机器人预览、预取、可疑请求和普通帖子点击均不计入。
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
        <!-- 普通短链接保留广告聚合表，同时补充与系统页面一致的逐次访问记录。 -->
        <ProjectVisitRecords
          :link-id="route.params.id"
          surface="short_link"
          :start="filters.start"
          :end="filters.end"
          :timezone="filters.tz"
          :ad-id="isTikTok ? '' : filters.ad_id"
          :ad-id-v2="isTikTok ? filters.ad_id : ''"
          :request-key="recordsVersion"
        />
      </template>
      <el-empty v-else-if="!busy && !error" description="暂无统计数据" />
    </div>
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import PageHeader from "../components/PageHeader.vue";
import ProjectVisitRecords from "../components/ProjectVisitRecords.vue";
import { getLinkStats } from "../api/analytics";
import { settings } from "../stores/settings";
import {
  DEFAULT_REPORT_TIMEZONE,
  REPORT_TIMEZONES,
} from "../constants/reportTimezones";
import { reportDateShortcuts } from "../utils/reportDateShortcuts";
import { locationLabel, locationVisitTotal } from "../utils";

const route = useRoute(),
  router = useRouter();
const data = ref(null),
  busy = ref(false),
  error = ref("");
// Visit rows use their own pagination but share the last successfully applied report filters.
const recordsVersion = ref(0);
const filters = reactive({ start: "", end: "", tz: "", ad_id: "" });
const page = ref(1),
  sort = ref("visits"),
  order = ref("desc");
let generation = 0,
  alive = true;
// Keep the summary aligned with the backend's single real-ad-click cohort.
const isTikTok = computed(() => data.value?.link.ad_platform === "tiktok");
const realClickDescription = computed(() =>
  isTikTok.value
    ? "仅统计携带有效 ttclid 和明确 ad_id_v2 的正常访客。"
    : "仅统计携带有效 fbclid 和明确 ad_id 的正常访客。",
);
const cards = computed(() => [
  {
    key: "visits",
    label: "真实广告点击",
    help: isTikTok.value
      ? "携带有效 ttclid 和明确 ad_id_v2"
      : "携带有效 fbclid 和明确 ad_id",
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
]);
const range = computed({
  get: () => [filters.start, filters.end],
  set: (v) => {
    filters.start = v?.[0] || "";
    filters.end = v?.[1] || "";
  },
});
// Every statistics page uses the same three shortcut ranges in its report timezone.
const dateShortcuts = reportDateShortcuts(
  () => filters.tz || settings.value.timezone || DEFAULT_REPORT_TIMEZONE,
);
const tableSort = computed(() => ({
  prop: sort.value,
  order: order.value === "asc" ? "ascending" : "descending",
}));
const number = (v) => Number(v || 0).toLocaleString("zh-CN");
// GeoIP tuples form a stable Vue key even when several locations share a visit count.
const locationKey = (location) =>
  JSON.stringify([location.country, location.region, location.city]);
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
  // New short-link reports start in fixed UTC-8 unless the operator switches it.
  const tz = settings.value.timezone || DEFAULT_REPORT_TIMEZONE;
  const end = new Date().toLocaleDateString("en-CA", { timeZone: tz });
  // A link's first statistics query is today's data; date controls expose historical ranges.
  return { start: end, end, tz, ad_id: "" };
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
function changePage(value) {
  query(value);
}
function sortChanged(value) {
  sort.value = value.order ? value.prop : "visits";
  order.value = value.order === "ascending" ? "asc" : "desc";
  query();
}
function openOverview() {
  // Do not mirror report filters into a navigation URL.
  router.push({ name: "overview" });
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
    if (alive && run === generation) {
      data.value = result;
      recordsVersion.value++;
    }
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
.location-list,
.location-popover {
  display: grid;
  gap: 6px;
}
.location-popover {
  max-height: 320px;
  overflow-y: auto;
  padding-right: 4px;
}
.location-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
  color: #606266;
  font-size: 12px;
  line-height: 1.45;
}
.location-row span {
  overflow-wrap: anywhere;
}
.location-row b {
  color: #303133;
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}
.location-more {
  justify-self: start;
  padding: 0;
  min-height: auto;
}
.location-total {
  color: #909399;
  font-size: 12px;
  border-top: 1px dashed #dcdfe6;
  padding-top: 5px;
  width: fit-content;
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
