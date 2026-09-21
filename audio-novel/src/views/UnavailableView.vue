<script setup>
defineProps({ error: { type: Object, required: true } });
</script>

<template>
  <main class="error-stage">
    <section>
      <p class="eyebrow">Archive notice · {{ error.status }}</p>
      <h1>The road ends here.</h1>
      <p>{{ error.message }}</p>
    </section>
  </main>
</template>
