<template>
  <div class="min-h-screen flex items-center justify-center bg-background p-4">
    <div class="w-full max-w-sm bg-card text-card-foreground rounded-lg border p-8">
      <div class="text-center mb-8">
        <img src="@/assets/logo.svg" alt="AniDog" class="h-12 w-12 mx-auto mb-4" />
        <h1 class="text-2xl font-semibold tracking-tight">AniDog</h1>
        <p class="text-sm text-muted-foreground mt-1">创建您的账户，开始追番之旅</p>
      </div>

      <form @submit.prevent="handleSubmit" class="space-y-4">
        <div class="space-y-2">
          <label class="text-sm font-medium">用户名</label>
          <div class="relative">
            <n-icon size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"><PersonOutline /></n-icon>
            <input v-model="formValue.username" type="text" required minlength="3" placeholder="请输入用户名"
              class="h-9 w-full rounded-md border border-input bg-background pl-9 pr-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">邮箱</label>
          <div class="relative">
            <n-icon size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"><MailOutline /></n-icon>
            <input v-model="formValue.email" type="email" required placeholder="请输入邮箱地址"
              class="h-9 w-full rounded-md border border-input bg-background pl-9 pr-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">密码</label>
          <div class="relative">
            <n-icon size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"><LockClosedOutline /></n-icon>
            <input v-model="formValue.password" type="password" required minlength="6" placeholder="请输入密码（至少6个字符）"
              class="h-9 w-full rounded-md border border-input bg-background pl-9 pr-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">确认密码</label>
          <div class="relative">
            <n-icon size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"><ShieldCheckmarkOutline /></n-icon>
            <input v-model="formValue.confirmPassword" type="password" required minlength="6" placeholder="请再次输入密码"
              class="h-9 w-full rounded-md border border-input bg-background pl-9 pr-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </div>

        <button type="submit" :disabled="loading"
          class="w-full bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-10 text-sm font-medium transition-colors disabled:opacity-50">
          {{ loading ? '注册中...' : '注册账户' }}
        </button>
      </form>

      <p class="text-center text-sm text-muted-foreground mt-6">
        已有账户?
        <router-link to="/auth/login" class="text-primary font-medium hover:underline">立即登录</router-link>
      </p>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { NIcon } from 'naive-ui'
import { post } from '@/utils/api'
import {
  PersonOutline, LockClosedOutline, MailOutline,
  ShieldCheckmarkOutline, FilmOutline
} from '@vicons/ionicons5'

const router = useRouter()
const message = useMessage()

const loading = ref(false)

const formValue = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: ''
})

const handleSubmit = async () => {
  if (formValue.password !== formValue.confirmPassword) {
    message.error('两次输入的密码不一致')
    return
  }

  loading.value = true
  try {
    await post('/auth/register', {
      username: formValue.username,
      email: formValue.email,
      password: formValue.password
    })
    message.success('注册成功，请登录')
    router.push('/auth/login')
  } catch (error) {
    message.error(error.message || '注册失败，请稍后重试')
  } finally {
    loading.value = false
  }
}
</script>
