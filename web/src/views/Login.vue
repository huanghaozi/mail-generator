<template>
  <div class="login-container">
    <a-card :title="$t('login.title')" style="width: 300px">
      <a-form :model="formState" @finish="onFinish">
        <a-form-item
          name="password"
          :rules="[{ required: true, message: $t('login.passwordPlaceholder') }]"
        >
          <a-input-password v-model:value="formState.password" :placeholder="$t('login.password')" />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" html-type="submit" block :loading="loading">{{ $t('login.loginButton') }}</a-button>
        </a-form-item>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import request from '../api/request';

const router = useRouter();
const loading = ref(false);
const formState = reactive({
  password: '',
});

const onFinish = async (values: any) => {
  loading.value = true;
  try {
    const res: any = await request.post('/login', values);
    localStorage.setItem('token', res.token);
    router.push('/');
  } finally {
    loading.value = false;
  }
};
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100vh;
  background-color: #f0f2f5;
}
</style>
