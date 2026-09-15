<template>
  <section>
    <PageHeader
      title="Meta 事件记录"
      context="广告咨询回传"
      description="核对浏览、手动咨询、自动跳转和测试事件的发送结果。"
    >
      <el-button @click="router.push({ name: 'meta-connections' })"
        >管理连接</el-button
      ><el-button :icon="Refresh" :loading="busy" @click="load()"
        >刷新</el-button
      >
    </PageHeader>
    <el-alert
      class="notice"
      title="只有同时具备有效 fbclid 和广告 ID 的真实广告点击才会回传；Meta 已接收不代表最终完成广告归因。"
      type="info"
      :closable="false"
    />
    <el-alert
      v-if="connectionError"
      class="notice"
      :title="connectionError"
      type="error"
      :closable="false"
    />
    <section class="panel">
      <el-form inline class="filters" @submit.prevent="load(true)">
        <el-form-item label="连接"
          ><el-select
            v-model="filters.connection_id"
            clearable
            placeholder="全部连接"
            style="width: 240px"
            ><el-option
              v-for="connection in connections"
              :key="connection.id"
              :value="connection.id"
              :label="connection.name" /></el-select
        ></el-form-item>
        <el-form-item label="状态"
          ><el-select
            v-model="filters.status"
            clearable
            placeholder="全部状态"
            style="width: 160px"
            ><el-option
              v-for="status in eventStatuses"
              :key="status"
              :value="status"
              :label="statusLabels[status]" /></el-select
        ></el-form-item>
        <el-form-item label="Pixel"
          ><el-select
            v-model="filters.pixel_record_id"
            clearable
            placeholder="全部 Pixel"
            style="width: 200px"
            ><el-option
              v-for="pixel in pixels"
              :key="pixel.id"
              :value="pixel.id"
              :label="pixel.name + ' · ' + pixel.pixel_id" /></el-select
        ></el-form-item>
        <el-form-item label="事件"
          ><el-select
            v-model="filters.event_name"
            clearable
            placeholder="全部事件"
            style="width: 190px"
            ><el-option
              v-for="name in eventNames"
              :key="name"
              :value="name" /></el-select
        ></el-form-item>
        <el-form-item
          ><el-button type="primary" native-type="submit" :loading="busy"
            >查询</el-button
          ><el-button @click="reset">重置</el-button></el-form-item
        >
      </el-form>
      <el-alert
        v-if="error"
        class="notice"
        :title="error"
        type="error"
        :closable="false"
      />
      <el-table
        :data="data.items"
        v-loading="busy"
        empty-text="暂无符合条件的事件。为短链接启用回传后，真实广告咨询会出现在这里。"
        row-key="id"
      >
        <el-table-column type="expand"
          ><template #default="{ row }"
            ><div class="event-detail">
              <div>
                <b>事件名称</b><span>{{ row.event_name }}</span>
              </div>
              <div>
                <b>事件编号</b><span>{{ row.id }}</span>
              </div>
              <div>
                <b>Pixel</b
                ><span
                  >{{ row.pixel_id || "—" }}（记录
                  {{ row.pixel_record_id || "—" }}）</span
                >
              </div>
              <div>
                <b>访问编号</b
                ><span>{{ row.visit_id || "测试事件，无访问编号" }}</span>
              </div>
              <div>
                <b>Meta 接收数</b><span>{{ row.events_received ?? "—" }}</span>
              </div>
              <div>
                <b>Meta 追踪编号</b><span>{{ row.fbtrace_id || "—" }}</span>
              </div>
              <div>
                <b>入队时间</b><span>{{ timestamp(row.created_at) }}</span>
              </div>
              <div>
                <b>更新时间</b><span>{{ timestamp(row.updated_at) }}</span>
              </div>
              <div class="wide">
                <b>最近错误</b><span>{{ row.last_error || "—" }}</span>
              </div>
            </div></template
          ></el-table-column
        >
        <el-table-column label="发生时间" width="185"
          ><template #default="{ row }">{{
            timestamp(row.event_time)
          }}</template></el-table-column
        >
        <el-table-column prop="connection_name" label="连接" min-width="180" />
        <el-table-column label="事件" min-width="195"
          ><template #default="{ row }"
            ><b>{{ row.event_name }}</b>
            <div class="subline">
              <el-tag size="small" :type="row.is_test ? 'warning' : 'info'">{{
                row.is_test
                  ? "测试事件"
                  : row.event_name === "PageView"
                    ? "浏览事件"
                    : row.event_name === "WhatsAppAutoRedirect"
                      ? "自动跳转"
                      : "手动咨询"
              }}</el-tag>
            </div></template
          ></el-table-column
        >
        <el-table-column label="状态" width="125"
          ><template #default="{ row }"
            ><el-tag :type="statusType(row.status)">{{
              statusLabels[row.status] || row.status
            }}</el-tag></template
          ></el-table-column
        >
        <el-table-column prop="attempts" label="尝试次数" width="100" />
        <el-table-column
          prop="last_error"
          label="最近错误"
          min-width="240"
          show-overflow-tooltip
          ><template #default="{ row }">{{
            row.last_error || "—"
          }}</template></el-table-column
        >
        <el-table-column label="操作" width="95" fixed="right"
          ><template #default="{ row }"
            ><el-button
              v-if="['failed', 'retry'].includes(row.status)"
              link
              type="primary"
              :disabled="!!retrying"
              :loading="retrying === row.id"
              @click="retry(row)"
              >重试</el-button
            ><span v-else class="muted">—</span></template
          ></el-table-column
        >
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
      时间按当前电脑时区显示。只有失败或等待重试的事件可以重新入队，已过期事件不能重试。展开记录可查看
      Meta 追踪编号；记录中不展示凭据与匹配信息。
    </p>
  </section>
