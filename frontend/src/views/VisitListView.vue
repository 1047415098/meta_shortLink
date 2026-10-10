<template>
  <section class="visits-page">
    <PageHeader title="访问明细"
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
    <section class="panel" v-loading="busy">
      <div class="panel-heading">
        <h2>访问记录</h2>
        <span class="muted"
          >共 {{ fmt(clicks.total) }} 条 · 时间按所选时区显示</span
        >
      </div>
      <el-table
        :data="clicks.items || []"
        empty-text="当前筛选条件下没有访问记录"
        ><el-table-column label="访问时间" width="170"
          ><template #default="{ row }">{{
            displayTime(row.occurred_at)
          }}</template></el-table-column
        ><el-table-column
          prop="code"
          label="短码"
          width="100"
        /><el-table-column label="访问 / 咨询" min-width="160"
          ><template #default="{ row }"
            ><div>
              {{ row.event_type === "landing" ? "落地页访问" : "直接跳转" }}
            </div>
            <small class="muted"
              ><span v-if="row.whatsapp_clicked_at"
                >手动咨询：{{ displayTime(row.whatsapp_clicked_at) }}</span
              ><br
                v-if="row.whatsapp_clicked_at && row.auto_redirected_at"
              /><span v-if="row.auto_redirected_at"
                >自动跳转：{{ displayTime(row.auto_redirected_at) }}</span
              ><span v-if="!row.whatsapp_clicked_at && !row.auto_redirected_at"
                >—</span
              ></small
            ></template
          ></el-table-column
        ><el-table-column label="入口" width="90"
          ><template #default="{ row }"
            ><el-tag
              :type="
                row.surface === 'audio_novel'
                  ? 'warning'
                  : row.surface === 'novel'
                    ? 'success'
                    : row.surface === 'cover'
                      ? 'primary'
                      : 'info'
              "
              effect="plain"
            >
              {{
                row.surface === "audio_novel"
                  ? "语音小说站"
                  : row.surface === "novel"
                    ? "免费小说站"
                    : row.surface === "cover"
                      ? "封面项目"
                      : "短链接"
              }}
            </el-tag></template
          ></el-table-column
        ><el-table-column label="设备" min-width="135"
          ><template #default="{ row }"
            >{{ deviceLabel[row.device] || row.device || "未知" }}
            <div class="muted">{{ row.os }} · {{ row.browser }}</div></template
          ></el-table-column
        ><el-table-column label="地区" min-width="130"
          ><template #default="{ row }">{{
            [row.country, row.region, row.city].filter(Boolean).join(" / ") ||
            "未知"
          }}</template></el-table-column
        ><el-table-column
          prop="source"
          label="来源"
          min-width="100"
        /><el-table-column
          prop="ad_id"
          label="广告 ID"
          min-width="130"
        /><el-table-column label="访客标识" width="115"
          ><template #default="{ row }"
            ><el-tag type="info" effect="plain">{{
              {
                recognized: "已识别",
                issued: "首次下发",
                disabled: "未启用",
              }[row.cookie_status] || row.cookie_status
            }}</el-tag></template
          ></el-table-column
        ><el-table-column label="流量分类" width="130"
          ><template #default="{ row }"
            ><el-tooltip :content="row.reason || '未记录分类原因'"
              ><el-tag
                :type="
                  row.classification === 'bot'
                    ? 'info'
                    : row.classification === 'suspicious'
                      ? 'warning'
                      : 'success'
                "
                >{{
                  className[row.classification] || row.classification
                }}</el-tag
              ></el-tooltip
            ><span v-if="row.attribution_conflict" class="conflict"
              >归因冲突</span
            ></template
          ></el-table-column
        ><!-- 完整明细独立加载，避免列表接口为每行重复返回大量归因和回传数据。 -->
        <el-table-column label="操作" width="82" fixed="right"
          ><template #default="{ row }"
            ><el-button
              link
              type="primary"
              :loading="detailLoading && selectedVisitID === row.id"
              @click="showDetail(row)"
              >明细</el-button
            ></template
          ></el-table-column
        ></el-table
      ><el-pagination
        class="pagination"
        layout="prev, pager, next, total"
        :total="clicks.total"
        :page-size="50"
        v-model:current-page="clickPage"
        @current-change="load(false)"
      />
    </section>
    <el-dialog
      v-model="detailOpened"
      title="访问完整明细"
      width="min(1080px, 96vw)"
      class="visit-detail-dialog"
      destroy-on-close
      @closed="clearDetail"
    >
      <div v-loading="detailLoading" class="detail-body">
        <el-alert
          v-if="detailError"
          :title="detailError"
          type="error"
          :closable="false"
        />
        <template v-else-if="detail">
          <!-- 访问表只展示已安全保存的快照；加密回传载荷不会下发到浏览器。 -->
          <el-alert
            title="本页明文展示访问诊断数据，并已记录管理员查看审计；后台 Cookie、Authorization、广告平台访问令牌及加密密钥不会采集或展示。"
            type="warning"
            :closable="false"
            class="detail-notice"
          />

          <section class="detail-section">
            <h3>基础访问</h3>
            <el-descriptions :column="2" border>
              <el-descriptions-item label="访问编号" :span="2"
                ><span class="mono">{{ detail.id }}</span></el-descriptions-item
              ><el-descriptions-item label="访问时间">{{
                displayTime(detail.occurred_at)
              }}</el-descriptions-item
              ><el-descriptions-item label="链接"
                >{{ detail.link_name || "—" }}（{{ detail.code }} / #{{
                  detail.link_id
                }}）</el-descriptions-item
              ><el-descriptions-item label="入口">{{
                surfaceLabel(detail.surface)
              }}</el-descriptions-item
              ><el-descriptions-item label="访问类型">{{
                eventTypeLabel(detail.event_type)
              }}</el-descriptions-item
              ><el-descriptions-item label="请求"
                >{{ detail.method }} · HTTP
                {{ detail.status }}</el-descriptions-item
              ><el-descriptions-item label="匿名访客标识"
                ><span class="mono">{{
                  showValue(detail.visitor_id)
                }}</span></el-descriptions-item
              ><el-descriptions-item label="Cookie 状态">{{
                cookieLabel(detail.cookie_status)
              }}</el-descriptions-item
              ><el-descriptions-item label="流量分类">{{
                className[detail.classification] || detail.classification || "—"
              }}</el-descriptions-item
              ><el-descriptions-item label="分类规则"
                >{{ showValue(detail.rule_version) }} ·
                {{ showValue(detail.reason) }}</el-descriptions-item
              ><el-descriptions-item label="目标地址" :span="2"
                ><span class="breakable">{{
                  showValue(detail.target_url)
                }}</span></el-descriptions-item
              >
            </el-descriptions>
          </section>

          <section class="detail-section">
            <h3>设备与地区</h3>
            <!-- 版本字段仅对迁移后的新访问可用，历史记录明确标记为未采集。 -->
            <el-descriptions :column="2" border>
              <el-descriptions-item label="设备类型">{{
                deviceLabel[detail.device] || detail.device || "未知设备"
              }}</el-descriptions-item
              ><el-descriptions-item label="设备型号">{{
                collectedValue(detail.device_model)
              }}</el-descriptions-item
              ><el-descriptions-item label="操作系统">{{
                showValue(detail.os)
              }}</el-descriptions-item
              ><el-descriptions-item label="系统版本">{{
                collectedValue(detail.os_version)
              }}</el-descriptions-item
              ><el-descriptions-item label="浏览器">{{
                showValue(detail.browser)
              }}</el-descriptions-item
              ><el-descriptions-item label="浏览器版本">{{
                collectedValue(detail.browser_version)
              }}</el-descriptions-item
              ><el-descriptions-item label="国家 / 地区" :span="2">{{
                [detail.country, detail.region, detail.city]
                  .filter(Boolean)
                  .join(" / ") || "未知"
              }}</el-descriptions-item
              ><el-descriptions-item label="来源页面" :span="2"
                ><span class="breakable">{{
                  showValue(detail.referrer)
                }}</span></el-descriptions-item
              >
            </el-descriptions>
          </section>

          <section class="detail-section">
            <h3>原始请求信息</h3>
            <!-- 仅展示访问入口采集的诊断白名单；账号凭证和后台会话请求头永不进入访问快照。 -->
            <el-descriptions :column="2" border>
              <el-descriptions-item label="客户端 IP">{{
                collectedValue(detail.client_ip)
              }}</el-descriptions-item
              ><el-descriptions-item label="语言偏好">{{
                collectedValue(detail.accept_language)
              }}</el-descriptions-item
              ><el-descriptions-item label="完整 User-Agent" :span="2"
                ><span class="breakable mono">{{
                  collectedValue(detail.user_agent)
                }}</span></el-descriptions-item
              ><el-descriptions-item label="完整访问 URL" :span="2"
                ><span class="breakable mono">{{
                  collectedValue(detail.request_url)
                }}</span></el-descriptions-item
              ><el-descriptions-item label="完整来源 URL" :span="2"
                ><span class="breakable mono">{{
                  collectedValue(detail.referrer_url)
                }}</span></el-descriptions-item
              >
            </el-descriptions>
            <h4>浏览器 Client Hints</h4>
            <pre class="detail-json">{{ pretty(detail.client_hints) }}</pre>
          </section>

          <section class="detail-section">
            <h3>广告归因</h3>
            <el-descriptions :column="2" border>
              <el-descriptions-item label="广告平台">{{
                platformLabel(detail.ad_platform)
              }}</el-descriptions-item
              ><el-descriptions-item label="来源">{{
                showValue(detail.source)
              }}</el-descriptions-item
              ><el-descriptions-item label="广告系列 ID">{{
                showValue(detail.campaign_id)
              }}</el-descriptions-item
              ><el-descriptions-item label="广告组 ID">{{
                showValue(detail.adset_id || detail.tiktok_adgroup_id)
              }}</el-descriptions-item
              ><el-descriptions-item label="广告 ID">{{
                showValue(detail.ad_id || detail.tiktok_ad_id_v2)
              }}</el-descriptions-item
              ><el-descriptions-item label="创意 ID">{{
                showValue(detail.tiktok_creative_id)
              }}</el-descriptions-item
              ><el-descriptions-item label="版位">{{
                showValue(detail.tiktok_placement)
              }}</el-descriptions-item
              ><el-descriptions-item label="归因冲突">{{
                yesNo(detail.attribution_conflict)
              }}</el-descriptions-item
              ><el-descriptions-item label="TikTok ttclid" :span="2"
                ><span class="breakable mono">{{
                  showValue(detail.tiktok_ttclid)
                }}</span></el-descriptions-item
              ><el-descriptions-item label="TikTok _ttp" :span="2"
                ><span class="breakable mono">{{
                  showValue(detail.tiktok_ttp)
                }}</span></el-descriptions-item
              >
            </el-descriptions>
            <h4>冻结的投放参数</h4>
            <pre class="detail-json">{{ pretty(detail.parameters) }}</pre>
          </section>

          <section class="detail-section">
            <h3>内容与用户行为</h3>
            <el-descriptions :column="2" border>
              <el-descriptions-item label="绑定小说"
                >{{ showValue(detail.novel_title)
                }}<span v-if="detail.novel_id">
                  (#{{ detail.novel_id }})</span
                ></el-descriptions-item
              ><el-descriptions-item label="入口章节"
                ><template v-if="detail.entry_chapter_id"
                  >第 {{ detail.entry_chapter_number || "—" }} 章 ·
                  {{ showValue(detail.entry_chapter_title) }} (#{{
                    detail.entry_chapter_id
                  }})</template
                ><template v-else>—</template></el-descriptions-item
              ><el-descriptions-item label="可见停留时长">{{
                seconds(detail.visible_seconds)
              }}</el-descriptions-item
              ><el-descriptions-item label="停留更新于">{{
                optionalTime(detail.visible_updated_at)
              }}</el-descriptions-item
              ><el-descriptions-item label="回传阈值">{{
                seconds(detail.time_spent_threshold)
              }}</el-descriptions-item
              ><el-descriptions-item label="阈值已回传于">{{
                optionalTime(detail.time_spent_reported_at)
              }}</el-descriptions-item
              ><el-descriptions-item label="语音小说"
                >{{ showValue(detail.audio_novel_title)
                }}<span v-if="detail.audio_novel_id">
                  (#{{ detail.audio_novel_id }})</span
                ></el-descriptions-item
              ><el-descriptions-item label="播放 / 有效消费"
                >{{ seconds(detail.playback_seconds) }} /
                {{
                  seconds(detail.media_consumed_seconds)
                }}</el-descriptions-item
              ><el-descriptions-item label="开始播放">{{
                optionalTime(detail.audio_started_at)
              }}</el-descriptions-item
              ><el-descriptions-item label="达到有效播放">{{
                optionalTime(detail.audio_qualified_at)
              }}</el-descriptions-item
              ><el-descriptions-item label="播放完成">{{
                optionalTime(detail.audio_completed_at)
              }}</el-descriptions-item
              ><el-descriptions-item label="播放更新于">{{
                optionalTime(detail.playback_updated_at)
              }}</el-descriptions-item
              ><el-descriptions-item label="手动咨询">{{
                optionalTime(detail.whatsapp_clicked_at)
              }}</el-descriptions-item
              ><el-descriptions-item label="自动跳转">{{
                optionalTime(detail.auto_redirected_at)
              }}</el-descriptions-item
              ><el-descriptions-item label="首屏主题">{{
                themeLabel(detail.startup_theme)
              }}</el-descriptions-item
              ><!-- 当前数据模型没有持久化封面墙内部步骤，避免把前端动画误当作已采集事件。 -->
              <el-descriptions-item label="行为采集说明"
                >仅展示已持久化的停留、播放、咨询及广告平台事件</el-descriptions-item
              >
            </el-descriptions>
          </section>

          <section class="detail-section">
            <h3>广告平台快照</h3>
            <el-descriptions :column="2" border>
              <el-descriptions-item label="Meta 账号"
                >{{ showValue(detail.meta_connection_name) }} ·
                {{ showValue(detail.meta_account_id) }}</el-descriptions-item
              ><el-descriptions-item label="Meta Pixel"
                >{{ showValue(detail.meta_pixel_name)
                }}<span v-if="detail.meta_pixel_id">
                  (#{{ detail.meta_pixel_id }})</span
                ></el-descriptions-item
              ><el-descriptions-item label="Meta 测量启用">{{
                yesNo(detail.meta_measurement)
              }}</el-descriptions-item
              ><el-descriptions-item label="Meta PageView">{{
                yesNo(detail.meta_pageview_enabled)
              }}</el-descriptions-item
              ><el-descriptions-item label="Meta 手动 / 自动事件"
                >{{ yesNo(detail.meta_manual_enabled) }} /
                {{ yesNo(detail.meta_auto_enabled) }}</el-descriptions-item
              ><el-descriptions-item label="Meta 手动事件名">{{
                showValue(detail.meta_manual_event_name)
              }}</el-descriptions-item
              ><el-descriptions-item label="PageView 入队于">{{
                optionalTime(detail.pageview_reported_at)
              }}</el-descriptions-item
              ><el-descriptions-item label="TikTok Pixel"
                >{{ showValue(detail.tiktok_pixel_name) }} ·
                {{ showValue(detail.tiktok_pixel_code) }}</el-descriptions-item
              ><el-descriptions-item label="TikTok 网络上下文">{{
                detail.tiktok_context_available ? "已加密保存" : "未保存"
              }}</el-descriptions-item
              ><el-descriptions-item label="TikTok 开始阅读">{{
                optionalTime(detail.tiktok_start_reading_at)
              }}</el-descriptions-item
              ><el-descriptions-item label="TikTok 有效阅读">{{
                optionalTime(detail.tiktok_view_content_at)
              }}</el-descriptions-item>
            </el-descriptions>
          </section>

          <section class="detail-section">
            <h3>Meta / TikTok 回传记录</h3>
            <el-table
              :data="detail.delivery_events || []"
              empty-text="该访问没有生成广告平台回传事件"
              border
            >
              <!-- 展开行展示回传记录的全部非密钥字段，避免主表横向失控。 -->
              <el-table-column type="expand" width="48">
                <template #default="{ row }">
                  <el-descriptions :column="2" border class="delivery-detail">
                    <el-descriptions-item label="事件编号" :span="2"
                      ><span class="breakable mono">{{
                        showValue(row.id)
                      }}</span></el-descriptions-item
                    ><el-descriptions-item label="测试事件">{{
                      yesNo(row.is_test)
                    }}</el-descriptions-item
                    ><el-descriptions-item label="载荷状态">{{
                      row.payload_available ? "已加密保存" : "未保存或已清理"
                    }}</el-descriptions-item
                    ><el-descriptions-item label="尝试 / 重试次数"
                      >{{ row.attempts }} /
                      {{ row.retry_count }}</el-descriptions-item
                    ><el-descriptions-item label="平台接收数量">{{
                      row.accepted_count
                    }}</el-descriptions-item
                    ><el-descriptions-item label="连接"
                      >{{ showValue(row.connection_name) }} (#{{
                        row.connection_id || "—"
                      }})</el-descriptions-item
                    ><el-descriptions-item label="Pixel 记录"
                      >{{ showValue(row.pixel_name) }} (#{{
                        row.pixel_record_id || "—"
                      }})</el-descriptions-item
                    ><el-descriptions-item label="Pixel Code" :span="2"
                      ><span class="breakable mono">{{
                        showValue(row.pixel_code)
                      }}</span></el-descriptions-item
                    ><el-descriptions-item label="平台请求编号" :span="2"
                      ><span class="breakable mono">{{
                        showValue(row.external_request_id)
                      }}</span></el-descriptions-item
                    ><el-descriptions-item label="HTTP / 业务码"
                      >{{ row.http_status || "—" }} /
                      {{ row.business_code || "—" }}</el-descriptions-item
                    ><el-descriptions-item label="最后错误">{{
                      showValue(row.last_error)
                    }}</el-descriptions-item
                    ><el-descriptions-item label="创建时间">{{
                      optionalTime(row.created_at)
                    }}</el-descriptions-item
                    ><el-descriptions-item label="更新时间">{{
                      optionalTime(row.updated_at)
                    }}</el-descriptions-item
                    ><el-descriptions-item label="平台响应消息" :span="2">
                      <pre class="detail-json compact-json">{{
                        pretty(row.response_messages)
                      }}</pre>
                    </el-descriptions-item>
                  </el-descriptions>
                </template>
              </el-table-column>
              <el-table-column label="平台" width="85"
                ><template #default="{ row }">{{
                  platformLabel(row.platform)
                }}</template></el-table-column
              ><el-table-column
                prop="event_name"
                label="事件"
                min-width="130"
              />
              <el-table-column label="事件时间" width="170"
                ><template #default="{ row }">{{
                  displayTime(row.event_time)
                }}</template></el-table-column
              ><el-table-column label="状态" width="105"
                ><template #default="{ row }"
                  ><el-tag :type="deliveryStatusType(row.status)">{{
                    row.status
                  }}</el-tag></template
                ></el-table-column
              ><el-table-column label="尝试" width="72"
                ><template #default="{ row }">{{
                  row.attempts
                }}</template></el-table-column
              ><el-table-column label="Pixel" min-width="150"
                ><template #default="{ row }"
                  >{{ row.pixel_name || "—" }}<br /><small class="muted">{{
                    row.pixel_code || "#" + row.pixel_record_id
                  }}</small></template
                ></el-table-column
              ><el-table-column label="平台响应" min-width="180"
                ><template #default="{ row }"
                  ><div v-if="row.external_request_id" class="mono breakable">
                    {{ row.external_request_id }}
                  </div>
                  <div v-if="row.http_status">
                    HTTP {{ row.http_status }} · Code {{ row.business_code }}
                  </div>
                  <div v-if="row.accepted_count">
                    接收 {{ row.accepted_count }} 条
                  </div>
                  <span
                    v-if="
                      !row.external_request_id &&
                      !row.http_status &&
                      !row.accepted_count
                    "
                    >—</span
                  >
                </template></el-table-column
              ><el-table-column label="错误" min-width="180"
                ><template #default="{ row }">{{
                  showValue(row.last_error)
                }}</template></el-table-column
              >
            </el-table>
          </section>
        </template>
      </div>
    </el-dialog>
  </section>
