<template>
  <div class="min-h-screen bg-gray-100 flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8">
    <div class="max-w-md w-full space-y-8">
      <div class="text-center">
        <h1 class="text-3xl font-bold text-gray-900">
          Mikanani Dog
        </h1>
        <p class="mt-2 text-sm text-gray-600">
          您的私人动画管理助手
        </p>
      </div>
      <router-view />
    </div>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { watch } from 'vue'

const router = useRouter()
const authStore = useAuthStore()

// 如果用户已登录，重定向到主页面
watch(
  () => authStore.isLoggedIn,
  (isLoggedIn) => {
    if (isLoggedIn) {
      const redirectPath = router.currentRoute.value.query.redirect || '/'
      router.push(redirectPath)
    }
  },
  { immediate: true }
)
</script> 