</template>

<script setup>
import { ref, reactive, onMounted, onBeforeUnmount, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus/es/components/message/index";
import { Refresh } from "@element-plus/icons-vue";
import PageHeader from "../components/PageHeader.vue";
import {
  listConnections,
  listPixels,
  listMetaEvents,
  retryMetaEvent,
} from "../api/meta";
import { statusLabels, statusType, timestamp } from "../utils/meta";
const route = useRoute(),
  router = useRouter();
const eventStatuses = [
  "pending",
  "processing",
  "succeeded",
  "retry",
  "failed",
  "expired",
  "skipped",
];
// 查询选项与后端真实、测试事件白名单保持一致。
const eventNames = [
  "PageView",
  "WhatsAppConsultClick",
  "WhatsAppAutoRedirect",
  "Contact",
];
const connectionQuery = () => Number(route.query.connection_id) || "";
const statusQuery = () =>
  eventStatuses.includes(route.query.status) ? route.query.status : "";
const pageQuery = () => Math.max(1, parseInt(route.query.page) || 1);
const pixelQuery = () => Number(route.query.pixel_record_id) || "";
const eventQuery = () =>
  eventNames.includes(route.query.event_name) ? route.query.event_name : "";
const filters = reactive({
  connection_id: connectionQuery(),
  status: statusQuery(),
  pixel_record_id: pixelQuery(),
  event_name: eventQuery(),
});
const page = ref(pageQuery()),
  data = ref({ items: [], total: 0, page_size: 50 }),
  connections = ref([]),
  pixels = ref([]);
const busy = ref(false),
  error = ref(""),
  connectionError = ref(""),
  retrying = ref(null);
let generation = 0;
async function load(resetPage = false, sync = true) {
  const run = ++generation;
  if (resetPage) page.value = 1;
  busy.value = true;
  error.value = "";
  data.value = { items: [], total: 0, page_size: data.value.page_size || 50 };
  try {
    if (sync)
      await router.replace({
        query: {
          ...(filters.connection_id
            ? { connection_id: String(filters.connection_id) }
            : {}),
          ...(filters.status ? { status: filters.status } : {}),
          ...(filters.pixel_record_id
            ? { pixel_record_id: String(filters.pixel_record_id) }
            : {}),
          ...(filters.event_name ? { event_name: filters.event_name } : {}),
          page: String(page.value),
        },
      });
    const result = await listMetaEvents({ ...filters, page: page.value });
    if (run === generation) data.value = result;
  } catch (e) {
    if (run === generation) error.value = e.message;
  } finally {
    if (run === generation) busy.value = false;
  }
}
function reset() {
  filters.connection_id = "";
  filters.status = "";
  filters.pixel_record_id = "";
  filters.event_name = "";
  load(true);
}
async function retry(row) {
  if (retrying.value) return;
  retrying.value = row.id;
  try {
    await retryMetaEvent(row.id);
    ElMessage.success("事件已重新加入发送队列");
    await load();
  } catch (e) {
    ElMessage.error(e.message);
  } finally {
    retrying.value = null;
  }
}
watch(
  () => route.query,
  () => {
    if (
      connectionQuery() === filters.connection_id &&
      statusQuery() === filters.status &&
      pixelQuery() === filters.pixel_record_id &&
      eventQuery() === filters.event_name &&
      pageQuery() === page.value
    )
      return;
    filters.connection_id = connectionQuery();
    filters.status = statusQuery();
    filters.pixel_record_id = pixelQuery();
    filters.event_name = eventQuery();
    page.value = pageQuery();
    load(false, false);
  },
);
onMounted(async () => {
  await Promise.allSettled([
    load(false, false),
    listConnections()
      .then((result) => {
        connections.value = result;
      })
      .catch((e) => {
        connectionError.value = e.message;
      }),
    listPixels()
      .then((result) => {
        pixels.value = result;
      })
      .catch((e) => {
        connectionError.value = e.message;
      }),
  ]);
});
onBeforeUnmount(() => {
  generation++;
});
</script>

<style scoped>
.filters {
  margin-bottom: 6px;
}
.subline {
  margin-top: 7px;
}
.event-detail {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px 28px;
  padding: 20px 34px;
  background: #f8fafc;
}
.event-detail > div {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.event-detail b {
  font-size: 12px;
  color: #909399;
}
.event-detail span {
  font-size: 13px;
  overflow-wrap: anywhere;
}
.wide {
  grid-column: 1 / -1;
}
@media (max-width: 650px) {
  .event-detail {
    grid-template-columns: 1fr;
    padding: 15px;
  }
  .filters :deep(.el-form-item) {
    display: flex;
    margin-right: 0;
  }
}
</style>
