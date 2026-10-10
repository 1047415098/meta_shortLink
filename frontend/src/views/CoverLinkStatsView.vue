<template>
  <section>
    <PageHeader
      :title="data ? `${data.link.name} · 封面统计` : '封面投放统计'"
      context="封面项目 / 投放链接"
      description="数据只属于当前封面短链，不与免费小说、语音小说或普通短链接混合。"
    >
      <el-button @click="router.push({ name: 'cover-links' })"
        >返回链接</el-button
      ><el-button :loading="loading" @click="load">刷新</el-button>
    </PageHeader>
    <section class="panel filters">
      <el-date-picker
        v-model="range"
        type="daterange"
        value-format="YYYY-MM-DD"
        range-separator="至"
        :clearable="false"
        :shortcuts="dateShortcuts"
      />
      <el-select v-model="filters.tz"
        ><el-option
          v-for="timezone in REPORT_TIMEZONES"
          :key="timezone.value"
          :label="timezone.label"
          :value="timezone.value"
      /></el-select>
      <el-input v-model="filters.ad_id" clearable placeholder="广告 ID" />
      <el-button type="primary" @click="query">查询</el-button>
    </section>
    <div v-loading="loading">
      <template v-if="data">
        <div class="cards">
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
        <!-- 封面停留行为与全部请求诊断合并到同一张访问记录表。 -->
        <ProjectVisitRecords
          :link-id="route.params.id"
          surface="cover"
          :start="filters.start"
          :end="filters.end"
          :timezone="filters.tz"
          :ad-id="filters.ad_id"
          :request-key="recordsVersion"
        />
      </template>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import PageHeader from "../components/PageHeader.vue";
import ProjectVisitRecords from "../components/ProjectVisitRecords.vue";
import { getCoverLinkStats } from "../api/coverLinks.js";
import { settings } from "../stores/settings.js";
import {
  DEFAULT_REPORT_TIMEZONE,
  REPORT_TIMEZONES,
} from "../constants/reportTimezones.js";
import { reportDateShortcuts } from "../utils/reportDateShortcuts.js";

const route = useRoute(),
  router = useRouter(),
  data = ref(null),
  loading = ref(false);
// The visit list refreshes only after the matching statistics request succeeds.
const recordsVersion = ref(0);
const today = () =>
  new Date().toLocaleDateString("en-CA", {
    timeZone: settings.value.timezone || DEFAULT_REPORT_TIMEZONE,
  });
const filters = reactive({
  start: today(),
  end: today(),
  tz: settings.value.timezone || DEFAULT_REPORT_TIMEZONE,
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
const dateShortcuts = reportDateShortcuts(() => filters.tz);
const cards = [
  { key: "visits", label: "访问次数", help: "当前封面链接的正常入口访问" },
  {
    key: "unique_visitors",
    label: "匿名独立访客",
    help: "按匿名访客 Cookie 去重",
  },
  {
    key: "ten_second_unique_visitors",
    label: "停留满10秒独立访客",
    help: "单次前台可见时长达到10秒",
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
  {
    key: "pending_events",
    label: "待发送事件",
    help: "Meta 与 TikTok 等待发送或重试",
  },
  {
    key: "accepted_events",
    label: "平台已接收",
    help: "仅表示广告平台接口已接收",
  },
  {
    key: "failed_events",
    label: "发送失败事件",
    help: "到对应平台事件记录查看原因",
  },
];
const number = (value) => Number(value || 0).toLocaleString("zh-CN");
const duration = (value) => {
  const seconds = Math.round(Number(value) || 0);
  return `${Math.floor(seconds / 60)}分 ${seconds % 60}秒`;
};
async function load() {
  loading.value = true;
  try {
    data.value = await getCoverLinkStats(route.params.id, { ...filters });
    recordsVersion.value++;
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
  margin: 15px 0;
  color: #253044;
  font-size: 28px;
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
