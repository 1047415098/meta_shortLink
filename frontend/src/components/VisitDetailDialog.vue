<template>
  <el-dialog
    v-model="opened"
    title="访问完整明细"
    width="min(1080px, 96vw)"
    class="visit-detail-dialog"
    destroy-on-close
    @closed="clear"
  >
    <div v-loading="loading" class="detail-body">
      <el-alert v-if="error" :title="error" type="error" :closable="false" />
      <template v-else-if="detail">
        <!-- 详情统一读取冻结的访问快照，所有项目看到相同的诊断字段。 -->
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
            >
            <el-descriptions-item label="访问时间">{{
              displayTime(detail.occurred_at)
            }}</el-descriptions-item>
            <el-descriptions-item label="链接"
              >{{ value(detail.link_name) }}（{{ value(detail.code) }} / #{{
                detail.link_id
              }}）</el-descriptions-item
            >
            <el-descriptions-item label="入口">{{
              surfaceLabel(detail.surface)
            }}</el-descriptions-item>
            <el-descriptions-item label="访问类型">{{
              eventTypeLabel(detail.event_type)
            }}</el-descriptions-item>
            <el-descriptions-item label="请求"
              >{{ value(detail.method) }} · HTTP
              {{ value(detail.status) }}</el-descriptions-item
            >
            <el-descriptions-item label="匿名访客标识"
              ><span class="mono breakable">{{
                value(detail.visitor_id)
              }}</span></el-descriptions-item
            >
            <el-descriptions-item label="Cookie 状态">{{
              cookieLabel(detail.cookie_status)
            }}</el-descriptions-item>
            <el-descriptions-item label="流量分类">{{
              classLabel(detail.classification)
            }}</el-descriptions-item>
            <el-descriptions-item label="分类规则"
              >{{ value(detail.rule_version) }} ·
              {{ value(detail.reason) }}</el-descriptions-item
            >
            <el-descriptions-item label="目标地址" :span="2"
              ><span class="breakable">{{
                value(detail.target_url)
              }}</span></el-descriptions-item
            >
          </el-descriptions>
        </section>

        <section class="detail-section">
          <h3>设备与地区</h3>
          <el-descriptions :column="2" border>
            <el-descriptions-item label="设备类型">{{
              deviceLabel(detail.device)
            }}</el-descriptions-item>
            <el-descriptions-item label="设备型号">{{
              collected(detail.device_model)
            }}</el-descriptions-item>
            <el-descriptions-item label="操作系统">{{
              value(detail.os)
            }}</el-descriptions-item>
            <el-descriptions-item label="系统版本">{{
              collected(detail.os_version)
            }}</el-descriptions-item>
            <el-descriptions-item label="浏览器">{{
              value(detail.browser)
            }}</el-descriptions-item>
            <el-descriptions-item label="浏览器版本">{{
              collected(detail.browser_version)
            }}</el-descriptions-item>
            <el-descriptions-item label="国家 / 地区" :span="2">{{
              [detail.country, detail.region, detail.city]
                .filter(Boolean)
                .join(" / ") || "未知"
            }}</el-descriptions-item>
            <el-descriptions-item label="来源页面" :span="2"
              ><span class="breakable">{{
                value(detail.referrer)
              }}</span></el-descriptions-item
            >
          </el-descriptions>
        </section>

        <section class="detail-section">
          <h3>原始请求信息</h3>
          <!-- 原始请求只展示访问白名单，敏感请求头从未写入访问记录。 -->
          <el-descriptions :column="2" border>
            <el-descriptions-item label="客户端 IP">{{
              collected(detail.client_ip)
            }}</el-descriptions-item>
            <el-descriptions-item label="语言偏好">{{
              collected(detail.accept_language)
            }}</el-descriptions-item>
            <el-descriptions-item label="完整 User-Agent" :span="2"
              ><span class="mono breakable">{{
                collected(detail.user_agent)
              }}</span></el-descriptions-item
            >
            <el-descriptions-item label="完整访问 URL" :span="2"
              ><span class="mono breakable">{{
                collected(detail.request_url)
              }}</span></el-descriptions-item
            >
            <el-descriptions-item label="完整来源 URL" :span="2"
              ><span class="mono breakable">{{
                collected(detail.referrer_url)
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
            }}</el-descriptions-item>
            <el-descriptions-item label="来源">{{
              value(detail.source)
            }}</el-descriptions-item>
            <el-descriptions-item label="广告系列 ID">{{
              value(detail.campaign_id)
            }}</el-descriptions-item>
            <el-descriptions-item label="广告组 ID">{{
              value(detail.adset_id || detail.tiktok_adgroup_id)
            }}</el-descriptions-item>
            <el-descriptions-item label="广告 ID">{{
              value(detail.ad_id || detail.tiktok_ad_id_v2)
            }}</el-descriptions-item>
            <el-descriptions-item label="创意 ID">{{
              value(detail.tiktok_creative_id)
            }}</el-descriptions-item>
            <el-descriptions-item label="版位">{{
              value(detail.tiktok_placement)
            }}</el-descriptions-item>
            <el-descriptions-item label="归因冲突">{{
              yesNo(detail.attribution_conflict)
            }}</el-descriptions-item>
            <el-descriptions-item label="TikTok ttclid" :span="2"
              ><span class="mono breakable">{{
                value(detail.tiktok_ttclid)
              }}</span></el-descriptions-item
            >
            <el-descriptions-item label="TikTok _ttp" :span="2"
              ><span class="mono breakable">{{
                value(detail.tiktok_ttp)
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
              >{{ value(detail.novel_title)
              }}<span v-if="detail.novel_id">
                (#{{ detail.novel_id }})</span
              ></el-descriptions-item
            >
            <el-descriptions-item label="入口章节"
              ><template v-if="detail.entry_chapter_id"
                >第 {{ detail.entry_chapter_number || "—" }} 章 ·
                {{ value(detail.entry_chapter_title) }} (#{{
                  detail.entry_chapter_id
                }})</template
              ><template v-else>—</template></el-descriptions-item
            >
            <el-descriptions-item label="可见停留时长">{{
              seconds(detail.visible_seconds)
            }}</el-descriptions-item>
            <el-descriptions-item label="停留更新于">{{
              optionalTime(detail.visible_updated_at)
            }}</el-descriptions-item>
            <el-descriptions-item label="回传阈值">{{
              seconds(detail.time_spent_threshold)
            }}</el-descriptions-item>
            <el-descriptions-item label="阈值已回传于">{{
              optionalTime(detail.time_spent_reported_at)
            }}</el-descriptions-item>
            <el-descriptions-item label="语音小说"
              >{{ value(detail.audio_novel_title)
              }}<span v-if="detail.audio_novel_id">
                (#{{ detail.audio_novel_id }})</span
              ></el-descriptions-item
            >
            <el-descriptions-item label="播放 / 有效消费"
              >{{ seconds(detail.playback_seconds) }} /
              {{ seconds(detail.media_consumed_seconds) }}</el-descriptions-item
            >
            <el-descriptions-item label="开始播放">{{
              optionalTime(detail.audio_started_at)
            }}</el-descriptions-item>
            <el-descriptions-item label="达到有效播放">{{
              optionalTime(detail.audio_qualified_at)
            }}</el-descriptions-item>
            <el-descriptions-item label="播放完成">{{
              optionalTime(detail.audio_completed_at)
            }}</el-descriptions-item>
            <el-descriptions-item label="播放更新于">{{
              optionalTime(detail.playback_updated_at)
            }}</el-descriptions-item>
            <el-descriptions-item label="手动咨询">{{
              optionalTime(detail.whatsapp_clicked_at)
            }}</el-descriptions-item>
            <el-descriptions-item label="自动跳转">{{
              optionalTime(detail.auto_redirected_at)
            }}</el-descriptions-item>
            <el-descriptions-item label="首屏主题">{{
              themeLabel(detail.startup_theme)
            }}</el-descriptions-item>
            <el-descriptions-item label="行为采集说明"
              >仅展示已持久化的停留、播放、咨询及广告平台事件</el-descriptions-item
            >
          </el-descriptions>
        </section>

        <section class="detail-section">
          <h3>广告平台快照</h3>
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Meta 账号"
              >{{ value(detail.meta_connection_name) }} ·
              {{ value(detail.meta_account_id) }}</el-descriptions-item
            >
            <el-descriptions-item label="Meta Pixel"
              >{{ value(detail.meta_pixel_name)
              }}<span v-if="detail.meta_pixel_id">
                (#{{ detail.meta_pixel_id }})</span
              ></el-descriptions-item
            >
            <el-descriptions-item label="Meta 测量启用">{{
              yesNo(detail.meta_measurement)
            }}</el-descriptions-item>
            <el-descriptions-item label="Meta PageView">{{
              yesNo(detail.meta_pageview_enabled)
            }}</el-descriptions-item>
            <el-descriptions-item label="Meta 手动 / 自动事件"
              >{{ yesNo(detail.meta_manual_enabled) }} /
              {{ yesNo(detail.meta_auto_enabled) }}</el-descriptions-item
            >
            <el-descriptions-item label="Meta 手动事件名">{{
              value(detail.meta_manual_event_name)
            }}</el-descriptions-item>
            <el-descriptions-item label="PageView 入队于">{{
              optionalTime(detail.pageview_reported_at)
            }}</el-descriptions-item>
            <el-descriptions-item label="TikTok Pixel"
              >{{ value(detail.tiktok_pixel_name) }} ·
              {{ value(detail.tiktok_pixel_code) }}</el-descriptions-item
            >
            <el-descriptions-item label="TikTok 网络上下文">{{
              detail.tiktok_context_available ? "已加密保存" : "未保存"
            }}</el-descriptions-item>
            <el-descriptions-item label="TikTok 开始阅读">{{
              optionalTime(detail.tiktok_start_reading_at)
            }}</el-descriptions-item>
            <el-descriptions-item label="TikTok 有效阅读">{{
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
            <!-- 展开行展示平台返回的全部非密钥字段，主表保持易读。 -->
            <el-table-column type="expand" width="48">
              <template #default="{ row }">
                <el-descriptions :column="2" border class="delivery-detail">
                  <el-descriptions-item label="事件编号" :span="2"
                    ><span class="mono breakable">{{
                      value(row.id)
                    }}</span></el-descriptions-item
                  >
                  <el-descriptions-item label="测试事件">{{
                    yesNo(row.is_test)
                  }}</el-descriptions-item>
                  <el-descriptions-item label="载荷状态">{{
                    row.payload_available ? "已加密保存" : "未保存或已清理"
                  }}</el-descriptions-item>
                  <el-descriptions-item label="尝试 / 重试次数"
                    >{{ row.attempts }} /
                    {{ row.retry_count }}</el-descriptions-item
                  >
                  <el-descriptions-item label="平台接收数量">{{
                    row.accepted_count
                  }}</el-descriptions-item>
                  <el-descriptions-item label="连接"
                    >{{ value(row.connection_name) }} (#{{
                      row.connection_id || "—"
                    }})</el-descriptions-item
                  >
                  <el-descriptions-item label="Pixel 记录"
                    >{{ value(row.pixel_name) }} (#{{
                      row.pixel_record_id || "—"
                    }})</el-descriptions-item
                  >
                  <el-descriptions-item label="Pixel Code" :span="2"
                    ><span class="mono breakable">{{
                      value(row.pixel_code)
                    }}</span></el-descriptions-item
                  >
                  <el-descriptions-item label="平台请求编号" :span="2"
                    ><span class="mono breakable">{{
                      value(row.external_request_id)
                    }}</span></el-descriptions-item
                  >
                  <el-descriptions-item label="HTTP / 业务码"
                    >{{ row.http_status || "—" }} /
                    {{ row.business_code || "—" }}</el-descriptions-item
                  >
                  <el-descriptions-item label="最后错误">{{
                    value(row.last_error)
                  }}</el-descriptions-item>
                  <el-descriptions-item label="创建时间">{{
                    optionalTime(row.created_at)
                  }}</el-descriptions-item>
                  <el-descriptions-item label="更新时间">{{
                    optionalTime(row.updated_at)
                  }}</el-descriptions-item>
                  <el-descriptions-item label="平台响应消息" :span="2">
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
            >
            <el-table-column prop="event_name" label="事件" min-width="130" />
            <el-table-column label="事件时间" width="170"
              ><template #default="{ row }">{{
                displayTime(row.event_time)
              }}</template></el-table-column
            >
            <el-table-column label="状态" width="105"
              ><template #default="{ row }"
                ><el-tag :type="deliveryStatusType(row.status)">{{
                  row.status
                }}</el-tag></template
              ></el-table-column
            >
            <el-table-column prop="attempts" label="尝试" width="72" />
            <el-table-column label="Pixel" min-width="150"
              ><template #default="{ row }"
                >{{ row.pixel_name || "—" }}<br /><small class="muted">{{
                  row.pixel_code || "#" + row.pixel_record_id
                }}</small></template
              ></el-table-column
            >
            <el-table-column label="平台响应" min-width="180"
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
                ></template
              ></el-table-column
            >
            <el-table-column label="错误" min-width="180"
              ><template #default="{ row }">{{
                value(row.last_error)
              }}</template></el-table-column
            >
          </el-table>
        </section>
      </template>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref } from "vue";
