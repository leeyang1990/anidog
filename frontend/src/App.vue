<template>
  <router-view />
  <AcToastContainer />
  <AcConfirmHost />
  <AcLoadingBar />
</template>

<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from './stores/auth'
import { AcToastContainer, AcConfirmHost, AcLoadingBar } from './components/ac'
import { useLoadingBar } from './composables/useLoadingBar'

const authStore = useAuthStore()
const router = useRouter()
const loadingBar = useLoadingBar()

onMounted(async () => {
  loadingBar.start()
  if (authStore.isLoggedIn) {
    try {
      await authStore.fetchUserInfo()
    } catch (error) {
      authStore.logout()
      if (router.currentRoute.value.meta?.requiresAuth) {
        router.push({ name: 'Login' })
      }
    }
  }
  loadingBar.finish()
})
</script>

<style>
html, body { margin: 0; padding: 0; height: 100%; width: 100%; }
#app { height: 100vh; }
</style>
