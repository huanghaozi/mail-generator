<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import enUS from 'ant-design-vue/es/locale/en_US';
import zhCN from 'ant-design-vue/es/locale/zh_CN';

const { locale } = useI18n();
const antdLocale = computed(() => (locale.value === 'zh' ? zhCN : enUS));
</script>

<template>
  <a-config-provider :locale="antdLocale">
    <router-view></router-view>
  </a-config-provider>
</template>

<style>
/* Global styles are handled in style.css */
</style>
