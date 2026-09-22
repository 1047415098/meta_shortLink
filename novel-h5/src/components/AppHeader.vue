<script setup>
import { useRoute, useRouter } from "vue-router";
const props = defineProps({ back:Boolean }); const route = useRoute(), router = useRouter();
</script>
<template><header class="app-header"><button v-if="back" class="icon-button" aria-label="Go back" @click="router.back()"><i class="fa-solid fa-chevron-left" /></button><RouterLink v-else class="brand-mark" :to="{ name:'home', params:{ code:route.params.code } }" aria-label="Story home"><i class="fa-solid fa-book-open" /></RouterLink><span class="header-spacer" /><RouterLink class="icon-button" :to="{ name:'search', params:{ code:route.params.code } }" aria-label="Search stories"><i class="fa-solid fa-magnifying-glass" /></RouterLink><!-- 页面可在搜索按钮后追加同一行操作，避免绝对定位重叠。 --><slot /></header></template>
