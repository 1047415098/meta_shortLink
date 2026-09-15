<template>
  <main class="unavailable">
    <h1>Link unavailable</h1>
    <p>This link is disabled or is no longer available.</p>
  </main>
</template>

<script setup>
import { onMounted } from "vue";
onMounted(() => {
  document.title = "Link unavailable";
});
</script>

<style scoped>
.unavailable {
  max-width: 600px;
  margin: 15vh auto;
  padding: 32px;
}
h1 {
  font-size: 32px;
  line-height: 1.2;
}
p {
  color: var(--muted);
}
</style>
