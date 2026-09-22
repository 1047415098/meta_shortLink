<template>
  <section>
    <PageHeader
      :title="data ? `${data.link.name} · 阅读统计` : '小说投放统计'"
      context="免费小说项目 / 投放链接"
      description="数据只属于当前短链，不与同一本小说的其他投手链接混合。"
      ><el-button @click="back">返回链接</el-button
      ><el-button :loading="loading" @click="load">刷新</el-button></PageHeader
    >
    <section class="panel filters">
      <el-date-picker
        v-model="range"
        type="daterange"
        value-format="YYYY-MM-DD"
        range-separator="至"
        :clearable="false"
      /><el-select v-model="filters.tz"
        ><el-option label="上海 · UTC+8" value="Asia/Shanghai" /><el-option
          label="UTC"
          value="UTC" /><el-option
          label="纽约"
          value="America/New_York" /><el-option
          label="洛杉矶"
          value="America/Los_Angeles" /></el-select
      ><el-input
        v-model="filters.ad_id"
        clearable
        placeholder="广告 ID"
      /><el-button type="primary" @click="query">查询</el-button>
    </section>
    <div v-loading="loading">
      <template v-if="data"
        ><div class="cards">
          <el-card v-for="card in cards" :key="card.key" shadow="never"
            ><span>{{ card.label }}</span
            ><strong>{{
              card.duration
                ? duration(data.summary[card.key])
                : number(data.summary[card.key])
            }}</strong
            ><small>{{ card.help }}</small></el-card
          >
        </div>
        <section class="panel">
          <div class="panel-heading">
            <div>
              <h2>访问明细</h2>
              <p>
                仅统计正常 GET 小说入口访问；未开始采集的历史记录显示“未采集”。
              </p>
            </div>
            <el-tag effect="plain">{{ number(data.total) }} 次</el-tag>
          </div>
          <el-table :data="data.items" empty-text="所选日期暂无访问"
            ><el-table-column label="访问时间" min-width="175"
              ><template #default="{ row }">{{
                timestamp(row.occurred_at)
              }}</template></el-table-column
            ><el-table-column label="匿名访客" min-width="150"
              ><template #default="{ row }"
                ><span class="visitor">{{
                  row.visitor_id ? row.visitor_id.slice(0, 12) : "无 Cookie"
                }}</span></template
              ></el-table-column
            ><el-table-column label="地区" min-width="160"
              ><template #default="{ row }">{{
                [row.country, row.region, row.city]
                  .filter(Boolean)
                  .join(" / ") || "未知"
              }}</template></el-table-column
            ><el-table-column label="设备 / 浏览器" min-width="180"
              ><template #default="{ row }"
                >{{ row.device }} · {{ row.browser }}</template
              ></el-table-column
            ><el-table-column
              prop="source"
              label="来源"
              min-width="110"
            /><el-table-column
              prop="ad_id"
              label="广告 ID"
              min-width="130"
            /><el-table-column label="可见时长" width="110"
              ><template #default="{ row }">{{
                row.visible_seconds == null
                  ? "未采集"
                  : duration(row.visible_seconds)
              }}</template></el-table-column
            ></el-table
          >
          <el-pagination
            v-if="data.total > data.page_size"
            class="pagination"
            background
            layout="total, prev, pager, next"
            :total="data.total"
            :page-size="data.page_size"
            :current-page="filters.page"
            @current-change="changePage"
          /></section
      ></template>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import PageHeader from "../components/PageHeader.vue";
import { getNovelLinkStats } from "../api/novelLinks.js";
import { settings } from "../stores/settings.js";
const route = useRoute(),
  router = useRouter(),
  data = ref(null),
  loading = ref(false);
const today = () =>
  new Date().toLocaleDateString("en-CA", {
    timeZone: settings.value.timezone || "Asia/Shanghai",
  });
const start = new Date(`${today()}T12:00:00Z`);
start.setUTCDate(start.getUTCDate() - 6);
const filters = reactive({
  start: start.toISOString().slice(0, 10),
  end: today(),
  tz: settings.value.timezone || "Asia/Shanghai",
  ad_id: "",
  page: 1,
});
const range = computed({
  get: () => [filters.start, filters.end],
  set: (value) => {
    filters.start = value?.[0] || "";
    filters.end = value?.[1] || "";
  },
});
const cards = [
  { key: "visits", label: "访问次数", help: "当前投放链接的正常入口访问" },
  {
    key: "unique_visitors",
    label: "匿名独立访客",
    help: "按匿名访客 Cookie 去重",
  },
  {
    key: "average_visible_seconds",
    label: "平均可见时长",
    help: "仅计算已采集访问",
    duration: true,
  },
  {
    key: "total_visible_seconds",
    label: "总可见时长",
    help: "前台可见时间合计",
    duration: true,
  },
];
const number = (value) => Number(value || 0).toLocaleString("zh-CN"),
  duration = (value) => {
    const seconds = Math.round(Number(value) || 0);
    return `${Math.floor(seconds / 60)}分 ${seconds % 60}秒`;
  };
const timestamp = (value) =>
  new Date(value).toLocaleString("zh-CN", {
    timeZone: data.value?.timezone || filters.tz,
    hour12: false,
  });
async function load() {
  loading.value = true;
  try {
    data.value = await getNovelLinkStats(route.params.id, { ...filters });
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    loading.value = false;
  }
}
function query() {
  filters.page = 1;
  load();
}
function changePage(page) {
  filters.page = page;
  load();
}
function back() {
  router.push({
    name: "novel-links",
    params: { id: data.value?.link.novel_id || route.query.novel_id || "" },
  });
}
onMounted(load);
</script>

<style scoped>
.filters {
  display: grid;
  grid-template-columns: minmax(260px, 1.5fr) 190px minmax(180px, 1fr) auto;
  gap: 12px;
}
.cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}
.cards span,
.cards small {
  display: block;
  color: #909399;
}
.cards strong {
  display: block;
  font-size: 28px;
  margin: 15px 0;
  color: #253044;
}
.visitor {
  font-family: monospace;
}
.pagination {
  justify-content: flex-end;
  margin-top: 20px;
}
@media (max-width: 900px) {
  .filters,
  .cards {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 600px) {
  .filters,
  .cards {
    grid-template-columns: 1fr;
  }
  .panel {
    overflow-x: auto;
  }
}
</style>
