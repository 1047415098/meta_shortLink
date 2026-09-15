<template>
  <section>
    <PageHeader
      title="日志管理"
      context="请求记录与排查"
      description="查看每次访客请求的参数、响应与处理耗时。"
    />
    <section class="panel request-logs">
      <div class="panel-heading">
        <div>
          <h2>访客接口日志</h2>
          <p class="muted">
            仅记录访客端：短链接访问与咨询提交 · 保留 7 天 · 时间为北京时间
          </p>
        </div>
        <el-button :loading="busy" @click="load()">刷新日志</el-button>
      </div>
      <el-alert
        title="新请求的 URL 参数 token 加密保存，管理员查看详情时显示原值。密码、Cookie 及其他敏感字段保持脱敏；JSON / 表单正文最多保存 32 KB。"
        type="info"
        :closable="false"
      />
      <el-form inline class="log-filters" @submit.prevent="load(true)">
        <el-form-item label="日期"
          ><el-date-picker
            v-model="range"
            type="daterange"
            value-format="YYYY-MM-DD"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            style="width: 260px"
        /></el-form-item>
        <el-form-item label="路径"
          ><el-input
            v-model="filters.path"
            placeholder="例如 /123 或 /123/contact"
            clearable
            style="width: 250px"
        /></el-form-item>
        <el-form-item label="方法"
          ><el-select
            v-model="filters.method"
            clearable
            placeholder="全部"
            style="width: 110px"
            ><el-option
              v-for="m in [
                'GET',
                'POST',
                'PATCH',
                'PUT',
                'DELETE',
                'HEAD',
                'OPTIONS',
              ]"
              :key="m"
              :value="m"
              :label="m" /></el-select
        ></el-form-item>
        <el-form-item label="状态码"
          ><el-input
            v-model="filters.status"
            placeholder="例如 200"
            clearable
            style="width: 115px" /></el-form-item
        ><el-form-item
          ><el-button type="primary" native-type="submit"
            >查询</el-button
          ></el-form-item
        >
      </el-form>
      <el-alert v-if="error" :title="error" type="error" :closable="false" />
      <el-table
        :data="data.items"
        v-loading="busy"
        empty-text="暂无符合条件的请求日志"
      >
        <el-table-column label="时间" width="180"
          ><template #default="{ row }">{{
            time(row.occurred_at)
          }}</template></el-table-column
        >
        <el-table-column label="访客动作" width="120"
          ><template #default="{ row }"
            ><el-tag
              :type="row.path.split('/').length === 3 ? 'success' : 'info'"
              >{{
                row.path.split("/").length === 3
                  ? row.trigger === "auto"
                    ? "定时跳转"
                    : "咨询提交"
                  : "网站入口访问"
              }}</el-tag
            ></template
          ></el-table-column
        >
        <el-table-column prop="method" label="方法" width="80" />
        <el-table-column
          prop="path"
          label="请求路径"
          min-width="220"
          show-overflow-tooltip
        />
        <el-table-column prop="client_ip" label="来源 IP" min-width="145" />
        <el-table-column label="状态" width="80"
          ><template #default="{ row }"
            ><el-tag
              :type="
                row.status >= 400
                  ? 'danger'
                  : row.status >= 300
                    ? 'warning'
                    : 'success'
              "
              >{{ row.status }}</el-tag
            ></template
          ></el-table-column
        >
        <el-table-column label="耗时" width="100"
          ><template #default="{ row }"
            >{{ Number(row.duration_ms).toFixed(1) }} ms</template
          ></el-table-column
        >
        <el-table-column label="响应大小" width="110"
          ><template #default="{ row }"
            >{{ row.response_bytes }} B</template
          ></el-table-column
        >
        <el-table-column label="操作" width="85" fixed="right"
          ><template #default="{ row }"
            ><el-button link type="primary" @click="show(row)"
              >详情</el-button
            ></template
          ></el-table-column
        >
      </el-table>
      <div class="log-pagination">
        <span class="muted">共 {{ data.total }} 条</span
        ><el-pagination
          v-model:current-page="page"
          :page-size="50"
          :total="data.total"
          layout="prev, pager, next"
          @current-change="load()"
        />
      </div>
    </section>
    <el-dialog
      v-model="opened"
      title="请求详情"
      width="min(900px, 94vw)"
      class="log-detail"
      @closed="clearClosedDetail"
    >
      <template v-if="detail">
        <!-- 旧日志没有原值；解密异常时保留脱敏详情，避免呈现错误的 token。 -->
        <el-alert
          v-if="detail.query_token_state === 'not_saved'"
          title="这条记录未保存 token 原值，无法还原。请重新访问原链接生成新记录；原值保存上限为 32 KB。"
          type="info"
          :closable="false"
        />
        <el-alert
          v-else-if="detail.query_token_state === 'unavailable'"
          title="token 暂时无法解密，请检查服务端密钥。其余请求详情仍可查看。"
          type="warning"
          :closable="false"
        />
        <el-descriptions :column="2" border
          ><el-descriptions-item label="请求编号" :span="2">{{
            detail.id
          }}</el-descriptions-item
          ><el-descriptions-item label="时间">{{
            time(detail.occurred_at)
          }}</el-descriptions-item
          ><el-descriptions-item label="来源 IP">{{
            detail.client_ip
          }}</el-descriptions-item
          ><el-descriptions-item label="请求" :span="2"
            >{{ detail.method }} {{ detail.path }}</el-descriptions-item
          ><el-descriptions-item label="状态码">{{
            detail.status
          }}</el-descriptions-item
          ><el-descriptions-item label="耗时"
            >{{
              Number(detail.duration_ms).toFixed(1)
            }}
            ms</el-descriptions-item
          ></el-descriptions
        >
        <div
          v-for="[key, label] in [
            ['query', 'URL 参数'],
            ['request_headers', '请求头'],
            ['request_body', '请求正文'],
            ['response_headers', '响应头'],
            ['response_body', '响应正文'],
          ]"
          :key="key"
        >
          <h3>{{ label }}</h3>
          <pre>{{ pretty(detail[key]) }}</pre>
        </div>
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { ref, reactive, onMounted, onBeforeUnmount, watch } from "vue";
import { getLogs, getLog } from "../api/requestLogs";
import PageHeader from "../components/PageHeader.vue";
import { buildQuery } from "../utils";
import { useRoute, useRouter } from "vue-router";
const route = useRoute(),
  router = useRouter();
