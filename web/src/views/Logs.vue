<template>
  <div>
    <a-button type="default" @click="fetchLogs" style="margin-bottom: 16px">{{ $t('common.refresh') }}</a-button>
    <a-table 
      :dataSource="logs" 
      :columns="columns" 
      rowKey="id"
      :pagination="pagination"
      @change="handleTableChange"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status'">
          <a-tag :color="record.status === 'success' ? 'green' : 'red'">{{ record.status }}</a-tag>
        </template>
        <template v-if="column.key === 'action'">
          <a @click="showContent(record)">{{ $t('log.viewContent') }}</a>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="open" :title="$t('log.contentTitle')" width="800px" :footer="null">
      <pre style="white-space: pre-wrap; word-wrap: break-word; max-height: 600px; overflow-y: auto;">{{ currentContent }}</pre>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import request from '../api/request';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const logs = ref([]);
const pagination = ref({
  current: 1,
  pageSize: 20,
  total: 0,
});
const open = ref(false);
const currentContent = ref('');

const columns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id' },
  { title: t('log.from'), dataIndex: 'from', key: 'from' },
  { title: t('log.to'), dataIndex: 'to', key: 'to' },
  { title: t('log.subject'), dataIndex: 'subject', key: 'subject' },
  { title: t('log.status'), dataIndex: 'status', key: 'status' },
  { title: t('log.time'), dataIndex: 'created_at', key: 'created_at' },
  { title: t('common.action'), key: 'action' },
]);

const fetchLogs = async () => {
  const res: any = await request.get('/logs', {
    params: {
      page: pagination.value.current,
      pageSize: pagination.value.pageSize
    }
  });
  logs.value = res.data;
  pagination.value.total = res.total;
};

const handleTableChange = (pag: any) => {
  pagination.value.current = pag.current;
  pagination.value.pageSize = pag.pageSize;
  fetchLogs();
};

const showContent = (record: any) => {
  currentContent.value = record.content || 'No Content';
  open.value = true;
};

onMounted(fetchLogs);
</script>