</template>

<script setup>
import { computed, ref } from "vue";

import PageHeader from "../components/PageHeader.vue";
import { Refresh, Download } from "@element-plus/icons-vue";

import AnalyticsFilter from "../components/AnalyticsFilter.vue";
import { useReport } from "../composables/useReport";
import { getVisit, getVisits } from "../api/analytics";

const {
  filters,
  data,
  links,
  busy,
  error,
  load,
  resetFilters,
  exportCSV,
  clickPage,
} = useReport(getVisits);
const clicks = computed(() => data.value || { items: [], total: 0 });
// Detail state is isolated from the report loading state so refreshing the list does not close the dialog.
const detailOpened = ref(false);
const detailLoading = ref(false);
const detailError = ref("");
const detail = ref(null);
const selectedVisitID = ref("");
let detailGeneration = 0;
const fmt = (value) => new Intl.NumberFormat("zh-CN").format(value || 0);
const deviceLabel = {
  mobile: "手机",
  tablet: "平板",
  desktop: "电脑",
  unknown: "未知设备",
};
const className = {
  normal: "过滤后",
  filtered: "过滤后",
  bot: "机器人",
  suspicious: "疑似异常",
  unclassified: "未分类",
  head: "HEAD 请求",
  prefetch: "预取",
};
function displayTime(v) {
  try {
    return new Intl.DateTimeFormat("zh-CN", {
      dateStyle: "short",
      timeStyle: "medium",
      timeZone: filters.tz,
    }).format(new Date(v));
  } catch {
    return v;
  }
}
const showValue = (value) =>
  value === null || value === undefined || value === "" ? "—" : String(value);