const queryText = (key) =>
  typeof route.query[key] === "string" ? route.query[key] : "";
const queryPage = () => Math.max(1, parseInt(queryText("page")) || 1);
const now = new Date().toLocaleDateString("en-CA", {
  timeZone: "Asia/Shanghai",
});
const range = ref([queryText("start") || now, queryText("end") || now]);
const filters = reactive({
  path: queryText("path"),
  method: queryText("method"),
  status: queryText("status"),
});
const data = ref({ items: [], total: 0 }),
  page = ref(queryPage()),
  busy = ref(false),
  error = ref(""),
  detail = ref(null),
  opened = ref(false);
let generation = 0;
const time = (value) =>
  new Date(value).toLocaleString("zh-CN", {
    timeZone: "Asia/Shanghai",
    hour12: false,
  });
const pretty = (value) => JSON.stringify(value ?? null, null, 2);
// 关闭详情后清理明文；快速切换记录时，不清空已经重新打开的详情。
function clearClosedDetail() {
  if (!opened.value) detail.value = null;
}
async function load(reset = false, sync = true) {
  const run = ++generation;
  if (reset) page.value = 1;
  if (!range.value?.[0] || !range.value?.[1]) {
    busy.value = false;
    error.value = "请选择日志日期范围";
    return;
  }
  busy.value = true;
  error.value = "";
  try {
    if (sync)
      await router.replace({
        query: {
          ...filters,
          start: range.value[0],
          end: range.value[1],
          page: String(page.value),
        },
      });
    const result = await getLogs(
      buildQuery({
        ...filters,
        start: range.value[0],
        end: range.value[1],
        tz: "Asia/Shanghai",
        page: page.value,
      }),
    );
    if (run === generation) data.value = result;
  } catch (e) {
    if (run === generation) error.value = e.message;
  } finally {
    if (run === generation) busy.value = false;
  }
}
async function show(row) {
  error.value = "";
  try {
    detail.value = await getLog(row.id);
    opened.value = true;
  } catch (e) {
    error.value = e.message;
  }
}
watch(
  () => route.query,
  () => {
    const nextRange = [queryText("start") || now, queryText("end") || now];
    const next = {
      path: queryText("path"),
      method: queryText("method"),
      status: queryText("status"),
    };
    if (
      JSON.stringify(nextRange) === JSON.stringify(range.value) &&
      JSON.stringify(next) === JSON.stringify({ ...filters }) &&
      queryPage() === page.value
    )
      return;
    range.value = nextRange;
    Object.assign(filters, next);
    page.value = queryPage();
    load(false, false);
  },
);
onMounted(() => load(false, false));
onBeforeUnmount(() => {
  generation++;
});
</script>

<style scoped>
.log-filters {
  margin-top: 24px;
}
.log-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 20px;
  gap: 12px;
  flex-wrap: wrap;
}
.request-logs .panel-heading p {
  margin: 6px 0 0;
}
.log-detail pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  background: #f5f7fa;
  padding: 16px;
  border: 1px solid #e5e9ee;
  border-radius: 8px;
  max-height: 380px;
  overflow: auto;
  font:
    12px/1.7 ui-monospace,
    monospace;
}
.log-detail h3 {
  font-size: 14px;
  margin-top: 24px;
}
</style>
