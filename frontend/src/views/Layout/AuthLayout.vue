<template>
  <router-view />
</template>

<script setup>
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { watch } from 'vue'

const router = useRouter()
const authStore = useAuthStore()

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
