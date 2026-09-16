<template>
  <component :is="page" />
</template>

<script setup>
import LandingView from "./views/LandingView.vue";
import UnavailableView from "./views/UnavailableView.vue";
import { landingData } from "./bootstrap.js";

// The Go bootstrap is authoritative for choosing the one screen rendered by
// this single-purpose landing bundle.
const page =
  landingData.error || !landingData.link ? UnavailableView : LandingView;
</script>

<style scoped></style>
