<template>
  <n-config-provider>
    <n-message-provider>
      <n-loading-bar-provider>
        <app-initializer />
      <router-view />
      </n-loading-bar-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup>
import { defineComponent, watch, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from './stores/auth'
import { darkTheme, NLoadingBarProvider, useLoadingBar } from 'naive-ui'

// 应用程序初始化组件
const AppInitializer = defineComponent({
  setup() {
const authStore = useAuthStore()
const router = useRouter()
    const loadingBar = useLoadingBar()
    const initialized = ref(false)
    
    onMounted(async () => {
      console.log('应用程序启动，检查认证状态')
      loadingBar.start()
      
      if (authStore.isLoggedIn) {
        try {
          console.log('检测到令牌，尝试获取用户信息')
          await authStore.fetchUserInfo()
          console.log('成功获取用户信息:', authStore.user)
        } catch (error) {
          console.error('获取用户信息失败，将重定向到登录页面', error)
          authStore.logout()
          if (router.currentRoute.value.meta.requiresAuth) {
      router.push({ name: 'Login' })
    }
        }
      } else {
        console.log('未检测到令牌，用户未登录')
      }
      
      initialized.value = true
      loadingBar.finish()
    })
    
    return () => null
  }
})
</script>

<style>
html, body {
  margin: 0;
  padding: 0;
  height: 100%;
  width: 100%;
}

#app {
  height: 100vh;
}
</style> 