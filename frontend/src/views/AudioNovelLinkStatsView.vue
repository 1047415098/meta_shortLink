<template>
  <section>
    <PageHeader
      :title="data ? `${data.link.name} · 播放统计` : '语音小说投放统计'"
      context="语音小说项目 / 投放链接"
      description="数据只属于当前短链，不与同一部语音小说的其他投手链接混合。"
    >
      <el-button @click="back">返回链接</el-button>
      <el-button :loading="loading" @click="load">刷新</el-button>
    </PageHeader>

    <section class="panel filters" :class="{ 'filters--tiktok': isTikTok }">
      <el-date-picker
        v-model="range"
        type="daterange"
        value-format="YYYY-MM-DD"
        range-separator="至"
        :clearable="false"
        :shortcuts="dateShortcuts"
      />
      <el-select v-model="filters.tz">
        <el-option
          v-for="timezone in REPORT_TIMEZONES"
          :key="timezone.value"
          :label="timezone.label"
          :value="timezone.value"
        />
      </el-select>
      <el-input
        v-if="!isTikTok"
        v-model="filters.ad_id"
        clearable
        placeholder="广告 ID"
      />
      <template v-else>
        <el-input
          v-model="filters.campaign_id"
          clearable
          placeholder="Campaign ID"
        />
        <el-input
          v-model="filters.adgroup_id"
          clearable
          placeholder="Ad Group ID"
        />
        <el-input
          v-model="filters.creative_id"
          clearable
          placeholder="Creative ID"
        />
        <el-input v-model="filters.ad_id_v2" clearable placeholder="Ad ID v2" />
      </template>
      <el-select
        v-model="filters.event_status"
        clearable
        placeholder="全部事件状态"
      >
        <el-option
          v-for="status in eventStatuses"
          :key="status"
          :value="status"
          :label="eventStatusLabel(status)"
        />
      </el-select>
      <el-button type="primary" @click="query">查询</el-button>
    </section>

    <el-alert
      class="notice"
      type="info"
      :closable="false"
      title="平台 API 已接收不代表最终广告归因；最终结果请到对应 Ads Manager 查看。播放完成仅用于内部统计，不额外回传广告事件。"
    />

    <div v-loading="loading">
      <template v-if="data">
        <!-- 只展示服务端脱敏后的固定原因，前端不读取任何平台凭证。 -->
        <el-alert
          v-if="data.delivery_config_status === 'blocked'"
          class="notice"
          type="warning"
          :closable="false"
          show-icon
          :title="`回传配置阻塞：${data.delivery_blocked_reason}`"
        />

        <div class="cards">
          <el-card v-for="card in cards" :key="card.key" shadow="never">
            <span>{{ card.label }}</span>
            <!-- NULL 表示尚未采集；数值 0 仍是有效的已采集结果。 -->
            <strong>
              {{
                card.duration
                  ? data.summary[card.key] == null
                    ? "未采集"
                    : duration(data.summary[card.key])
                  : card.percent
                    ? percent(data.summary[card.key])
                    : number(data.summary[card.key])
              }}
            </strong>
            <small>{{ cardHelp(card) }}</small>
          </el-card>
        </div>

        <!-- 单表同时承载有效播放行为与异常请求诊断，顶部卡片口径保持不变。 -->
        <ProjectVisitRecords
          :link-id="route.params.id"
          surface="audio_novel"
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
import { getAudioNovelLinkStats } from "../api/audioNovelLinks.js";
import { settings } from "../stores/settings.js";
import {
  DEFAULT_REPORT_TIMEZONE,
  REPORT_TIMEZONES,
} from "../constants/reportTimezones.js";
import { reportDateShortcuts } from "../utils/reportDateShortcuts.js";
import { tiktokEventStatuses, tiktokStatusLabels } from "../utils/tiktok.js";

const route = useRoute();
const router = useRouter();
const data = ref(null);
// Keep the shared visit list synchronized with the last completed playback report query.
const recordsVersion = ref(0);
const loading = ref(false);
// Audio reports share the fixed UTC-8 default with all operator statistics.
const timezone = settings.value.timezone || DEFAULT_REPORT_TIMEZONE;
const today = () =>
  new Date().toLocaleDateString("en-CA", { timeZone: timezone });
