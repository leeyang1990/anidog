<template>
  <n-card>
    <n-form
      ref="formRef"
      :model="formValue"
      :rules="rules"
      label-placement="left"
      label-width="auto"
      require-mark-placement="right-hanging"
      size="large"
    >
      <n-form-item label="用户名" path="username">
        <n-input
          v-model:value="formValue.username"
          placeholder="请输入用户名"
          @keydown.enter="handleSubmit"
        />
      </n-form-item>
      <n-form-item label="密码" path="password">
        <n-input
          v-model:value="formValue.password"
          type="password"
          placeholder="请输入密码"
          @keydown.enter="handleSubmit"
        />
      </n-form-item>
      <div class="flex justify-between items-center mt-4">
        <n-checkbox v-model:checked="rememberMe">
          记住我
        </n-checkbox>
        <router-link
          to="/auth/register"
          class="text-sm text-blue-600 hover:text-blue-800"
        >
          没有账号？立即注册
        </router-link>
      </div>
      <div class="mt-6">
        <n-button
          type="primary"
          block
          :loading="loading"
          @click="handleSubmit"
        >
          登录
        </n-button>
      </div>
    </n-form>
  </n-card>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { useMessage } from 'naive-ui'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const message = useMessage()

const formRef = ref(null)
const loading = ref(false)
const rememberMe = ref(false)

const formValue = ref({
  username: '',
  password: ''
})

const rules = {
  username: {
    required: true,
    message: '请输入用户名',
    trigger: 'blur'
  },
  password: {
    required: true,
    message: '请输入密码',
    trigger: 'blur'
  }
}

async function handleSubmit() {
  try {
    await formRef.value?.validate()
    loading.value = true
    
    console.log('尝试登录:', formValue.value.username)
    
    const success = await authStore.login({
      username: formValue.value.username,
      password: formValue.value.password,
      remember: rememberMe.value
    })

    if (success) {
      console.log('登录成功，令牌:', authStore.token ? authStore.token.substring(0, 10) + '...' : '无令牌')
      message.success('登录成功')
      const redirectPath = route.query.redirect || '/'
      router.push(redirectPath)
    }
  } catch (error) {
    console.error('登录失败:', error)
    if (error?.message) {
      message.error(error.message)
    }
  } finally {
    loading.value = false
  }
}
</script> 