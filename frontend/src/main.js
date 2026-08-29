import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import './assets/tailwind.css'
import { installPreloadRecovery } from './utils/preloadRecovery'
// 引入即应用 localStorage 里保存的皮肤（在 mount 前 set data-skin，避免初始闪烁）
import './composables/useSkin'

installPreloadRecovery()

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(i18n)

app.mount('#app')