// Newly introduced device fields distinguish genuinely missing historical data from an unknown parsed name.
const collectedValue = (value) =>
  value === null || value === undefined || value === ""
    ? "未采集"
    : String(value);
const optionalTime = (value) => (value ? displayTime(value) : "—");
const yesNo = (value) => (value ? "是" : "否");
const seconds = (value) => {
  const number = Number(value || 0);
  return Number.isInteger(number) ? number + " 秒" : number.toFixed(2) + " 秒";
};
const pretty = (value) => JSON.stringify(value || {}, null, 2);
const cookieLabel = (value) =>
  ({ recognized: "已识别", issued: "首次下发", disabled: "未启用" })[value] ||
  showValue(value);
// 封面项目使用独立入口类型，便于在访问明细中与免费小说数据区分。
const surfaceLabel = (value) =>
  ({
    short_link: "短链接",
    audio_novel: "语音小说站",
    novel: "免费小说站",
    cover: "封面项目",
  })[value] || showValue(value);
const eventTypeLabel = (value) =>
  ({ landing: "落地页访问", redirect: "直接跳转" })[value] || showValue(value);
const platformLabel = (value) =>
  ({ meta: "Meta", tiktok: "TikTok" })[value] || showValue(value);
const themeLabel = (value) =>
  ({ countdown: "倒计时进度条", cover_wall: "封面墙引导" })[value] ||
  showValue(value);
