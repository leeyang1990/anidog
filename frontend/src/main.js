import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './assets/tailwind.css'
// 引入即应用 localStorage 里保存的皮肤（在 mount 前 set data-skin，避免初始闪烁）
import './composables/useSkin'

const app = createApp(App)

app.use(createPinia())
app.use(router)

app.mount('#app')
