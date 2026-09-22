<script setup>
defineProps({ story:{ type:Object, required:true }, compact:Boolean });
</script>
<template><RouterLink class="story-card" :class="{ compact }" :to="{ name:'story', params:{ code:$route.params.code, slug:story.slug } }"><div class="cover-box"><img v-if="story.cover_path" :src="story.cover_path" :alt="`${story.title} cover`" loading="lazy" /><i v-else class="fa-solid fa-book-open" aria-hidden="true" /></div><div class="story-copy"><h3>{{ story.title }}</h3><p v-if="!compact">{{ story.excerpt }}</p><small v-else>{{ story.author || story.category }}</small></div></RouterLink></template>