import { getVisit } from "../api/analytics.js";

const props = defineProps({ timezone: { type: String, default: "Etc/GMT+8" } });
const opened = ref(false),
  loading = ref(false),
  error = ref(""),
  detail = ref(null);
let generation = 0;

// The parent passes only the immutable visit ID; the complete safe snapshot loads on demand.
async function open(id) {
  const run = ++generation;
  opened.value = true;
  loading.value = true;
  error.value = "";
  detail.value = null;
  try {
    const result = await getVisit(id);
    if (run === generation) detail.value = result;
  } catch (reason) {
    if (run === generation) error.value = reason.message;
  } finally {
    if (run === generation) loading.value = false;
  }
}
function clear() {
  generation++;
  loading.value = false;
  error.value = "";
  detail.value = null;
}
defineExpose({ open });

const value = (item) =>
  item === null || item === undefined || item === "" ? "—" : String(item);
const collected = (item) =>
  item === null || item === undefined || item === "" ? "未采集" : String(item);
const pretty = (item) => JSON.stringify(item || {}, null, 2);
const yesNo = (item) => (item ? "是" : "否");
const seconds = (item) => {
  const number = Number(item || 0);
  return `${Number.isInteger(number) ? number : number.toFixed(2)} 秒`;
};
const optionalTime = (item) => (item ? displayTime(item) : "—");
const deviceLabel = (item) =>
  ({ mobile: "手机", tablet: "平板", desktop: "电脑", unknown: "未知设备" })[
    item
  ] || value(item);