const deliveryStatusType = (value) => {
  if (["succeeded", "accepted"].includes(value)) return "success";
  if (["failed", "expired"].includes(value)) return "danger";
  if (["retry", "pending", "processing", "sending"].includes(value))
    return "warning";
  return "info";
};
async function showDetail(row) {
  const run = ++detailGeneration;
  selectedVisitID.value = row.id;
  detail.value = null;
  detailError.value = "";
  detailLoading.value = true;
  detailOpened.value = true;
  try {
    const result = await getVisit(row.id);
    if (run === detailGeneration) detail.value = result;
  } catch (e) {
    if (run === detailGeneration) detailError.value = e.message;
  } finally {
    if (run === detailGeneration) detailLoading.value = false;
  }
}
function clearDetail() {
  detailGeneration++;
  detail.value = null;
  detailError.value = "";
  detailLoading.value = false;
  selectedVisitID.value = "";
}
</script>

<style scoped>
.visits-page {
  min-width: 0;
}
.detail-body {
  min-height: 120px;
}
.detail-notice {
  margin-bottom: 18px;
}
.detail-section + .detail-section {
  margin-top: 24px;
}
.detail-section h3 {
  margin: 0 0 12px;
  font-size: 16px;
}
.detail-section h4 {
  margin: 16px 0 8px;
  font-size: 14px;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}
.breakable {
  overflow-wrap: anywhere;
  word-break: break-word;
}
.detail-json {
  margin: 0;
  padding: 14px;
  overflow: auto;
  border: 1px solid #e5e9ee;
  border-radius: 8px;
  background: #f5f7fa;
  font:
    12px/1.7 ui-monospace,
    SFMono-Regular,
    Menlo,
    Monaco,
    Consolas,
    monospace;
}
.delivery-detail {
  margin: 12px 16px;
}
.compact-json {
  padding: 8px;
}
</style>
