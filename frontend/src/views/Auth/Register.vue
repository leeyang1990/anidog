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
        />
      </n-form-item>
      <n-form-item label="密码" path="password">
        <n-input
          v-model:value="formValue.password"
          type="password"
          placeholder="请输入密码"
        />
      </n-form-item>
      <n-form-item label="确认密码" path="confirmPassword">
        <n-input
          v-model:value="formValue.confirmPassword"
          type="password"
          placeholder="请再次输入密码"
        />
      </n-form-item>
      <div class="flex justify-end mt-4">
        <router-link
          to="/auth/login"
          class="text-sm text-blue-600 hover:text-blue-800"
        >
          已有账号？立即登录
        </router-link>
      </div>
      <div class="mt-6">
        <n-button
          type="primary"
          block
          :loading="loading"
          @click="handleSubmit"
        >
          注册
        </n-button>
      </div>
    </n-form>
  </n-card>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'

const router = useRouter()
const message = useMessage()

const formRef = ref(null)
const loading = ref(false)

const formValue = ref({
  username: '',
  password: '',
  confirmPassword: ''
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
  },
  confirmPassword: {
    required: true,
    message: '请确认密码',
    trigger: 'blur',
    validator: (rule, value) => {
      return value === formValue.value.password || new Error('两次输入的密码不一致')
    }
  }
}

async function handleSubmit() {
  try {
    await formRef.value?.validate()
    loading.value = true
    
    const response = await fetch('/api/v1/auth/register', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        username: formValue.value.username,
        password: formValue.value.password
      })
    })

    if (!response.ok) {
      const error = await response.json()
      throw new Error(error.message || '注册失败')
    }

    message.success('注册成功，请登录')
    router.push('/auth/login')
  } catch (error) {
    if (error?.message) {
      message.error(error.message)
    }
  } finally {
    loading.value = false
  }
}
</script> 