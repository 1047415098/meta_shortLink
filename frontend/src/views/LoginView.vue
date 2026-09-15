<template>
  <div class="login-shell">
    <section class="login-story">
      <div class="brand"><span class="brand-symbol">↗</span> LinkScope</div>
      <div>
        <p class="eyebrow">WHATSAPP · AD ANALYTICS</p>
        <h1>每一次点击，<br />都有迹可循。</h1>
        <p>连接广告与 WhatsApp，<br />看清流量的来源、分布与质量。</p>
      </div>
      <small>自有短链接 · 独立数据 · 清晰归因</small>
    </section>
    <section class="login-panel">
      <form @submit.prevent="signIn" class="login-form">
        <p class="eyebrow">管理控制台</p>
        <h2>欢迎回来</h2>
        <p class="muted">登录后查看你的链接与投放表现。</p>
        <el-alert
          v-if="error"
          :title="error"
          type="error"
          :closable="false"
        /><label for="username">账号</label
        ><el-input
          id="username"
          v-model="login.username"
          autocomplete="username"
          required
          placeholder="管理员账号"
          size="large"
        /><label for="password">密码</label
        ><el-input
          id="password"
          v-model="login.password"
          autocomplete="current-password"
          type="password"
          show-password
          required
          placeholder="输入密码"
          size="large"
        /><el-button
          native-type="submit"
          type="primary"
          size="large"
          :loading="saving"
          >登录后台 →</el-button
        ><small class="muted">账号由部署管理员配置，请妥善保管。</small>
      </form>
    </section>
  </div>
</template>

<script setup>
import { ref, reactive } from "vue";
import { useRouter } from "vue-router";

import { useRoute } from "vue-router";
import { login as authenticate } from "../stores/auth";
import { safeReturnPath } from "../utils/routes";
const router = useRouter(),
  route = useRoute();
const login = reactive({ username: "", password: "" }),
  saving = ref(false),
  error = ref(
    route.query.reason === "network" ? "暂时无法连接服务，请重试" : "",
  );
async function signIn() {
  saving.value = true;
  error.value = "";
  try {
    await authenticate(login);
    login.password = "";
    await router.replace(safeReturnPath(route.query.redirect));
  } catch (e) {
    error.value = e.message;
  } finally {
    saving.value = false;
  }
}
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
.is-collapsed .brand {
  padding: 0 20px;
}
.login-shell {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 1fr 1fr;
  background: #fff;
}
.login-story {
  padding: 55px 12%;
  background: linear-gradient(145deg, #f0f6ff, #eaf2ff);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  min-height: 600px;
  color: #263d5e;
}
.login-story .brand {
  padding: 0;
}
.login-story h1 {
  font-size: clamp(34px, 3.8vw, 58px);
  line-height: 1.45;
  color: #24426d;
  margin: 25px 0;
}
.login-story p:not(.eyebrow) {
  font-size: 15px;
  line-height: 1.9;
  color: #7d95b5;
}
.login-story small {
  color: #8e9fba;
  font-size: 12px;
}
.login-panel {
  display: grid;
  place-items: center;
  padding: 45px;
}
.login-form {
  width: 100%;
  max-width: 360px;
}
.login-form h2 {
  font-size: 29px;
  margin: 12px 0;
}
.login-form label {
  display: block;
  color: #606266;
  font-size: 13px;
  margin: 25px 0 9px;
}
.login-form > :deep(.el-button) {
  width: 100%;
  margin: 28px 0 18px;
  height: 42px;
}
.login-form > small {
  display: block;
  text-align: center;
  font-size: 11px;
}
.login-form :deep(.el-alert) {
  margin-top: 20px;
}
@media (max-width: 800px) {
  .sidebar .brand {
    padding: 0 14px;
    height: 64px;
  }
}
@media (max-width: 800px) {
  .sidebar .brand > span:last-child:not(.brand-symbol) {
    display: none;
  }
}
@media (max-width: 800px) {
  .login-shell {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 800px) {
  .login-story {
    min-height: 0;
    padding: 20px;
  }
}
@media (max-width: 800px) {
  .login-story > div:nth-child(2),
  .login-story > small {
    display: none;
  }
}
@media (max-width: 800px) {
  .login-panel {
    padding: 40px 24px;
  }
}
</style>
