<template>
  <div class="page-heading">
    <div>
      <div class="heading-context">
        <span class="context-line"></span>{{ context }}
      </div>
      <h1>{{ title }}</h1>
      <p class="muted">{{ description }}</p>
    </div>
    <div class="heading-actions"><slot /></div>
  </div>
</template>

<script setup>
defineProps({
  title: String,
  context: { type: String, default: "投放数据分析" },
  description: {
    type: String,
    default: "集中查看访问表现，为下一次投放提供依据。",
  },
});
</script>

<style scoped>
.page-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 25px;
  gap: 16px;
}
.heading-context {
  display: flex;
  align-items: center;
  gap: 7px;
  color: #909399;
  font-size: 11px;
  margin-bottom: 10px;
}
.context-line {
  height: 10px;
  width: 3px;
  background: #409eff;
  border-radius: 2px;
}
.page-heading .muted {
  margin: 0;
}
.heading-actions {
  display: flex;
  gap: 10px;
  flex-shrink: 0;
}
.heading-actions :deep(.el-button + .el-button) {
  margin: 0;
}
@media (max-width: 800px) {
  .page-heading {
    align-items: flex-start;
    flex-wrap: wrap;
    margin-bottom: 20px;
  }
}
@media (max-width: 800px) {
  .page-heading .muted {
    font-size: 12px;
  }
}
@media (max-width: 800px) {
  .heading-actions :deep(.el-button) {
    height: 32px;
    font-size: 12px;
  }
}
</style>
