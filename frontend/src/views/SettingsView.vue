<template>
  <section>
    <PageHeader
      title="系统设置"
      context="工作空间配置"
      description="查看采集配置、数据保留与服务状态。"
    /><el-alert v-if="error" :title="error" type="error" />
    <section class="panel settings-panel">
      <h2>部署配置</h2>
      <p class="muted">这些配置由部署环境管理，修改后需重启服务。</p>
      <el-descriptions :column="1" border
        ><el-descriptions-item label="短链接域名">{{
          settings.public_base_url || "未配置"
        }}</el-descriptions-item
        ><el-descriptions-item label="默认时区">{{
          settings.timezone
        }}</el-descriptions-item
        ><el-descriptions-item label="Cookie 模式">{{
          settings.cookie_mode === "all" ? "已启用 · 匿名访客 Cookie" : "未启用"
        }}</el-descriptions-item
        ><el-descriptions-item label="明细保留天数">{{
          settings.retention_days
        }}</el-descriptions-item
        ><el-descriptions-item label="IP 地区数据库">{{
          settings.geo_enabled ? "已启用" : "未配置，地区显示未知"
        }}</el-descriptions-item></el-descriptions
      >
      <div class="footnote">
        Cookie 仅识别短域名下的浏览器，不读取 Facebook 或 WhatsApp 的
        Cookie。未配置地区数据库时不会虚构地理位置。
      </div>
    </section>
  </section>
</template>

<script setup>
import { ref, onMounted } from "vue";

import PageHeader from "../components/PageHeader.vue";

import { settings, loadSettings } from "../stores/settings";
const error = ref("");
onMounted(() => loadSettings().catch((e) => (error.value = e.message)));
</script>

<style scoped>
.settings-panel {
  max-width: 960px;
}
</style>
