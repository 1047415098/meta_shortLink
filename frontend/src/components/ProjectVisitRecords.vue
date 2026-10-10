<template>
  <section class="panel project-visits" v-loading="loading">
    <div class="panel-heading">
      <div>
        <h2>访问记录</h2>
        <p>{{ scopeDescription }}</p>
      </div>
      <div class="heading-actions">
        <!-- 默认只看参与统计的有效访问，运营仍可切换检查全部和异常请求。 -->
        <el-radio-group
          v-model="trafficScope"
          size="small"
          aria-label="访问记录范围"
        >
          <el-radio-button value="valid">有效访问</el-radio-button>
          <el-radio-button value="all">全部记录</el-radio-button>
          <el-radio-button value="abnormal">异常流量</el-radio-button>
        </el-radio-group>
        <el-tag effect="plain">共 {{ number(total) }} 条</el-tag>
      </div>
    </div>
    <el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      class="notice"
    />
    <el-table :data="items" empty-text="当前筛选条件下没有访问记录">
      <el-table-column label="访问时间" width="170">
        <template #default="{ row }">{{
          displayTime(row.occurred_at)
        }}</template>
      </el-table-column>
      <el-table-column prop="code" label="短码" width="100" />
      <el-table-column label="统计口径" width="105">
        <template #default="{ row }">
          <el-tag :type="isIncluded(row) ? 'success' : 'info'" effect="plain">
            {{ isIncluded(row) ? "计入统计" : "不计入" }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="访问行为" min-width="220">
        <template #default="{ row }">
          <div>{{ eventTypeLabel(row) }} · {{ row.method || "—" }}</div>
          <small v-if="surface === 'short_link'" class="muted behavior-lines">
            <span v-if="row.whatsapp_clicked_at"
              >手动咨询：{{ displayTime(row.whatsapp_clicked_at) }}</span
            >
            <span v-if="row.auto_redirected_at"
              >自动跳转：{{ displayTime(row.auto_redirected_at) }}</span
            >
            <span v-if="!row.whatsapp_clicked_at && !row.auto_redirected_at"
              >暂无后续动作</span
            >
          </small>
          <small
            v-else-if="surface === 'audio_novel'"
            class="muted behavior-lines"
          >
            <span>可见：{{ duration(row.visible_seconds) }}</span>
            <span>
              播放：{{ duration(row.playback_seconds) }} / 有效消费：{{
                duration(row.media_consumed_seconds)
              }}
            </span>
            <span class="behavior-tags">
              <el-tag v-if="row.audio_started" size="small">已播放</el-tag>
              <el-tag v-if="row.audio_qualified" size="small" type="success"
                >已达有效播放</el-tag
              >
              <el-tag v-if="row.audio_completed" size="small" type="warning"
                >已完成</el-tag
              >
            </span>
          </small>
          <small v-else-if="surface === 'novel'" class="muted behavior-lines">
            <span>{{ chapterLabel(row) }}</span>
            <span>可见：{{ duration(row.visible_seconds) }}</span>
            <span class="behavior-tags">
              <el-tag v-if="row.reading_started" size="small"
                >已开始阅读</el-tag
              >
              <el-tag v-if="row.reading_qualified" size="small" type="success"
                >已达有效阅读</el-tag
              >
            </span>
          </small>
          <small v-else class="muted behavior-lines">
            <span>可见：{{ duration(row.visible_seconds) }}</span>
          </small>
        </template>
      </el-table-column>
      <el-table-column label="入口" width="100">
        <template #default="{ row }">
          <el-tag :type="surfaceType(row.surface)" effect="plain">
            {{ surfaceLabel(row.surface) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="设备" min-width="145">
        <template #default="{ row }">
          {{ deviceLabel[row.device] || row.device || "未知" }}
          <div class="muted">
            {{ row.os || "未知系统" }} · {{ row.browser || "未知浏览器" }}
          </div>
        </template>
      </el-table-column>
      <el-table-column label="地区" min-width="150">
        <template #default="{ row }">
          {{
            [row.country, row.region, row.city].filter(Boolean).join(" / ") ||
            "未知"
          }}
        </template>
      </el-table-column>
      <el-table-column label="来源 / 广告" min-width="170">
        <template #default="{ row }">
          <div>{{ row.source || "—" }}</div>
          <div class="muted">
            {{ row.ad_id_v2 || row.ad_id || "无广告 ID" }}
          </div>
        </template>
      </el-table-column>
      <el-table-column label="Campaign / Ad Group" min-width="185">
        <template #default="{ row }">
          <div>{{ row.campaign_id || "—" }}</div>
          <div class="muted">{{ row.adgroup_id || row.adset_id || "—" }}</div>
        </template>
      </el-table-column>
      <el-table-column label="Creative / Placement" min-width="175">
        <template #default="{ row }">
          <div>{{ row.creative_id || "—" }}</div>
          <div class="muted">{{ row.placement || "—" }}</div>
        </template>
      </el-table-column>
      <el-table-column label="访客标识" min-width="150">
        <template #default="{ row }">
          <span class="visitor">{{ visitorLabel(row.visitor_id) }}</span>
          <div>
            <el-tag type="info" effect="plain" size="small">
              {{ cookieLabel(row.cookie_status) }}
            </el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="流量分类" width="130">
        <template #default="{ row }">
          <el-tooltip :content="row.reason || '未记录分类原因'">
            <el-tag :type="classificationType(row.classification)">
              {{ classLabel[row.classification] || row.classification }}
            </el-tag>
          </el-tooltip>
          <span v-if="row.attribution_conflict" class="conflict">归因冲突</span>
        </template>
      </el-table-column>
      <!-- 详情固定在右侧，横向滚动时仍可随时打开完整访问快照。 -->
      <el-table-column label="操作" width="82" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="detailDialog?.open(row.id)">
            明细
          </el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination
      v-if="total > pageSize"
      class="pagination"
      layout="prev, pager, next, total"
      :total="total"
      :page-size="pageSize"
      v-model:current-page="page"
      @current-change="load"
    />
    <VisitDetailDialog ref="detailDialog" :timezone="timezone" />
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { getVisits } from "../api/analytics.js";
import VisitDetailDialog from "./VisitDetailDialog.vue";

const props = defineProps({
  linkId: { type: [String, Number], required: true },
  surface: { type: String, required: true },
  start: { type: String, required: true },
  end: { type: String, required: true },
  timezone: { type: String, required: true },
  adId: { type: String, default: "" },
  campaignId: { type: String, default: "" },
  adgroupId: { type: String, default: "" },
  creativeId: { type: String, default: "" },
  adIdV2: { type: String, default: "" },
  eventStatus: { type: String, default: "" },
  requestKey: { type: Number, default: 0 },
});
const items = ref([]),
  total = ref(0),
  page = ref(1),
  trafficScope = ref("valid"),
  loading = ref(false),
  error = ref(""),
  detailDialog = ref(null);
const pageSize = 50;
let generation = 0,
  alive = true;

const scopeDescription = computed(
  () =>
    ({
      valid: "仅展示计入顶部统计卡片的正常入口访问。",
      all: "展示当前链接的全部请求，包括机器人、预取和异常流量。",
      abnormal: "仅展示不计入业务统计的机器人、预取及异常请求。",
    })[trafficScope.value],
);

// Project lists call the same JSON-body endpoint as /admin/visits, scoped to one link and surface.
async function load() {
  if (!props.start || !props.end || !props.timezone || !props.linkId) return;
  const run = ++generation;
  loading.value = true;
  error.value = "";
  try {
    const result = await getVisits({
      start: props.start,
      end: props.end,
      tz: props.timezone,
      link_id: String(props.linkId),
      surface: props.surface,
      ad_id: props.adId || "",
      campaign_id: props.campaignId || "",
      adgroup_id: props.adgroupId || "",
      creative_id: props.creativeId || "",
      ad_id_v2: props.adIdV2 || "",
      event_status: props.eventStatus || "",
      traffic_scope: trafficScope.value,
      page: page.value,
    });
    if (alive && run === generation) {
      items.value = result.items || [];
      total.value = Number(result.total || 0);
    }
  } catch (reason) {
    if (alive && run === generation) {
      items.value = [];
      total.value = 0;
      error.value = reason.message;
    }
  } finally {
    if (alive && run === generation) loading.value = false;
  }
}

watch(
  // Parent filters apply only after their summary request succeeds; the range tab refreshes immediately.
  () => [props.linkId, props.surface, props.requestKey, trafficScope.value],
  () => {
    page.value = 1;
    load();
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  alive = false;
  generation++;
});

const number = (item) => Number(item || 0).toLocaleString("zh-CN");
const deviceLabel = {
  mobile: "手机",
  tablet: "平板",
  desktop: "电脑",
  unknown: "未知设备",
};
const classLabel = {
  normal: "正常访问",
  filtered: "正常访问",
  bot: "机器人",
  suspicious: "疑似异常",
  unclassified: "未分类",
  head: "HEAD 请求",
  prefetch: "预取",
};
const cookieLabel = (item) =>
  ({ recognized: "已识别", issued: "首次下发", disabled: "未启用" })[item] ||
  item ||
  "未知";
const surfaceLabel = (item) =>
  ({
    short_link: "短链接",
    audio_novel: "语音小说站",
    novel: "免费小说站",
    cover: "封面项目",
  })[item] || item;
const surfaceType = (item) =>
  item === "audio_novel"
    ? "warning"
    : item === "novel"
      ? "success"
      : item === "cover"
        ? "primary"
        : "info";
const classificationType = (item) =>
  item === "bot" || item === "head" || item === "prefetch"
    ? "info"
    : item === "suspicious" || item === "unclassified"
      ? "warning"
      : "success";
const visitorLabel = (item) => (item ? item.slice(0, 12) : "无 Cookie");
const eventTypeLabel = (row) =>
  row.event_type === "landing" ? "落地页访问" : "直接跳转";
const isIncluded = (row) =>
  row.classification === "normal" &&
  row.method === "GET" &&
  (row.event_type === "landing" ||
    (row.surface === "short_link" && row.event_type === "redirect"));
const duration = (item) => {
  if (item === null || item === undefined) return "未采集";
  const seconds = Math.round(Number(item) || 0);
  return `${Math.floor(seconds / 60)}分 ${seconds % 60}秒`;
};
const chapterLabel = (row) =>
  row.entry_chapter_number
    ? `第 ${row.entry_chapter_number} 章${row.entry_chapter_title ? ` · ${row.entry_chapter_title}` : ""}`
    : "未绑定入口章节";
function displayTime(item) {
  if (!item) return "—";
  try {
    return new Intl.DateTimeFormat("zh-CN", {
      dateStyle: "short",
      timeStyle: "medium",
      timeZone: props.timezone,
    }).format(new Date(item));
  } catch {
    return item;
  }
}
</script>

<style scoped>
.project-visits {
  min-width: 0;
  margin-top: 20px;
  overflow: hidden;
}
.panel-heading,
.heading-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.panel-heading p {
  margin: 6px 0 0;
  color: #909399;
}
.heading-actions {
  justify-content: flex-end;
  flex-wrap: wrap;
}
.behavior-lines,
.behavior-tags {
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  gap: 4px;
  margin-top: 4px;
}
.behavior-tags {
  align-items: center;
  flex-direction: row;
  flex-wrap: wrap;
}
.visitor {
  display: block;
  margin-bottom: 5px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.pagination {
  justify-content: flex-end;
  margin-top: 20px;
}
.conflict {
  display: block;
  margin-top: 4px;
  color: #c45656;
  font-size: 12px;
}
@media (max-width: 720px) {
  .panel-heading {
    align-items: flex-start;
    flex-direction: column;
  }
  .heading-actions {
    align-items: flex-start;
    justify-content: flex-start;
  }
  .project-visits {
    overflow-x: auto;
  }
}
</style>
