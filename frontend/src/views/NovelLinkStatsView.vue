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
      /><el-select v-model="filters.tz"
        ><el-option label="上海 · UTC+8" value="Asia/Shanghai" /><el-option
          label="UTC"
          value="UTC" /><el-option
          label="纽约"
          value="America/New_York" /><el-option
          label="洛杉矶"
          value="America/Los_Angeles" /></el-select
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
              v-if="!isTikTok"
              prop="ad_id"
              label="广告 ID"
              min-width="130"
            /><el-table-column v-if="isTikTok" label="Pixel" min-width="180"
              ><template #default="{ row }"
                >{{ row.pixel_name || "—" }}
                <div class="muted">{{ row.pixel_code || "—" }}</div></template
              ></el-table-column
            ><el-table-column
              v-if="isTikTok"
              label="Campaign / Ad Group"
              min-width="200"
              ><template #default="{ row }"
                >{{ row.campaign_id || "—" }}
                <div class="muted">{{ row.adgroup_id || "—" }}</div></template
              ></el-table-column
            ><el-table-column
              v-if="isTikTok"
              label="Creative / Ad"
              min-width="190"
              ><template #default="{ row }"
                >{{ row.creative_id || "—" }}
                <div class="muted">{{ row.ad_id_v2 || "—" }}</div></template
              ></el-table-column
            ><el-table-column v-if="isTikTok" label="点击标识" min-width="130"
              ><template #default="{ row }"
                ><span class="visitor">{{ row.ttclid || "自然访问" }}</span>
                <div class="muted">{{ row.placement || "—" }}</div></template
              ></el-table-column
            ><el-table-column v-if="isTikTok" label="阅读漏斗" min-width="130"
              ><template #default="{ row }"
                ><el-tag
                  size="small"
                  :type="row.started ? 'success' : 'info'"
                  >{{ row.started ? "已开始" : "未开始" }}</el-tag
                ><el-tag
                  size="small"
                  :type="row.qualified ? 'success' : 'info'"
                  >{{ row.qualified ? "已达标" : "未达标" }}</el-tag
                ></template
              ></el-table-column
            ><el-table-column v-if="isTikTok" label="达标事件" min-width="145"
              ><template #default="{ row }"
                ><el-tag
                  v-if="row.qualified_event_status"
                  :type="tiktokStatusType(row.qualified_event_status)"
                  >{{
                    tiktokStatusLabels[row.qualified_event_status] ||
                    row.qualified_event_status
                  }}</el-tag
                ><span v-else class="muted">—</span></template
              ></el-table-column
            ><el-table-column label="可见时长" width="110"
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
import {
  tiktokEventStatuses,
  tiktokStatusLabels,
  tiktokStatusType,
} from "../utils/tiktok.js";
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
const baseCards = [
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
.visitor {
  font-family: monospace;
}
:deep(.el-tag + .el-tag) {
  margin-left: 6px;
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
