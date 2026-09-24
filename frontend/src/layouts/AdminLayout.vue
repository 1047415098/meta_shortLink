<template>
  <el-container class="app-shell" :class="{ 'is-collapsed': collapsed }">
    <el-aside class="sidebar" :width="collapsed ? '76px' : '232px'">
      <div class="brand">
        <span class="brand-symbol"
          ><el-icon><Connection /></el-icon></span
        ><span v-if="!collapsed">LinkScope<small>广告数据工作台</small></span>
      </div>
      <div class="workspace-label" v-if="!collapsed">
        WORKSPACE <el-tag size="small" effect="plain">自有数据</el-tag>
      </div>
      <!-- Detail routes keep their parent navigation item selected. -->
      <el-menu
        :default-active="activeMenu"
        :default-openeds="openGroups"
        :collapse="collapsed"
        :collapse-transition="false"
        @select="navigate"
        class="main-menu"
        aria-label="主导航"
      >
        <!-- 按前端项目和后台职能分组，避免运营人员混淆内容归属。 -->
        <template v-for="item in nav" :key="item.name">
          <el-sub-menu v-if="item.children" :index="item.name">
            <template #title
              ><el-icon><component :is="item.icon" /></el-icon
              ><span>{{ item.label }}</span></template
            >
            <el-menu-item
              v-for="child in item.children"
              :key="child.name"
              :index="child.name"
              >{{ child.label }}</el-menu-item
            >
          </el-sub-menu>
          <el-menu-item v-else :index="item.name"
            ><el-icon><component :is="item.icon" /></el-icon
            ><template #title>{{ item.label }}</template></el-menu-item
          >
        </template>
      </el-menu>
      <div class="sidebar-bottom" v-if="!collapsed">
        <div class="sidebar-app-icon">
          <el-icon><Connection /></el-icon>
        </div>
        <b>WhatsApp 引流分析</b>
        <p>管理链接，洞察每次访问</p>
        <el-tag type="success" effect="light" size="small">独立数据空间</el-tag>
      </div>
    </el-aside>
    <el-container direction="vertical" class="workspace"
      ><el-header class="topbar" height="64px"
        ><div class="topbar-left">
          <el-button
            text
            circle
            :icon="collapsed ? Expand : Fold"
            :aria-label="collapsed ? '展开导航' : '收起导航'"
            @click="collapsed = !collapsed"
          /><el-breadcrumb separator="/"
            ><el-breadcrumb-item>工作空间</el-breadcrumb-item
            ><el-breadcrumb-item>{{
              route.meta.title
            }}</el-breadcrumb-item></el-breadcrumb
          >
        </div>
        <div class="topbar-right">
          <el-tag effect="plain" class="workspace-tag"
            >WhatsApp Analytics</el-tag
          ><el-divider direction="vertical" /><el-dropdown @command="logout"
            ><span class="account-menu"
              ><el-avatar :size="30" :icon="User" /><span>{{
                user?.username
              }}</span
              ><el-icon><ArrowDown /></el-icon></span
            ><template #dropdown
              ><el-dropdown-menu
                ><el-dropdown-item command="logout" :icon="SwitchButton"
                  >退出登录</el-dropdown-item
                ></el-dropdown-menu
              ></template
            ></el-dropdown
          >
        </div></el-header
      >
      <main><router-view /></main>
      <footer>
        LinkScope
        <a
          v-if="settings.geo_enabled"
          href="https://db-ip.com"
          target="_blank"
          rel="noopener noreferrer"
          >IP Geolocation by DB-IP</a
        ><span>访问分析，不将点击等同于转化。</span>
      </footer></el-container
    ></el-container
  >
</template>

