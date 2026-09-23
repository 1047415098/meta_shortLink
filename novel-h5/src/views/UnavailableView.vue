<script setup>import { useI18n } from "vue-i18n";defineProps({ error:{ type:Object, required:true } });const {t}=useI18n({useScope:"global"});</script>
<template><section class="state-page"><i class="fa-solid fa-book-open" /><h1>{{ t('restingTitle') }}</h1><p>{{ t('restingMessage') }}</p></section></template>
