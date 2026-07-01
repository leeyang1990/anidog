<template>
  <div class="min-h-screen flex items-center justify-center bg-background ac-grass-pattern p-4 relative overflow-hidden">
    <div class="absolute -left-12 top-12 w-32 h-32 text-ac-grass-light opacity-50 animate-bounce-soft" aria-hidden="true">
      <svg viewBox="0 0 24 24" fill="currentColor" class="w-full h-full"><path d="M12 2 C 5 5, 4 14, 12 22 C 20 14, 19 5, 12 2 Z" /></svg>
    </div>

    <AcCard padding="lg" rounded="3xl" shadow="lg" class="w-full max-w-sm bg-card border-2 border-ac-sand">
      <div class="text-center mb-7">
        <div class="inline-flex items-center justify-center size-16 rounded-3xl bg-ac-sun/30 border-2 border-ac-sun shadow-md mb-4">
          <img src="@/assets/logo.svg" alt="AniDog" class="size-12" />
        </div>
        <h1 class="text-3xl font-bold tracking-tight text-foreground">注册新账号</h1>
        <p class="text-sm text-muted-foreground mt-1.5">来岛上一起追番吧 🐾</p>
      </div>

      <form @submit.prevent="handleSubmit" class="space-y-3.5">
        <div class="space-y-1.5">
          <label class="text-sm font-bold text-foreground">用户名</label>
          <AcInput v-model="formValue.username" placeholder="请输入用户名" required size="lg" autocomplete="username">
            <template #prefix><PersonOutline class="size-4" /></template>
          </AcInput>
        </div>

        <div class="space-y-1.5">
          <label class="text-sm font-bold text-foreground">邮箱</label>
          <AcInput v-model="formValue.email" type="email" placeholder="请输入邮箱地址" required size="lg" autocomplete="email">
            <template #prefix><MailOutline class="size-4" /></template>
          </AcInput>
        </div>

        <div class="space-y-1.5">
          <label class="text-sm font-bold text-foreground">密码</label>
          <AcInput v-model="formValue.password" type="password" placeholder="请输入密码（至少6位）" required size="lg" autocomplete="new-password">
            <template #prefix><LockClosedOutline class="size-4" /></template>
          </AcInput>
        </div>

        <div class="space-y-1.5">
          <label class="text-sm font-bold text-foreground">确认密码</label>
          <AcInput v-model="formValue.confirmPassword" type="password" placeholder="请再次输入密码" required size="lg" autocomplete="new-password">
            <template #prefix><ShieldCheckmarkOutline class="size-4" /></template>
          </AcInput>
        </div>

        <AcButton type="submit" variant="sun" size="lg" block :loading="loading" class="!mt-5">
          {{ loading ? '注册中...' : '✨ 创建账号' }}
        </AcButton>
      </form>

      <p class="text-center text-sm text-muted-foreground mt-6">
        已有账号？
        <router-link to="/auth/login" class="text-ac-grass-dark font-bold hover:underline">立即登录</router-link>
      </p>
    </AcCard>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '../../composables/useToast'
import { post } from '@/utils/api'
import {
  PersonOutline, LockClosedOutline, MailOutline, ShieldCheckmarkOutline,
} from '@vicons/ionicons5'
import { AcCard, AcInput, AcButton } from '../../components/ac'

const router = useRouter()
const toast = useToast()

const loading = ref(false)

const formValue = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
})

const handleSubmit = async () => {
  if (formValue.password !== formValue.confirmPassword) {
    toast.error('两次输入的密码不一致')
    return
  }
  loading.value = true
  try {
    await post('/auth/register', {
      username: formValue.username,
      email: formValue.email,
      password: formValue.password,
    })
    toast.success('注册成功，请登录~')
    router.push('/auth/login')
  } catch (error) {
    toast.error(error.message || '注册失败，请稍后重试')
  } finally {
    loading.value = false
  }
}
</script>