// The initial audio report is today's data; wider ranges require an operator selection.
const filters = reactive({
  start: today(),
  end: today(),
  tz: timezone,
  ad_id: "",
  campaign_id: "",
  adgroup_id: "",
  creative_id: "",
  ad_id_v2: "",
  event_status: "",
  page: 1,
  page_size: 50,
});
const isTikTok = computed(() => data.value?.link.ad_platform === "tiktok");
const metaEventStatuses = [
  "pending",
  "processing",
  "succeeded",
  "retry",
  "failed",
  "expired",
  "skipped",
];
const eventStatuses = computed(() =>
  isTikTok.value ? tiktokEventStatuses : metaEventStatuses,
);
const range = computed({
  get: () => [filters.start, filters.end],
  set: (value) => {
    filters.start = value?.[0] || "";
    filters.end = value?.[1] || "";
  },
});
// Match the shared dashboard calendar shortcuts and selected report timezone.
const dateShortcuts = reportDateShortcuts(() => filters.tz);

const cards = [
  { key: "visits", label: "访问次数", help: "当前链接的正常入口访问" },
  {
    key: "unique_visitors",
    label: "匿名独立访客",
    help: "按匿名访客 Cookie 去重",
  },
  {
    key: "average_visible_seconds",
    label: "平均页面可见",
    help: "仅计算已采集访问",
    duration: true,
  },
  {
    key: "total_visible_seconds",
    label: "总页面可见",
    help: "页面在前台的时间合计",
    duration: true,
  },
  {
    key: "average_playback_seconds",
    label: "平均实际播放",
    help: "仅计算已开始播放访问",
    duration: true,
  },
  {
    key: "total_playback_seconds",
    label: "总实际播放",
    help: "真实播放时间合计",
    duration: true,
  },
  {
    key: "started_count",
    label: "开始收听",
    countKey: "started_visitors",
    help: "开始收听人数",
  },
  {
    key: "start_rate",
    label: "开始率",
    help: "开始次数 / 访问次数",
    percent: true,
  },
  {
    key: "qualified_count",
    label: "播放达标",
    countKey: "qualified_visitors",
    help: "达标人数",
  },
  {
    key: "qualified_rate",
    label: "达标率",
    help: "达标次数 / 开始次数",
    percent: true,
  },
  {
    key: "completed_count",
    label: "播放完成",
    countKey: "completed_visitors",
    help: "完成人数",
  },
  {
    key: "completion_rate",
    label: "完成率",
    help: "完成次数 / 开始次数",
    percent: true,
  },
  { key: "saved_events", label: "已保存事件", help: "当前平台的语音广告事件" },
  { key: "pending_events", label: "待发送事件", help: "包含发送中与等待重试" },
  { key: "accepted_events", label: "平台已接收", help: "仅代表平台 API 接收" },
  { key: "failed_events", label: "发送失败", help: "可到平台事件记录诊断" },
];

const number = (value) => Number(value || 0).toLocaleString("zh-CN");
const percent = (value) => `${Number(value || 0).toFixed(1)}%`;
function duration(value) {
  const seconds = Math.max(0, Math.round(Number(value) || 0));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const tail = `${minutes}分 ${seconds % 60}秒`;
  return hours ? `${hours}时 ${tail}` : tail;
}
function cardHelp(card) {
  if (card.countKey)
    return `${number(data.value?.summary[card.countKey])} 位匿名访客`;
  return card.help;
}
function eventStatusLabel(status) {
  const labels = {
    "": "—",
    browser_only: "浏览器 Pixel",
    pending: "已保存 / 待发送",
    processing: "发送中",
    sending: "发送中",
    succeeded: "Meta 已接收",
    accepted: "TikTok 已接收",
    retry: "等待重试",
    failed: "发送失败",
    expired: "已过期",
    skipped: "未发送",
  };
  return labels[status] || tiktokStatusLabels[status] || status || "—";
}
async function load() {
  loading.value = true;
  try {
    data.value = await getAudioNovelLinkStats(route.params.id, { ...filters });
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
    name: "audio-novel-links",
    params: {
      // The current response is the source of the parent ID; no URL query fallback.
      id: data.value?.link.audio_novel_id || "",
    },
  });
}
onMounted(load);
</script>

<style scoped>
.filters {
  display: grid;
  grid-template-columns:
    minmax(260px, 1.5fr) 190px minmax(170px, 1fr) minmax(170px, 1fr)
    auto;
  gap: 12px;
}
.filters--tiktok {
  grid-template-columns: repeat(4, minmax(150px, 1fr));
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
@media (max-width: 1050px) {
  .filters,
  .filters--tiktok,
  .cards {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 600px) {
  .filters,
  .filters--tiktok,
  .cards {
    grid-template-columns: 1fr;
  }
  .panel {
    overflow-x: auto;
  }
}
</style>
