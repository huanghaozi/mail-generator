<template>
  <a-layout style="min-height: 100vh">
    <a-layout-sider v-model:collapsed="collapsed" collapsible>
      <div class="logo">MailGen</div>
      <a-menu v-model:selectedKeys="selectedKeys" theme="dark" mode="inline">
        <a-menu-item key="/domains">
          <router-link to="/domains">
            <global-outlined />
            <span>{{ $t('menu.domains') }}</span>
          </router-link>
        </a-menu-item>
        <a-menu-item key="/accounts">
          <router-link to="/accounts">
            <user-outlined />
            <span>{{ $t('menu.accounts') }}</span>
          </router-link>
        </a-menu-item>
        <a-menu-item key="/logs">
          <router-link to="/logs">
            <file-text-outlined />
            <span>{{ $t('menu.logs') }}</span>
          </router-link>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>
    <a-layout>
      <a-layout-header style="background: #fff; padding: 0 16px; display: flex; justify-content: flex-end; align-items: center">
        <a-dropdown>
          <a class="ant-dropdown-link" @click.prevent>
            <translation-outlined /> {{ locale === 'zh' ? '简体中文' : 'English' }}
          </a>
          <template #overlay>
            <a-menu>
              <a-menu-item @click="changeLocale('zh')">简体中文</a-menu-item>
              <a-menu-item @click="changeLocale('en')">English</a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </a-layout-header>
      <a-layout-content style="margin: 16px">
        <div :style="{ padding: '24px', background: '#fff', minHeight: '360px' }">
          <router-view></router-view>
        </div>
      </a-layout-content>
      <a-layout-footer style="text-align: center">
        Mail Generator ©2025 Created by HH
      </a-layout-footer>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { GlobalOutlined, UserOutlined, FileTextOutlined, TranslationOutlined } from '@ant-design/icons-vue';
import { useI18n } from 'vue-i18n';

const route = useRoute();
const collapsed = ref(false);
const selectedKeys = ref<string[]>([route.path]);
const { locale } = useI18n();

watch(
  () => route.path,
  (newPath) => {
    selectedKeys.value = [newPath];
  }
);

const changeLocale = (lang: string) => {
  locale.value = lang;
  localStorage.setItem('locale', lang);
};
</script>

<style scoped>
.logo {
  height: 32px;
  margin: 16px;
  background: rgba(255, 255, 255, 0.3);
  color: white;
  text-align: center;
  line-height: 32px;
  font-weight: bold;
}
.ant-dropdown-link {
  color: rgba(0, 0, 0, 0.65);
  font-size: 14px;
  cursor: pointer;
}
</style>
