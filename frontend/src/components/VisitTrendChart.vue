<template>
  <div
    ref="host"
    class="trend-chart"
    role="img"
    aria-label="所选日期的网站访问、手动咨询和自动跳转趋势"
  ></div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, watch } from "vue";
import { use, init } from "echarts/core";
import { LineChart } from "echarts/charts";
import {
  TooltipComponent,
  GridComponent,
  LegendComponent,
} from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
use([
  LineChart,
  TooltipComponent,
  GridComponent,
  LegendComponent,
  CanvasRenderer,
]);
const props = defineProps({ data: { type: Array, default: () => [] } });
const host = ref();
let chart, observer;
function render() {
  if (!chart) return;
  chart.setOption({
    color: ["#409eff", "#22a06b", "#e6a23c"],
    tooltip: { trigger: "axis" },
    legend: { bottom: 0, icon: "circle" },
    grid: { top: 20, left: 45, right: 20, bottom: 58 },
    xAxis: {
      type: "category",
      data: props.data.map((x) => x.date),
      boundaryGap: false,
      axisLine: { lineStyle: { color: "#dbe4e7" } },
      axisLabel: { color: "#71838a" },
    },
    yAxis: {
      type: "value",
      minInterval: 1,
      splitLine: { lineStyle: { color: "#edf1f2" } },
      axisLabel: { color: "#71838a" },
    },
    series: [
      {
        name: "用户端网站访问",
        type: "line",
        smooth: false,
        data: props.data.map((x) => x.landing_views || 0),
        showSymbol: false,
        areaStyle: { opacity: 0.08 },
        lineStyle: { width: 3 },
      },
      {
        name: "手动咨询点击",
        type: "line",
        smooth: false,
        data: props.data.map((x) => x.whatsapp_clicks || 0),
        showSymbol: false,
        lineStyle: { type: "dashed" },
      },
      {
        name: "自动跳转",
        type: "line",
        smooth: false,
        data: props.data.map((x) => x.auto_redirects || 0),
        showSymbol: false,
        lineStyle: { type: "dotted" },
      },
    ],
  });
}
onMounted(() => {
  chart = init(host.value);
  observer = new ResizeObserver(() => chart.resize());
  observer.observe(host.value);
  render();
});
watch(() => props.data, render, { deep: true });
onBeforeUnmount(() => {
  observer?.disconnect();
  chart?.dispose();
});
</script>

<style scoped>
.trend-chart {
  height: 300px;
  width: 100%;
}
</style>