<script setup>
import {
  DataAnalysis,
  Link,
  Document,
  TrendCharts,
  Setting,
  Fold,
  Expand,
  ArrowDown,
  User,
  SwitchButton,
  Connection,
  Reading,
} from "@element-plus/icons-vue";
import { computed, ref, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { user, logout as signOut } from "../stores/auth";
import { settings, loadSettings } from "../stores/settings";
import { ElMessage } from "element-plus/es/components/message/index";
const route = useRoute(),
  router = useRouter(),
  // 手机端默认收起菜单，二级分组通过 Element Plus 浮层展示。
  collapsed = ref(window.matchMedia("(max-width: 800px)").matches);
const nav = [
  { name: "overview", icon: DataAnalysis, label: "数据总览" },
  {
    name: "short-link-project",
    icon: Link,
    label: "短链接项目",
    children: [{ name: "links", label: "短链接管理" }],
  },
  {
    name: "audio-novel-project",
    icon: Reading,
    label: "语音小说项目",
    children: [{ name: "audio-novels", label: "语音小说管理" }],
  },
  {
    name: "novel-project",
    icon: Reading,
    label: "免费小说项目",
    children: [{ name: "novels", label: "小说管理" }],
  },
  { name: "visits", icon: Document, label: "访问明细" },
  { name: "ads", icon: TrendCharts, label: "导入统计" },
  {
    name: "meta-management",
    icon: Connection,
    label: "Meta 管理",
    children: [
      { name: "meta-connections", label: "Meta 帐号" },
      { name: "meta-pixels", label: "Meta Pixel" },
      { name: "meta-credentials", label: "Meta 凭证" },
      { name: "meta-source", label: "来源诊断" },
      { name: "meta-events", label: "Meta 事件记录" },
    ],
  },
  {
    name: "tiktok-management",
    icon: TrendCharts,
    label: "TikTok 管理",
    // 按运营顺序排列：先配置 Pixel 与凭证，再核对事件记录。
    children: [
      { name: "tiktok-pixels", label: "TikTok Pixel" },
      { name: "tiktok-connections", label: "TikTok 凭证" },
      { name: "tiktok-events", label: "TikTok 事件记录" },
    ],
  },
  {
    name: "system-management",
    icon: Setting,
    label: "系统",
    children: [
      { name: "logs", label: "日志管理" },
      { name: "settings", label: "系统设置" },
    ],
  },
];
const activeMenu = computed(() => route.meta.activeMenu || route.name);
// 详情页沿用所属列表菜单，并自动展开对应项目分组。
const openGroups = computed(() =>
  nav
    .filter((item) =>
      item.children?.some((child) => child.name === activeMenu.value),
    )
    .map((item) => item.name),
);
function navigate(name) {
  router.push({ name });
}
async function logout() {
  try {
    await signOut();
    router.replace("/admin/login");
  } catch (e) {
    ElMessage.error(e.message);
  }
}
onMounted(() => loadSettings().catch((e) => ElMessage.error(e.message)));
</script>

<style scoped>
.brand {
  display: flex;
  align-items: center;
  gap: 11px;
  color: #233047;
  font-size: 22px;
  font-weight: 700;
  white-space: nowrap;
  height: 76px;
  padding: 0 22px;
}
.brand small {
  display: block;
  font-size: 10px;
  letter-spacing: 2px;
  font-weight: 400;
  color: #a3aab5;
  margin-top: 3px;
}
.brand-symbol {
  width: 35px;
  height: 35px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #409eff;
  color: #fff;
  box-shadow: 0 4px 10px #409eff26;
  flex-shrink: 0;
  font-size: 22px;
}
.app-shell {
  min-height: 100vh;
}
.sidebar {
  position: fixed;
  top: 0;
  bottom: 0;
  left: 0;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-right: 1px solid #ebeef5;
  z-index: 15;
  transition: width 0.2s;
  overflow: hidden;
}
.workspace-label {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 10px;
  color: #b1b7c1;
  letter-spacing: 1.4px;
  padding: 24px 24px 14px;
}
.workspace-label :deep(.el-tag) {
  font-size: 10px;
  letter-spacing: 0;
  border-color: #e8edf5;
  color: #969fab;
  background: #fafbfd;
}
.main-menu.el-menu {
  overflow-y: auto;
  min-height: 0;
  flex: 1;
  border: 0;
  padding: 4px 12px;
  background: transparent;
}
.main-menu:not(.el-menu--collapse) {
  width: 232px;
}
.main-menu :deep(.el-menu-item) {
  margin-bottom: 7px;
  border-radius: 7px;
  color: #606266;
  font-size: 14px;
  padding-left: 16px !important;
}
.main-menu :deep(.el-sub-menu__title) {
  margin-bottom: 7px;
  border-radius: 7px;
  color: #606266;
  font-size: 14px;
  padding-left: 16px !important;
}
.main-menu :deep(.el-sub-menu .el-menu) {
  background: transparent;
}
.main-menu :deep(.el-sub-menu .el-menu-item) {
  min-width: 0;
  padding-left: 47px !important;
  color: #73777f;
}
.main-menu :deep(.el-menu-item .el-icon) {
  font-size: 18px;
  margin-right: 13px;
}
.main-menu :deep(.el-sub-menu__title .el-icon) {
  font-size: 18px;
  margin-right: 13px;
}
.main-menu :deep(.el-menu-item:hover),
.main-menu :deep(.el-sub-menu__title:hover) {
  background: #f5f7fa;
}
.main-menu :deep(.el-menu-item.is-active) {
  color: #409eff;
  background: #ecf5ff;
  font-weight: 600;
}
.main-menu :deep(.el-sub-menu.is-active > .el-sub-menu__title) {
  color: #409eff;
  font-weight: 600;
}
.main-menu.el-menu--collapse {
  width: 76px;
}
.main-menu.el-menu--collapse :deep(.el-menu-item) {
  padding-left: 15px !important;
}
.main-menu.el-menu--collapse :deep(.el-sub-menu__title) {
  padding-left: 15px !important;
}
.sidebar-bottom {
  flex-shrink: 0;
  margin: auto 20px 24px;
  padding: 20px 15px;
  border: 1px solid #e5efff;
  border-radius: 10px;
  background: linear-gradient(145deg, #f4f8ff, #fafcff);
  font-size: 12px;
}
.sidebar-bottom b {
  display: block;
  margin: 10px 0 5px;
  color: #546479;
}
.sidebar-bottom p {
  font-size: 11px;
  color: #a0aab8;
  margin-bottom: 14px;
}
.sidebar-app-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 7px;
  background: white;
  color: #409eff;
  font-size: 18px;
}
.workspace {
  margin-left: 232px;
  min-width: 0;
  transition: margin 0.2s;
}
.is-collapsed .workspace {
  margin-left: 76px;
}
.is-collapsed .brand {
  padding: 0 20px;
}
.topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 28px;
  background: white;
  border-bottom: 1px solid #ebeef5;
  position: sticky;
  top: 0;
  z-index: 10;
  gap: 16px;
}
.topbar-left,
.topbar-right {
  display: flex;
  align-items: center;
  gap: 15px;
}
.topbar-left :deep(.el-breadcrumb) {
  font-size: 13px;
}
.topbar-left :deep(.el-button) {
  font-size: 19px;
  color: #909399;
}
.workspace-tag:deep(.el-tag) {
  color: #909399;
  border-color: #e7ebf1;
  font-size: 11px;
}
.account-menu {
  display: flex;
  align-items: center;
  gap: 9px;
  cursor: pointer;
  outline: none;
  color: #606266;
  font-size: 13px;
}
.account-menu :deep(.el-avatar) {
  background: #ecf5ff;
  color: #409eff;
}
.account-menu > :deep(.el-icon) {
  color: #a8abb2;
}
main {
  width: 100%;
  max-width: 1600px;
  margin: 0 auto;
  padding: 30px 30px 24px;
}
footer {
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  max-width: 1600px;
  width: 100%;
  padding: 0 30px 22px;
  margin: auto;
  color: #b2b8c2;
  font-size: 11px;
}
footer a {
  color: #a1abb9;
  text-decoration: none;
}
@media (min-width: 1500px) {
  main {
    padding-top: 35px;
  }
}
@media (max-width: 1200px) {
  main {
    padding: 24px;
  }
}
@media (max-width: 1200px) {
  .topbar {
    padding: 0 24px;
  }
}
@media (max-width: 800px) {
  .sidebar,
  .is-collapsed .sidebar {
    width: 64px !important;
  }
}
@media (max-width: 800px) {
  .sidebar .brand {
    padding: 0 14px;
    height: 64px;
  }
}
@media (max-width: 800px) {
  .sidebar .brand > span:last-child:not(.brand-symbol),
  .workspace-label,
  .sidebar-bottom {
    display: none;
  }
}
@media (max-width: 800px) {
  .main-menu.el-menu {
    width: 64px;
    padding: 12px 7px;
  }
}
@media (max-width: 800px) {
  .main-menu :deep(.el-menu-item) {
    padding: 0 !important;
    justify-content: center;
    font-size: 0;
  }
}
@media (max-width: 800px) {
  .main-menu :deep(.el-menu-item .el-icon) {
    font-size: 20px;
    margin: 0;
  }
}
@media (max-width: 800px) {
  .workspace,
  .is-collapsed .workspace {
    margin-left: 64px;
  }
}
@media (max-width: 800px) {
  .topbar {
    padding: 0 16px;
    height: 58px !important;
  }
}
@media (max-width: 800px) {
  .topbar-left > :deep(.el-button),
  .workspace-tag,
  .topbar-right > :deep(.el-divider) {
    display: none;
  }
}
@media (max-width: 800px) {
  .topbar-left :deep(.el-breadcrumb) {
    font-size: 12px;
  }
}
@media (max-width: 800px) {
  .topbar-right {
    gap: 0;
  }
}
@media (max-width: 800px) {
  .account-menu > span:not(:deep(.el-avatar)) {
    display: none;
  }
}
@media (max-width: 800px) {
  main {
    padding: 22px 16px;
  }
}
@media (max-width: 800px) {
  footer {
    padding: 0 16px 22px;
  }
}
</style>
