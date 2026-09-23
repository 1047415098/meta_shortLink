<script setup>
import { computed, inject } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
const props = defineProps({ back:Boolean, showLanguage:Boolean, showSearch:{ type:Boolean, default:true } }); const route = useRoute(), router = useRouter();
const { locale, t } = useI18n({ useScope:"global" });
const { availableLocales, setLocale } = inject("localeController");
// 入口只显示当前语言，完整列表由轻量 Popover 承载。
const currentLanguage = computed(() => availableLocales.value.find((option) => option.code === locale.value) || availableLocales.value[0]);
</script>
<template>
  <header class="app-header">
    <button v-if="back" class="icon-button" :aria-label="t('goBack')" @click="router.back()"><i class="fa-solid fa-chevron-left" /></button>
    <RouterLink v-else class="brand-mark" :to="{ name:'home', params:{ code:route.params.code } }" :aria-label="t('storyHome')"><i class="fa-solid fa-book-open" /></RouterLink>
    <span class="header-spacer" />
    <div v-if="showLanguage" class="language-menu">
      <button class="language-trigger" type="button" popovertarget="language-popover" aria-haspopup="menu" :aria-label="t('chooseLanguage')">
        <span class="language-trigger-icon"><i class="fa-solid fa-globe" aria-hidden="true" /></span>
        <span class="language-current-name">{{ currentLanguage.name }}</span>
        <i class="language-trigger-chevron fa-solid fa-chevron-down" aria-hidden="true" />
      </button>
      <div id="language-popover" class="language-popover" popover="auto" role="menu" :aria-label="t('chooseLanguage')">
        <div class="language-popover-title"><span>{{ t('chooseLanguage') }}</span><i class="fa-solid fa-language" aria-hidden="true" /></div>
        <!-- Popover 原生支持点击外部和 Esc 关闭；语言按钮同时负责收起菜单。 -->
        <button
          v-for="option in availableLocales"
          :key="option.code"
          class="language-option"
          :class="{ selected:option.code===locale }"
          type="button"
          role="menuitemradio"
          :aria-checked="option.code===locale"
          popovertarget="language-popover"
          popovertargetaction="hide"
          @click="setLocale(option.code)"
        >
          <span class="language-code">{{ option.code.toUpperCase() }}</span>
          <span class="language-name">{{ option.name }}</span>
          <i v-if="option.code===locale" class="fa-solid fa-check" aria-hidden="true" />
        </button>
      </div>
    </div>
    <RouterLink v-if="showSearch" class="icon-button" :to="{ name:'search', params:{ code:route.params.code } }" :aria-label="t('searchStories')"><i class="fa-solid fa-magnifying-glass" /></RouterLink>
    <!-- 页面可在搜索按钮后追加同一行操作，避免绝对定位重叠。 -->
    <slot />
  </header>
</template>
