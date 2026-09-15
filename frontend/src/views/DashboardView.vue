<template>
  <section class="dashboard-page">
    <PageHeader title="数据总览"
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
      <div class="metric-grid">
        <el-card
          v-for="(card, i) in cards"
          :key="card[0]"
          class="metric"
          :class="'metric-' + i"
          shadow="never"
          ><div class="metric-header">
            <span>{{ card[0] }}</span
            ><span class="metric-icon"
              ><el-icon><component :is="metricIcons[i]" /></el-icon
            ></span>
          </div>
          <el-statistic :value="Number(card[1] || 0)" group-separator="," />
          <div v-if="card[0] === '咨询点击总数'" class="consultation-breakdown">
            <div>
              <span>手动咨询点击</span
              ><strong>{{ fmt(summary.whatsapp_clicks) }}</strong>
            </div>
            <div>
              <span>自动跳转咨询</span
              ><strong>{{ fmt(summary.auto_redirects) }}</strong>
            </div>
          </div>
          <div class="metric-caption">
            <span class="metric-dot"></span>{{ card[2] }}
          </div></el-card
        >
      </div>
      <section class="panel chart-panel">
        <div class="panel-heading">
          <div>
            <h2>网站访问与咨询趋势</h2>
            <small class="muted">手动咨询与倒计时自动跳转分别统计</small>
            <p>
              按 {{ filters.tz }} 展示；咨询归入打开页面的日期，同次访问只计一次
            </p>
          </div>
          <el-tag type="info" effect="plain"
            >{{ filters.start === filters.end ? "每小时" : "每日" }}趋势</el-tag
          >
        </div>
        <TrendChart
          v-if="analytics?.summary?.landing_views > 0"
          :data="analytics.trends"
        /><el-empty
          v-else
          description="所选时间内暂无访问记录"
          :image-size="75"
        />
      </section>
      <div class="breakdown-grid">
        <section
          v-for="[key, label] in [
            ['devices', '设备分布'],
            ['countries', '地区分布'],
            ['sources', '来源分布'],
          ]"
          :key="key"
          class="panel"
        >
          <h2>{{ label }}</h2>
          <div v-if="analytics?.[key]?.length" class="distribution">
            <div v-for="item in analytics[key].slice(0, 8)" :key="item.name">
              <div>
                <span>{{
                  key === "devices"
                    ? deviceLabel[item.name] || item.name
                    : item.name === "unknown"
                      ? "未知地区"
                      : item.name || "未知"
                }}</span
                ><b>{{ fmt(item.count) }}</b>
              </div>
              <el-progress
                :percentage="
                  Math.round(
                    (item.count /
                      Math.max(
                        analytics[key].reduce((n, x) => n + x.count, 0),
                        1,
                      )) *
                      100,
                  )
                "
                :show-text="false"
                :stroke-width="6"
              />
            </div>
          </div>
          <el-empty v-else description="暂无分布数据" :image-size="48" />
        </section>
      </div>
      <div class="quality-strip">
        <span
          >已知机器人 <b>{{ fmt(summary.bot) }}</b></span
        ><span
          >未分类 <b>{{ fmt(summary.unclassified) }}</b></span
        ><span
          >无访客标识 <b>{{ fmt(summary.no_cookie) }}</b></span
        ><span
          >HEAD / 预取
          <b>{{ fmt((summary.head || 0) + (summary.prefetch || 0)) }}</b></span
        >
      </div>
      <div class="footnote">
        统计说明：过滤后点击不等于已证实的真人；估算访客受
        Cookie、设备和浏览器限制。点击不能证明 WhatsApp 已打开或已发送消息。
      </div>
      <el-alert
        v-if="analytics?.health?.write_failures"
        :title="
          '本次服务启动后存在访问写入失败记录：' +
          analytics.health.write_failures +
          '。请检查服务告警，报表可能存在缺口。'
        "
        type="warning"
        :closable="false"
      />
    </section>
  </section>
</template>

<script setup>
import { computed, defineAsyncComponent } from "vue";

import PageHeader from "../components/PageHeader.vue";
import {
  Link,
  Refresh,
  Download,
  User,
  Pointer,
  View,
} from "@element-plus/icons-vue";

import AnalyticsFilter from "../components/AnalyticsFilter.vue";
import { useReport } from "../composables/useReport";
import { getAnalytics } from "../api/analytics";
import { fillTrend } from "../utils";

