<template>
  <section>
    <PageHeader
      :title="data ? `${data.link.name} · 阅读统计` : '小说投放统计'"
      context="免费小说项目 / 投放链接"
      description="数据只属于当前短链，不与同一本小说的其他投手链接混合。"
      ><el-button @click="back">返回链接</el-button
      ><el-button :loading="loading" @click="load">刷新</el-button></PageHeader
    >
    <section class="panel filters" :class="{ 'filters--tiktok': isTikTok }">
      <el-date-picker
        v-model="range"
        type="daterange"
        value-format="YYYY-MM-DD"
        range-separator="至"
        :clearable="false"
        :shortcuts="dateShortcuts"
      /><el-select v-model="filters.tz"
        ><el-option
          v-for="timezone in REPORT_TIMEZONES"
          :key="timezone.value"
          :label="timezone.label"
          :value="timezone.value" /></el-select
      ><el-input
        v-if="!isTikTok"
        v-model="filters.ad_id"
        clearable
        placeholder="广告 ID"
      /><template v-else
        ><el-input
          v-model="filters.campaign_id"
          clearable
          placeholder="Campaign ID" /><el-input
          v-model="filters.adgroup_id"
          clearable
          placeholder="Ad Group ID" /><el-input
          v-model="filters.creative_id"
          clearable
          placeholder="Creative ID" /><el-input
          v-model="filters.ad_id_v2"
          clearable
          placeholder="Ad ID v2" /><el-select
          v-model="filters.event_status"
          clearable
          placeholder="全部事件状态"
          ><el-option
            v-for="status in eventStatuses"
            :key="status"
            :value="status"
            :label="tiktokStatusLabels[status]" /></el-select></template
      ><el-button type="primary" @click="query">查询</el-button>
    </section>
    <el-alert
      v-if="isTikTok"
      class="notice"
      type="info"
      :closable="false"
      title="TikTok 是否归因：本系统未知，请到 TikTok Ads Manager 查看。"
    />
    <div v-loading="loading">
      <template v-if="data"
        ><div class="cards">
          <el-card v-for="card in cards" :key="card.key" shadow="never"
            ><span>{{ card.label }}</span
            ><strong>{{
              card.duration
                ? duration(data.summary[card.key])
                : card.percent
                  ? percent(data.summary[card.key])
                  : number(data.summary[card.key])
            }}</strong
            ><small>{{ cardHelp(card) }}</small></el-card
          >
        </div>
        <!-- 阅读行为、入口章节和异常流量合并到同一张访问记录表。 -->
        <ProjectVisitRecords
          :link-id="route.params.id"
          surface="novel"
          :start="filters.start"
          :end="filters.end"
          :timezone="filters.tz"
          :ad-id="filters.ad_id"
          :campaign-id="filters.campaign_id"
          :adgroup-id="filters.adgroup_id"
          :creative-id="filters.creative_id"
          :ad-id-v2="filters.ad_id_v2"
          :event-status="filters.event_status"
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
import { getNovelLinkStats } from "../api/novelLinks.js";
import { settings } from "../stores/settings.js";
import {
  DEFAULT_REPORT_TIMEZONE,
  REPORT_TIMEZONES,
} from "../constants/reportTimezones.js";
import { reportDateShortcuts } from "../utils/reportDateShortcuts.js";
import { tiktokEventStatuses, tiktokStatusLabels } from "../utils/tiktok.js";
const route = useRoute(),
  router = useRouter(),
  data = ref(null),
  loading = ref(false);
// A successful report query is the single refresh boundary for the complete visit list.
const recordsVersion = ref(0);
const today = () =>
  new Date().toLocaleDateString("en-CA", {
    // Use fixed UTC-8 until the settings response supplies the deployed choice.
    timeZone: settings.value.timezone || DEFAULT_REPORT_TIMEZONE,
  });
// Start with today's report day; operators choose any historical range in the filter.
const filters = reactive({
  start: today(),
  end: today(),
  tz: settings.value.timezone || DEFAULT_REPORT_TIMEZONE,
  ad_id: "",
  campaign_id: "",
  adgroup_id: "",
  creative_id: "",
  ad_id_v2: "",
  event_status: "",
  page: 1,
});
const isTikTok = computed(() => data.value?.link.ad_platform === "tiktok");
const eventStatuses = tiktokEventStatuses;
const range = computed({
  get: () => [filters.start, filters.end],
  set: (value) => {
    filters.start = value?.[0] || "";
    filters.end = value?.[1] || "";
  },
});
// Match the shared dashboard calendar shortcuts and selected report timezone.
const dateShortcuts = reportDateShortcuts(() => filters.tz);
const baseCards = [
  { key: "visits", label: "访问次数", help: "当前投放链接的正常入口访问" },
  {
    key: "unique_visitors",
    label: "匿名独立访客",
    help: "按匿名访客 Cookie 去重",
  },
  // 固定 10 秒指标用于比较各投放短链的阅读质量，不跟随平台回传阈值变化。
  {
    key: "ten_second_unique_visitors",
    label: "停留满10秒独立访客",
    help: "单次前台可见时长达到10秒，按匿名访客 Cookie 去重",
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
const tiktokCards = [
  {
    key: "start_reading_visitors",
    label: "开始阅读人数",
    countKey: "start_reading_count",
  },
  {
    key: "qualified_visitors",
    label: "达标阅读人数",
    countKey: "qualified_count",
  },
  {
    key: "qualified_rate",
    label: "达标阅读率",
    help: "按当前短链访问次数计算",
    percent: true,
  },
  {
    key: "tiktok_pending_events",
    label: "待发送事件",
    help: "包含已保存、发送中和等待重试",
  },
  {
    key: "tiktok_accepted_events",
    label: "TikTok 已接收",
    help: "仅表示 Events API 接收成功",
  },
  {
    key: "tiktok_failed_events",
    label: "发送失败事件",
    help: "可到 TikTok 事件记录查看并重试",
  },
];
const cards = computed(() =>
  isTikTok.value ? [...baseCards, ...tiktokCards] : baseCards,
);
const number = (value) => Number(value || 0).toLocaleString("zh-CN"),
  percent = (value) => `${Number(value || 0).toFixed(1)}%`,
  duration = (value) => {
    const seconds = Math.round(Number(value) || 0);
    return `${Math.floor(seconds / 60)}分 ${seconds % 60}秒`;
  };
function cardHelp(card) {
  if (card.countKey)
    return `${number(data.value?.summary[card.countKey])} 次阅读动作`;
  return card.help;
}
async function load() {
  loading.value = true;
  try {
    data.value = await getNovelLinkStats(route.params.id, { ...filters });
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
function back() {
  router.push({
    name: "novel-links",
    // The current response is the source of the parent ID; no URL query fallback.
    params: { id: data.value?.link.novel_id || "" },
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
.filters--tiktok {
  grid-template-columns: repeat(4, minmax(155px, 1fr));
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
