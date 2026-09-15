<template>
  <section>
    <PageHeader
      title="来源诊断"
      context="Meta 参数工具"
      description="生成显式 Meta ID 参数并检查投放链接；工具只解析文本，不访问目标网址。"
    />
    <section class="panel">
      <el-form label-position="top" @submit.prevent="inspect"
        ><el-form-item label="短链接地址"
          ><el-input
            v-model="baseURL"
            placeholder="https://example.com/your-code"
        /></el-form-item>
        <div class="grid">
          <el-form-item label="来源"
            ><el-input
              v-model="params.utm_source"
              placeholder="facebook" /></el-form-item
          ><el-form-item label="Meta 版位来源宏"
            ><el-input v-model="params.site_source_name" /></el-form-item
          ><el-form-item label="Campaign ID"
            ><el-input v-model="params.campaign_id" /></el-form-item
          ><el-form-item label="Ad set ID"
            ><el-input v-model="params.adset_id" /></el-form-item
          ><el-form-item label="Ad ID"
            ><el-input v-model="params.ad_id"
          /></el-form-item>
        </div>
        <el-button @click="generate">生成链接</el-button
        ><el-button type="primary" native-type="submit" :loading="busy"
          >检查链接</el-button
        ><el-input
          v-model="url"
          type="textarea"
          :rows="3"
          class="output"
          placeholder="生成或粘贴待检查链接" /></el-form
      ><el-alert
        v-if="error"
        class="notice"
        type="error"
        :closable="false"
        :title="error"
      />
      <div v-if="result" class="result">
        <el-result
          :icon="result.valid ? 'success' : 'warning'"
          :title="result.valid ? '参数有效' : '参数需要修正'"
        /><el-descriptions border :column="2"
          ><el-descriptions-item label="来源">{{
            result.source || "—"
          }}</el-descriptions-item
          ><el-descriptions-item label="Campaign">{{
            result.campaign_id || "—"
          }}</el-descriptions-item
          ><el-descriptions-item label="Ad set">{{
            result.adset_id || "—"
          }}</el-descriptions-item
          ><el-descriptions-item label="Ad">{{
            result.ad_id || "—"
          }}</el-descriptions-item></el-descriptions
        ><el-alert
          v-for="issue in result.issues || []"
          :key="issue"
          class="notice"
          type="warning"
          :closable="false"
          :title="issue"
        />
      </div>
    </section>
  </section>
</template>
<script setup>
import { ref, reactive } from "vue";
import { ElMessage } from "element-plus/es/components/message/index";
import PageHeader from "../components/PageHeader.vue";
import { inspectMetaSource } from "../api/meta";
import { buildMetaTrackingURL } from "../utils/meta";
const baseURL = ref(""),
  url = ref(""),
  params = reactive({
    utm_source: "facebook",
    site_source_name: "{{site_source_name}}",
    campaign_id: "",
    adset_id: "",
    ad_id: "",
  }),
  busy = ref(false),
  error = ref(""),
  result = ref(null);
function generate() {
  try {
    url.value = buildMetaTrackingURL(baseURL.value, params);
  } catch (e) {
    ElMessage.warning(
      e.message === "链接包含凭证信息，请先移除后重试"
        ? e.message
        : "请输入有效的完整链接",
    );
  }
}
async function inspect() {
  busy.value = true;
  error.value = "";
  try {
    result.value = await inspectMetaSource(url.value);
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
</script>
<style scoped>
.grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 0 18px;
}
.output {
  margin-top: 18px;
}
.result {
  margin-top: 20px;
}
@media (max-width: 700px) {
  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
