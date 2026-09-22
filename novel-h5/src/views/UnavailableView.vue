<script setup>defineProps({ error:{ type:Object, required:true } });</script>
<template><section class="state-page"><i class="fa-solid fa-book-open" /><h1>Stories are resting</h1><p>{{ error.message }}</p></section></template>
