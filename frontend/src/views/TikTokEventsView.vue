<template>
  <section>
    <PageHeader
      title="TikTok 事件记录"
      context="小说网页事件回传"
      description="查看文字与语音小说事件的保存、发送和接口接收状态。"
    >
      <el-button :icon="Refresh" :loading="busy" @click="load()"
        >刷新</el-button
      >
    </PageHeader>
    <el-alert
      class="notice"
      type="info"
      :closable="false"
      title="TikTok 是否归因：本系统未知，请到 TikTok Ads Manager 查看。"
    />
    <section class="panel">
      <el-form inline class="filters" @submit.prevent="load(true)">
        <el-form-item label="状态">
          <el-select
            v-model="filters.status"
            clearable
            placeholder="全部状态"
            style="width: 170px"
          >
            <el-option
              v-for="status in tiktokEventStatuses"
              :key="status"
              :value="status"
              :label="tiktokStatusLabels[status]"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="事件">
          <el-select
            v-model="filters.event_name"
            clearable
            placeholder="全部事件"
            style="width: 170px"
          >
            <el-option v-for="name in eventNames" :key="name" :value="name" />
          </el-select>
        </el-form-item>
        <el-form-item label="Pixel">
          <el-select
            v-model="filters.pixel_record_id"
            clearable
            placeholder="全部 Pixel"
            style="width: 230px"
          >
            <el-option
              v-for="pixel in pixels"
              :key="pixel.id"
              :value="pixel.id"
              :label="`${pixel.name} · ${pixel.pixel_code}`"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="投放链接 ID">
          <el-input
            v-model="filters.link_id"
            inputmode="numeric"
            placeholder="全部链接"
            style="width: 140px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" native-type="submit" :loading="busy"
            >查询</el-button
          >
          <el-button @click="reset">重置</el-button>
        </el-form-item>
      </el-form>
      <el-alert
        v-if="error"
        class="notice"
        type="error"
        :closable="false"
        :title="error"
      />
      <el-table
        :data="data.items"
        v-loading="busy"
        row-key="id"
        empty-text="暂无符合条件的 TikTok 事件"
      >
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="event-detail">
              <div>
                <b>事件编号</b><span>{{ row.id }}</span>
              </div>
              <div>
                <b>访问编号</b><span>{{ row.visit_id }}</span>
              </div>
              <div>
                <b>投放链接</b><span>{{ row.link_id }}</span>
              </div>
              <div>
                <b>小说编号</b><span>{{ row.novel_id || "—" }}</span>
              </div>
              <div>
                <b>语音小说编号</b><span>{{ row.audio_novel_id || "—" }}</span>
              </div>
              <div>
                <b>Pixel Code</b><span>{{ row.pixel_code }}</span>
              </div>
              <div>
                <b>HTTP 状态</b><span>{{ row.http_status || "—" }}</span>
              </div>
              <div>
                <b>业务码</b><span>{{ row.business_code || "—" }}</span>
              </div>
              <div>
                <b>请求编号</b><span>{{ row.request_id || "—" }}</span>
              </div>
              <div>
                <b>入队时间</b><span>{{ localTimestamp(row.created_at) }}</span>
              </div>
              <div>
                <b>更新时间</b><span>{{ localTimestamp(row.updated_at) }}</span>
              </div>
              <div class="wide">
                <b>最近错误</b><span>{{ row.last_error || "—" }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="发生时间" width="185">
          <template #default="{ row }">{{
            localTimestamp(row.event_time)
          }}</template>
        </el-table-column>
        <el-table-column label="Pixel" min-width="190">
          <template #default="{ row }">
            <b>{{ row.pixel_name }}</b>
            <div class="muted">{{ row.pixel_code }}</div>
          </template>
        </el-table-column>
        <el-table-column label="事件" min-width="190">
          <template #default="{ row }">
            <b>{{ row.event_name }}</b>
            <div class="subline">
              <el-tag size="small" type="info">{{ contentLabel(row) }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="150">
          <template #default="{ row }">
            <el-tag :type="tiktokStatusType(row.status)">
              {{ tiktokStatusLabels[row.status] || row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="attempts" label="尝试次数" width="100" />
        <el-table-column
          prop="last_error"
          label="最近错误"
          min-width="240"
          show-overflow-tooltip
        >
          <template #default="{ row }">{{ row.last_error || "—" }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="['failed', 'retry'].includes(row.status)"
              link
              type="primary"
              :loading="retrying === row.id"
              :disabled="Boolean(retrying)"
              @click="retry(row)"
              >重试</el-button
            >
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-model:current-page="page"
        class="pagination"
        :page-size="data.page_size || 50"
        :total="data.total"
        layout="total, prev, pager, next"
        @current-change="load()"
      />
    </section>
    <p class="footnote">
      “TikTok 已接收”仅代表 Events API
      返回成功；事件是否匹配到用户并计入广告归因，以 TikTok Ads Manager
      最终结果为准。
    </p>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus/es/components/message/index";
import { Refresh } from "@element-plus/icons-vue";

import PageHeader from "../components/PageHeader.vue";
import {
  listTikTokEvents,
  listTikTokPixels,
  retryTikTokEvent,
} from "../api/tiktok.js";
import {
  localTimestamp,
  tiktokEventStatuses,
  tiktokStatusLabels,
  tiktokStatusType,
} from "../utils/tiktok.js";

const eventNames = [
  "StartReading",
  "StartListening",
  "ViewContent",
  "PageView",
];
const filters = reactive({
  status: "",
  event_name: "",
  pixel_record_id: "",
  link_id: "",
});
const page = ref(1);
const data = ref({ items: [], total: 0, page_size: 50 });
const pixels = ref([]);
const busy = ref(false);
const error = ref("");
const retrying = ref(null);

function contentLabel(row) {
  // The durable content ID keeps this label accurate after visit retention cleanup.
  return row.audio_novel_id ? "语音小说事件" : "免费小说事件";
}

async function load(resetPage = false) {
  if (resetPage) page.value = 1;
  busy.value = true;
  error.value = "";
  try {
    data.value = await listTikTokEvents({ ...filters, page: page.value });
  } catch (loadError) {
    error.value = loadError.message;
  } finally {
    busy.value = false;
  }
}

function reset() {
  Object.assign(filters, {
    status: "",
    event_name: "",
    pixel_record_id: "",
    link_id: "",
  });
  load(true);
}

async function retry(row) {
  retrying.value = row.id;
  try {
    await retryTikTokEvent(row.id);
    ElMessage.success("TikTok 事件已重新进入发送队列");
    await load();
  } catch (retryError) {
    ElMessage.error(retryError.message);
  } finally {
    retrying.value = null;
  }
}

onMounted(async () => {
  try {
    pixels.value = await listTikTokPixels();
  } catch (pixelError) {
    error.value = pixelError.message;
  }
  await load();
});
</script>

<style scoped>
.event-detail {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 28px;
  padding: 18px 46px;
}
.event-detail div {
  display: grid;
  grid-template-columns: 95px minmax(0, 1fr);
  gap: 12px;
}
.event-detail .wide {
  grid-column: 1 / -1;
}
.event-detail span {
  overflow-wrap: anywhere;
}
.subline {
  margin-top: 6px;
}
@media (max-width: 760px) {
  .event-detail {
    grid-template-columns: 1fr;
    padding: 14px;
  }
  .event-detail .wide {
    grid-column: auto;
  }
}
</style>
