<template>
  <div class="min-h-screen flex items-center justify-center bg-background p-4">
    <div class="w-full max-w-sm bg-card text-card-foreground rounded-lg border p-8">
      <!-- Header -->
      <div class="text-center mb-8">
        <div class="h-10 w-10 rounded-md bg-primary flex items-center justify-center mx-auto mb-4">
          <n-icon size="22" color="#fff"><FilmOutline /></n-icon>
        </div>
        <h1 class="text-2xl font-semibold tracking-tight">御宅追番</h1>
        <p class="text-sm text-muted-foreground mt-1">欢迎回来，继续追番之旅</p>
      </div>

      <!-- Form -->
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
          <label class="text-sm font-medium">密码</label>
          <div class="relative">
            <n-icon size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"><LockClosedOutline /></n-icon>
            <input v-model="formValue.password" type="password" required minlength="6" placeholder="请输入密码"
              class="h-9 w-full rounded-md border border-input bg-background pl-9 pr-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </div>

        <div class="flex items-center gap-2">
          <input type="checkbox" v-model="rememberMe" id="remember" class="rounded border-input" />
          <label for="remember" class="text-sm text-muted-foreground">记住我</label>
        </div>

        <button type="submit" :disabled="loading"
          class="w-full bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-10 text-sm font-medium transition-colors disabled:opacity-50">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>

      <p class="text-center text-sm text-muted-foreground mt-6">
        还没有账户?
        <router-link to="/auth/register" class="text-primary font-medium hover:underline">立即注册</router-link>
      </p>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { useMessage } from 'naive-ui'
import { NIcon } from 'naive-ui'
import { PersonOutline, LockClosedOutline, FilmOutline } from '@vicons/ionicons5'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const message = useMessage()

const loading = ref(false)
const rememberMe = ref(false)

const formValue = reactive({
  username: '',
  password: ''
})

const handleSubmit = async () => {
  loading.value = true
  try {
    await authStore.login({
      username: formValue.username,
      password: formValue.password,
      remember: rememberMe.value
    })
    message.success('登录成功')
    const redirectPath = route.query.redirect || '/'
    router.push(redirectPath)
  } catch (error) {
    message.error(error.message || '登录失败，请检查用户名和密码')
  } finally {
    loading.value = false
  }
}
</script>