const classLabel = (item) =>
  ({
    normal: "过滤后",
    filtered: "过滤后",
    bot: "机器人",
    suspicious: "疑似异常",
    unclassified: "未分类",
    head: "HEAD 请求",
    prefetch: "预取",
  })[item] || value(item);
const cookieLabel = (item) =>
  ({ recognized: "已识别", issued: "首次下发", disabled: "未启用" })[item] ||
  value(item);
const surfaceLabel = (item) =>
  ({
    short_link: "短链接",
    audio_novel: "语音小说站",
    novel: "免费小说站",
    cover: "封面项目",
  })[item] || value(item);
const eventTypeLabel = (item) =>
  ({ landing: "落地页访问", redirect: "直接跳转" })[item] || value(item);
const platformLabel = (item) =>
  ({ meta: "Meta", tiktok: "TikTok" })[item] || value(item);
const themeLabel = (item) =>
  ({ countdown: "倒计时进度条", cover_wall: "封面墙引导" })[item] ||
  value(item);
const deliveryStatusType = (item) =>
  ["succeeded", "accepted"].includes(item)
    ? "success"
    : ["failed", "expired"].includes(item)
      ? "danger"
      : ["retry", "pending", "processing", "sending"].includes(item)
        ? "warning"
        : "info";
function displayTime(item) {
  if (!item) return "—";
  try {
    return new Intl.DateTimeFormat("zh-CN", {
      dateStyle: "short",
      timeStyle: "medium",
      timeZone: props.timezone,
    }).format(new Date(item));
  } catch {
    return item;
  }
}
</script>

<style scoped>
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
@media (max-width: 640px) {
  .detail-section :deep(.el-descriptions__body) {
    overflow-x: auto;
  }
  .detail-section :deep(.el-descriptions__table) {
    min-width: 620px;
  }
}
</style>
