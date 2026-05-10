<template>
  <n-config-provider :theme="isDark ? darkTheme : undefined" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <n-loading-bar-provider>
          <app-initializer />
          <router-view />
        </n-loading-bar-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup>
import { defineComponent, watch, onMounted, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from './stores/auth'
import { darkTheme, NLoadingBarProvider, useLoadingBar } from 'naive-ui'

const isDark = computed(() => document.documentElement.classList.contains('dark'))

const themeOverrides = computed(() => {
  const common = {
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Helvetica Neue', Helvetica, Arial, sans-serif",
    borderRadius: '0.625rem',
  }
  if (isDark.value) {
    return {
      common: {
        ...common,
        primaryColor: '#818cf8',
        primaryColorHover: '#a5b4fc',
        primaryColorPressed: '#6366f1',
        primaryColorSuppl: '#818cf8',
        bodyColor: '#09090b',
        cardColor: '#09090b',
        modalColor: '#09090b',
        popoverColor: '#09090b',
        textColorBase: '#fafafa',
        borderColor: '#27272a',
        dividerColor: '#27272a',
        inputColor: '#27272a',
        tableColor: '#09090b',
        hoverColor: '#27272a',
        actionColor: '#18181b',
      },
      Card: { color: '#09090b', borderColor: '#27272a', borderRadius: '0.625rem' },
      Modal: { color: '#09090b' },
      Dropdown: { color: '#18181b', optionColorHover: '#27272a' },
      Input: { color: '#09090b', borderColor: '#27272a', borderRadius: '0.625rem' },
      Button: { borderRadiusMedium: '0.625rem' },
    }
  }
  return {
    common: {
      ...common,
      primaryColor: '#667eea',
      primaryColorHover: '#818cf8',
      primaryColorPressed: '#4f46e5',
      primaryColorSuppl: '#667eea',
      bodyColor: '#ffffff',
      cardColor: '#ffffff',
      modalColor: '#ffffff',
      popoverColor: '#ffffff',
      textColorBase: '#09090b',
      borderColor: '#e4e4e7',
      dividerColor: '#e4e4e7',
      inputColor: '#ffffff',
      tableColor: '#ffffff',
      hoverColor: '#f4f4f5',
      actionColor: '#f4f4f5',
    },
    Card: { color: '#ffffff', borderColor: '#e4e4e7', borderRadius: '0.625rem' },
    Modal: { color: '#ffffff' },
    Dropdown: { color: '#ffffff', optionColorHover: '#f4f4f5' },
    Input: { color: '#ffffff', borderColor: '#e4e4e7', borderRadius: '0.625rem' },
    Button: { borderRadiusMedium: '0.625rem' },
  }
})

const AppInitializer = defineComponent({
  setup() {
    const authStore = useAuthStore()
    const router = useRouter()
    const loadingBar = useLoadingBar()
    const initialized = ref(false)

    onMounted(async () => {
      loadingBar.start()

      if (authStore.isLoggedIn) {
        try {
          await authStore.fetchUserInfo()
        } catch (error) {
          authStore.logout()
          if (router.currentRoute.value.meta.requiresAuth) {
            router.push({ name: 'Login' })
          }
        }
      }

      initialized.value = true
      loadingBar.finish()
    })

    return () => null
  }
})
</script>

<style>
html, body { margin: 0; padding: 0; height: 100%; width: 100%; }
#app { height: 100vh; }
</style>
