<template>
  <div>
    <div style="margin-bottom: 16px">
      <a-button type="primary" @click="showModal">{{ $t('account.addTitle') }}</a-button>
    </div>
    <a-table :dataSource="accounts" :columns="columns" rowKey="id">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'action'">
          <a-popconfirm :title="$t('common.confirmDelete')" @confirm="deleteAccount(record.id)">
            <a>{{ $t('common.delete') }}</a>
          </a-popconfirm>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="open" :title="$t('account.addTitle')" @ok="handleOk">
      <a-form layout="vertical">
        <a-form-item :label="$t('account.pattern')">
          <a-input v-model:value="form.pattern" :placeholder="$t('account.patternPlaceholder')" />
          <small>{{ $t('account.patternTip') }}</small>
        </a-form-item>
        <a-form-item :label="$t('account.forwardTo')">
          <a-input v-model:value="form.forward_to" :placeholder="$t('account.forwardToPlaceholder')" />
        </a-form-item>
        <a-form-item :label="$t('common.description')">
          <a-input v-model:value="form.description" />
        </a-form-item>
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
const accounts = ref([]);
const open = ref(false);
const form = reactive({ pattern: '', forward_to: '', description: '' });

const columns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id' },
  { title: t('account.pattern'), dataIndex: 'pattern', key: 'pattern' },
  { title: t('account.forwardTo'), dataIndex: 'forward_to', key: 'forward_to' },
  { title: t('account.hitCount'), dataIndex: 'hit_count', key: 'hit_count' },
  { title: t('common.description'), dataIndex: 'description', key: 'description' },
  { title: t('common.action'), key: 'action' },
]);

const fetchAccounts = async () => {
  accounts.value = await request.get('/accounts');
};

const showModal = () => {
  open.value = true;
};

const handleOk = async () => {
  if (!form.pattern || !form.forward_to) return;
  await request.post('/accounts', form);
  message.success(t('account.added'));
  open.value = false;
  fetchAccounts();
};

const deleteAccount = async (id: number) => {
  await request.delete(`/accounts/${id}`);
  message.success(t('account.deleted'));
  fetchAccounts();
};

onMounted(fetchAccounts);
</script>
