<template>
  <div>
    <div style="margin-bottom: 16px">
      <a-button type="primary" @click="showModal">{{ $t('domain.addTitle') }}</a-button>
    </div>
    <a-table :dataSource="domains" :columns="columns" rowKey="id">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'action'">
          <a-popconfirm :title="$t('common.confirmDelete')" @confirm="deleteDomain(record.id)">
            <a>{{ $t('common.delete') }}</a>
          </a-popconfirm>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="open" :title="$t('domain.addTitle')" @ok="handleOk">
      <a-form layout="vertical">
        <a-form-item :label="$t('domain.name')">
          <a-input v-model:value="form.name" :placeholder="$t('domain.namePlaceholder')" />
        </a-form-item>
        <p>{{ $t('domain.instruction') }}</p>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue';
import request from '../api/request';
import { message } from 'ant-design-vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const domains = ref([]);
const open = ref(false);
const form = reactive({ name: '' });

const columns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id' },
  { title: t('domain.name'), dataIndex: 'name', key: 'name' },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at' },
  { title: t('common.action'), key: 'action' },
]);

const fetchDomains = async () => {
  domains.value = await request.get('/domains');
};

const showModal = () => {
  open.value = true;
};

const handleOk = async () => {
  if (!form.name) return;
  await request.post('/domains', form);
  message.success(t('domain.added'));
  open.value = false;
  form.name = '';
  fetchDomains();
};

const deleteDomain = async (id: number) => {
  await request.delete(`/domains/${id}`);
  message.success(t('domain.deleted'));
  fetchDomains();
};

onMounted(fetchDomains);
</script>
