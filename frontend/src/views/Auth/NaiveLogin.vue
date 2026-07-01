<template>
  <div class="min-h-screen flex items-center justify-center bg-background ac-grass-pattern p-4 relative overflow-hidden">
    <!-- 背景装饰：飘过的叶子 -->
    <div class="absolute -left-12 top-12 w-32 h-32 text-ac-grass-light opacity-50 animate-bounce-soft" aria-hidden="true">
      <svg viewBox="0 0 24 24" fill="currentColor" class="w-full h-full"><path d="M12 2 C 5 5, 4 14, 12 22 C 20 14, 19 5, 12 2 Z" /></svg>
    </div>
    <div class="absolute -right-10 bottom-16 w-24 h-24 text-ac-sun opacity-40 animate-wiggle" aria-hidden="true">
      <svg viewBox="0 0 24 24" fill="currentColor" class="w-full h-full"><circle cx="12" cy="12" r="6" /></svg>
    </div>

    <AcCard padding="lg" rounded="3xl" shadow="lg" class="w-full max-w-sm bg-card border-2 border-ac-sand">
      <!-- Header -->
      <div class="text-center mb-7">
        <div class="inline-flex items-center justify-center size-16 rounded-3xl bg-ac-grass-light/40 border-2 border-ac-grass shadow-md mb-4">
          <img src="@/assets/logo.svg" alt="AniDog" class="size-12" />
        </div>
        <h1 class="text-3xl font-bold tracking-tight text-foreground">AniDog</h1>
        <p class="text-sm text-muted-foreground mt-1.5">欢迎回来，继续追番之旅 🌿</p>
      </div>

      <!-- Form -->
      <form @submit.prevent="handleSubmit" class="space-y-4">
        <div class="space-y-1.5">
          <label class="text-sm font-bold text-foreground">用户名</label>
          <AcInput v-model="formValue.username" placeholder="请输入用户名" required size="lg" autocomplete="username">
            <template #prefix>
              <PersonOutline class="size-4" />
            </template>
          </AcInput>
        </div>

        <div class="space-y-1.5">
          <label class="text-sm font-bold text-foreground">密码</label>
          <AcInput v-model="formValue.password" type="password" placeholder="请输入密码" required size="lg" autocomplete="current-password">
            <template #prefix>
              <LockClosedOutline class="size-4" />
            </template>
          </AcInput>
        </div>

        <AcCheckbox v-model="rememberMe">记住我</AcCheckbox>

        <AcButton type="submit" variant="primary" size="lg" block :loading="loading">
          {{ loading ? '登录中...' : '🐾 登录' }}
        </AcButton>
      </form>

      <p class="text-center text-sm text-muted-foreground mt-6">
        还没有账户？
        <router-link to="/auth/register" class="text-ac-grass-dark font-bold hover:underline">立即注册</router-link>
      </p>
    </AcCard>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { useToast } from '../../composables/useToast'
import { PersonOutline, LockClosedOutline } from '@vicons/ionicons5'
import { AcCard, AcInput, AcButton, AcCheckbox } from '../../components/ac'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const loading = ref(false)
const rememberMe = ref(false)

const formValue = reactive({
  username: '',
  password: '',
})

const handleSubmit = async () => {
  loading.value = true
  try {
    await authStore.login({
      username: formValue.username,
      password: formValue.password,
      remember: rememberMe.value,
    })
    toast.success('登录成功，欢迎回来~')
    const redirectPath = route.query.redirect || '/'
    router.push(redirectPath)
  } catch (error) {
    toast.error(error.message || '登录失败，请检查用户名和密码')
  } finally {
    loading.value = false
  }
}
</script>
