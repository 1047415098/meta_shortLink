<template>
  <el-card class="filter-card" shadow="never"
    ><el-form
      class="filters"
      label-position="top"
      @submit.prevent="emit('query')"
      ><el-form-item label="统计日期" class="date-filter"
        ><el-date-picker
          v-model="dateRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          format="YYYY/MM/DD"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          :shortcuts="dateShortcuts"
          :clearable="false" /></el-form-item
      ><el-form-item label="统计时区"
        ><el-select v-model="filters.tz" aria-label="统计时区"
          ><el-option label="上海 · UTC+8" value="Asia/Shanghai" /><el-option
            label="协调世界时 · UTC"
            value="UTC" /><el-option
            label="纽约"
            value="America/New_York" /><el-option
            label="洛杉矶"
            value="America/Los_Angeles" /></el-select></el-form-item
      ><el-form-item label="短链接"
        ><el-select
          v-model="filters.link_id"
          clearable
          filterable
          placeholder="全部链接"
          aria-label="短链接"
          ><el-option
            v-for="link in links"
            :key="link.id"
            :label="link.name"
            :value="String(link.id)" /></el-select></el-form-item
      ><el-form-item label="广告 ID"
        ><el-input
          v-model="filters.ad_id"
          :prefix-icon="Search"
          clearable
          placeholder="输入广告 ID"
          @keyup.enter="emit('query')" /></el-form-item
      ><el-form-item class="filter-actions"
        ><el-button
          type="primary"
          :icon="Search"
          :loading="busy"
          @click="emit('query')"
          >查询</el-button
        ><el-button @click="emit('reset')">重置</el-button></el-form-item
      ></el-form
    ></el-card
  >
</template>

<script setup>
import { computed } from "vue";
import { Search } from "@element-plus/icons-vue";
const props = defineProps({
  filters: { type: Object, required: true },
  links: { type: Array, default: () => [] },
  busy: Boolean,
});
const emit = defineEmits(["query", "reset"]);
const dateRange = computed({
  get: () => [props.filters.start, props.filters.end],
  set: (v) => {
    props.filters.start = v?.[0] || "";
    props.filters.end = v?.[1] || "";
  },
});
const dateShortcuts = [
  { text: "今天", value: () => [new Date(), new Date()] },
  {
    text: "最近 7 天",
    value: () => {
      const d = new Date();
      d.setDate(d.getDate() - 6);
      return [d, new Date()];
    },
  },
  {
    text: "最近 30 天",
    value: () => {
      const d = new Date();
      d.setDate(d.getDate() - 29);
      return [d, new Date()];
    },
  },
];
</script>

<style scoped>
.filter-card {
  margin-bottom: 22px;
}
.filter-card > :deep(.el-card__body) {
  padding: 18px 20px 2px;
}
.filters {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0 14px;
}
.filters :deep(.el-form-item) {
  flex: 1;
  min-width: 140px;
  margin-bottom: 16px;
}
.filters :deep(.el-form-item__label) {
  font-size: 12px;
  color: #606266;
  margin-bottom: 8px !important;
  line-height: 20px !important;
}
.filters .date-filter {
  flex: 1.8;
  min-width: 260px;
}
.filters :deep(.el-date-editor) {
  width: 100% !important;
  min-width: 0;
}
.filters :deep(.el-form-item__content) {
  min-width: 0;
}
.filters .filter-actions {
  flex: 0 0 auto;
  min-width: auto;
}
.filters .filter-actions :deep(.el-form-item__content) {
  flex-wrap: nowrap;
}
.filters :deep(.el-select) {
  width: 100%;
}
@media (max-width: 1200px) {
  .filters .date-filter {
    flex: 1.5;
    min-width: 260px;
  }
}
@media (max-width: 1200px) {
  .filters :deep(.el-form-item) {
    min-width: 155px;
  }
}
@media (max-width: 800px) {
  .filter-card > :deep(.el-card__body) {
    padding: 15px 14px 0;
  }
}
@media (max-width: 800px) {
  .filters {
    gap: 0 10px;
  }
}
@media (max-width: 800px) {
  .filters :deep(.el-form-item) {
    min-width: 120px;
    flex-basis: 40%;
  }
}
@media (max-width: 800px) {
  .filters .date-filter {
    min-width: 100%;
    flex-basis: 100%;
  }
}
@media (max-width: 800px) {
  .filters .filter-actions {
    flex-basis: 100%;
  }
}
</style>
