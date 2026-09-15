<template>
  <section class="not-found">
    <h1>页面不存在</h1>
    <p>请检查地址，或返回数据总览。</p>
    <router-link to="/admin/overview">返回数据总览</router-link>
  </section>
</template>

<script setup></script>

<style scoped>
.not-found {
  padding: 48px;
  text-align: center;
}
</style>
