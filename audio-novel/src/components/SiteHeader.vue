<script setup>
import { inject, ref } from "vue";
const bootstrap = inject("bootstrap");
const open = ref(false);
</script>

<template>
  <header class="site-header">
    <RouterLink class="brand" :to="{ name: 'home', params: { code: bootstrap.link.code } }">
      <span class="brand-mark">✦</span>
      <span>The Lantern Archive</span>
    </RouterLink>
    <button class="menu-toggle" type="button" :aria-expanded="open" aria-label="Toggle navigation" @click="open = !open">Menu</button>
    <nav :class="{ open }" aria-label="Primary navigation">
      <RouterLink :to="{ name: 'home', params: { code: bootstrap.link.code } }" @click="open = false">Home</RouterLink>
      <RouterLink :to="{ name: 'stories', params: { code: bootstrap.link.code } }" @click="open = false">Stories</RouterLink>
    </nav>
  </header>
</template>