const {
  filters,
  data: analytics,
  links,
  busy,
  error,
  load,
  resetFilters,
  exportCSV,
} = useReport(async (f) => {
  const a = await getAnalytics(f);
  a.trends = fillTrend(a.trends || [], f);
  return a;
});
const TrendChart = defineAsyncComponent(
  () => import("../components/VisitTrendChart.vue"),
);
const deviceLabel = {
  mobile: "手机",
  tablet: "平板",
  desktop: "电脑",
  unknown: "未知设备",
};
const metricIcons = [Link, View, Pointer, User];
const summary = computed(() => analytics.value?.summary || {});
const fmt = (n) => Number(n || 0).toLocaleString("zh-CN");
const cards = computed(() => [
  [
    "可用短链接总数",
    summary.value.available_links,
    "已启用且未过期，不受访问筛选影响",
  ],
  [
    "用户端网站访问",
    summary.value.landing_views,
    "维度 1 · 返回落地页，排除已识别异常访问",
  ],
  [
    "咨询点击总数",
    Number(summary.value.whatsapp_clicks || 0) +
      Number(summary.value.auto_redirects || 0),
    "总数 = 手动咨询 + 自动跳转；分项按访问去重",
  ],
  ["估算独立访客", summary.value.unique, "按所选范围去重"],
]);
</script>

<style scoped>
.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 18px;
  margin-bottom: 22px;
}
.metric :deep(.el-card__body) {
  padding: 20px 22px 18px;
}
.metric-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #606266;
  font-size: 13px;
  margin-bottom: 9px;
}
.metric-icon {
  height: 34px;
  width: 34px;
  border-radius: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #ecf5ff;
  color: #409eff;
  font-size: 19px;
}
.metric-1 .metric-icon {
  background: #f0f9eb;
  color: #67c23a;
}
.metric-2 .metric-icon {
  background: #f1edff;
  color: #9570e8;
}
.metric-3 .metric-icon {
  background: #fdf6ec;
  color: #e6a23c;
}
.metric :deep(.el-statistic__content) {
  font-size: 32px;
  font-weight: 650;
  color: #253044;
  font-variant-numeric: tabular-nums;
  line-height: 1.5;
  letter-spacing: -0.5px;
}
.metric-caption {
  border-top: 1px solid #f2f4f7;
  padding-top: 13px;
  margin-top: 13px;
  color: #a8abb2;
  font-size: 11px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.metric-dot {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #bbc5d3;
}
.breakdown-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
}
.distribution > div {
  margin: 20px 0;
}
.distribution > div > div:first-child {
  display: flex;
  justify-content: space-between;
  color: #606266;
  font-size: 13px;
  margin-bottom: 10px;
}
.distribution b {
  font-size: 12px;
  color: #606266;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
.distribution :deep(.el-progress-bar__outer) {
  background: #f2f6fc;
}
.distribution :deep(.el-progress-bar__inner) {
  background: #8cc5ff;
}
.quality-strip {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
  padding: 15px 18px;
  background: #eef2f7;
  border-radius: 7px;
  font-size: 12px;
  color: #909399;
  margin-bottom: 15px;
}
.quality-strip b {
  color: #606266;
  margin-left: 8px;
  font-weight: 500;
}
@media (max-width: 1200px) {
  .metric-grid {
    gap: 12px;
  }
}
@media (max-width: 1200px) {
  .metric :deep(.el-card__body) {
    padding: 17px;
  }
}
@media (max-width: 1200px) {
  .metric-caption {
    font-size: 10px;
  }
}
@media (max-width: 1200px) {
  .breakdown-grid {
    gap: 14px;
  }
}
@media (max-width: 800px) {
  .metric-grid {
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
}
@media (max-width: 800px) {
  .metric :deep(.el-card__body) {
    padding: 13px;
  }
}
@media (max-width: 800px) {
  .metric-header {
    font-size: 11px;
  }
}
@media (max-width: 800px) {
  .metric-icon {
    width: 27px;
    height: 27px;
    font-size: 15px;
  }
}
@media (max-width: 800px) {
  .metric :deep(.el-statistic__content) {
    font-size: 28px;
  }
}
@media (max-width: 800px) {
  .metric-caption {
    line-height: 1.6;
    margin-top: 8px;
    padding-top: 10px;
  }
}
@media (max-width: 800px) {
  .breakdown-grid {
    grid-template-columns: 1fr;
    gap: 0;
  }
}
@media (max-width: 800px) {
  .quality-strip {
    gap: 14px;
    font-size: 11px;
  }
}
.consultation-breakdown {
  display: grid;
  gap: 10px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #f2f4f7;
}
.consultation-breakdown > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
  color: #606266;
}
.consultation-breakdown strong {
  color: #253044;
  font-size: 18px;
  font-variant-numeric: tabular-nums;
}
</style>
