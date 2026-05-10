<template>
  <div class="flex h-screen bg-background">
    <!-- Mobile overlay -->
    <div
      v-if="isMobile && !collapsed"
      class="fixed inset-0 bg-black/40 z-40 transition-opacity"
      @click="collapsed = true"
    />

    <!-- Sidebar -->
    <aside
      class="flex flex-col bg-sidebar border-r border-sidebar-border shrink-0 z-50 transition-all duration-300"
      :class="[
        collapsed ? 'w-16' : 'w-60',
        isMobile ? 'fixed inset-y-0 left-0' : ''
      ]"
      :style="isMobile && collapsed ? { transform: 'translateX(-100%)' } : {}"
    >
      <!-- Logo -->
      <div class="h-14 flex items-center px-4 border-b border-sidebar-border shrink-0">
        <div class="w-8 h-8 rounded-md bg-sidebar-primary flex items-center justify-center shrink-0">
          <n-icon size="18" color="#fff"><FilmOutline /></n-icon>
        </div>
        <span v-if="!collapsed" class="ml-3 text-sm font-semibold text-sidebar-foreground whitespace-nowrap">御宅追番</span>
      </div>

      <!-- Navigation -->
      <nav class="flex-1 py-2 overflow-y-auto">
        <a
          v-for="item in menuItems"
          :key="item.key"
          class="flex items-center gap-3 mx-2 h-10 px-3 rounded-md text-sm cursor-pointer transition-colors"
          :class="activeKey === item.key
            ? 'bg-sidebar-accent text-sidebar-primary font-medium'
            : 'text-sidebar-muted-foreground hover:text-sidebar-accent-foreground hover:bg-sidebar-accent'"
          @click="navigateTo(item.route)"
        >
          <n-icon size="18"><component :is="item.icon" /></n-icon>
          <span v-if="!collapsed" class="truncate">{{ item.label }}</span>
        </a>
      </nav>

      <!-- User section -->
      <div class="border-t border-sidebar-border p-3 shrink-0">
        <n-dropdown :options="userOptions" @select="handleUserSelect">
          <div class="flex items-center cursor-pointer">
            <div class="w-8 h-8 rounded-full bg-sidebar-primary flex items-center justify-center text-sidebar-primary-foreground text-xs font-semibold shrink-0">
              {{ authStore.user?.username?.charAt(0)?.toUpperCase() || 'U' }}
            </div>
            <div v-if="!collapsed" class="ml-3 flex-1 min-w-0">
              <p class="text-sm font-medium text-sidebar-foreground truncate">{{ authStore.user?.username || '用户' }}</p>
              <span class="text-xs text-sidebar-muted-foreground">{{ authStore.user?.is_admin ? '管理员' : '用户' }}</span>
            </div>
          </div>
        </n-dropdown>
      </div>
    </aside>

    <!-- Main area -->
    <div class="flex-1 flex flex-col min-w-0">
      <!-- Top bar -->
      <header class="h-14 border-b bg-background flex items-center gap-4 px-4 md:px-6 shrink-0">
        <button
          class="p-2 rounded-md hover:bg-accent text-muted-foreground transition-colors"
          @click="collapsed = !collapsed"
        >
          <n-icon size="20"><MenuOutline /></n-icon>
        </button>

        <h2 class="text-sm font-medium text-foreground hidden sm:block">{{ pageTitle }}</h2>

        <div class="flex-1" />

        <!-- Search -->
        <div class="relative hidden md:block">
          <n-icon size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"><SearchOutline /></n-icon>
          <input
            v-model="searchQuery"
            class="h-9 w-64 rounded-md border border-input bg-background pl-9 pr-4 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
            placeholder="搜索番剧..."
            @keydown.enter="handleSearch"
          />
        </div>

        <button
          class="p-2 rounded-md hover:bg-accent text-muted-foreground transition-colors"
          @click="toggleTheme"
        >
          <n-icon size="18">
            <SunnyOutline v-if="isDark" />
            <MoonOutline v-else />
          </n-icon>
        </button>
      </header>

      <!-- Content -->
      <main class="flex-1 overflow-y-auto p-4 md:p-6">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>
  </div>

  <!-- AI chat modal -->
  <n-modal
    v-model:show="showChat"
    preset="card"
    :style="{ width: isMobile ? '90vw' : '400px' }"
    title="AI 助手"
    :bordered="false"
    :auto-focus="false"
    :mask-closable="true"
  >
    <div class="h-[400px] overflow-y-auto flex flex-col gap-3 p-2">
      <div
        v-for="(msg, i) in chatMessages" :key="i"
        class="max-w-[80%]"
        :class="msg.type === 'user' ? 'self-end' : 'self-start'"
      >
        <div class="text-xs text-muted-foreground mb-1">{{ msg.type === 'user' ? '你' : 'AI' }}</div>
        <div
          class="px-3 py-2 rounded-lg text-sm leading-relaxed"
          :class="msg.type === 'user'
            ? 'bg-primary text-primary-foreground rounded-br-sm'
            : 'bg-muted text-foreground rounded-bl-sm'"
        >{{ msg.content }}</div>
      </div>
    </div>
    <template #footer>
      <div class="flex gap-2">
        <input
          v-model="userInput"
          class="flex-1 h-9 rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder="输入消息..."
          @keydown.enter="sendMessage"
        />
        <button
          class="h-9 px-4 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
          @click="sendMessage"
        >发送</button>
      </div>
    </template>
  </n-modal>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { useResponsive } from '../../composables/useResponsive'
import {
  MenuOutline, SearchOutline, SunnyOutline, MoonOutline,
  HomeOutline, FilmOutline, DownloadOutline, SettingsOutline,
  CalendarOutline, LogoRss, NotificationsOutline, ChatbubbleEllipsesOutline,
  LibraryOutline, CodeSlashOutline
} from '@vicons/ionicons5'
import { NIcon, NDropdown, NModal } from 'naive-ui'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { isMobile } = useResponsive()

const collapsed = ref(isMobile.value)
const searchQuery = ref('')
const showChat = ref(false)
const userInput = ref('')
const chatMessages = ref([
  { type: 'assistant', content: '您好！我是您的AI助手，有什么可以帮您的吗？' }
])

const isDark = ref(localStorage.getItem('theme') === 'dark')

const menuItems = [
  { label: '首页', key: 'dashboard', icon: HomeOutline, route: '/' },
  { label: '我的追番', key: 'anime-list', icon: FilmOutline, route: '/anime' },
  { label: '下载管理', key: 'downloads', icon: DownloadOutline, route: '/downloads' },
  { label: 'RSS订阅', key: 'rss', icon: LogoRss, route: '/rss' },
  { label: '放送日历', key: 'calendar', icon: CalendarOutline, route: '/calendar' },
  { label: '番剧库', key: 'anime-library', icon: LibraryOutline, route: '/anime-library' },
  { label: '规则管理', key: 'stream-rules', icon: CodeSlashOutline, route: '/stream-rules' },
  { label: '番剧搜索', key: 'search', icon: SearchOutline, route: '/search' },
  { label: '通知设置', key: 'notifications', icon: NotificationsOutline, route: '/notifications' },
  { label: '设置', key: 'settings', icon: SettingsOutline, route: '/settings' }
]

const userOptions = [
  { label: '设置', key: 'settings' },
  { type: 'divider', key: 'd1' },
  { label: '退出登录', key: 'logout' }
]

const activeKey = computed(() => {
  const path = route.path
  if (path.startsWith('/anime-library')) return 'anime-library'
  if (path.startsWith('/anime')) return 'anime-list'
  if (path.startsWith('/downloads')) return 'downloads'
  if (path.startsWith('/rss')) return 'rss'
  if (path.startsWith('/calendar')) return 'calendar'
  if (path.startsWith('/stream-rules')) return 'stream-rules'
  if (path.startsWith('/search')) return 'search'
  if (path.startsWith('/notifications')) return 'notifications'
  if (path.startsWith('/settings')) return 'settings'
  return 'dashboard'
})

const pageTitle = computed(() => {
  const item = menuItems.find(m => m.key === activeKey.value)
  return item?.label || '御宅追番'
})

function navigateTo(route_path) {
  router.push(route_path)
  if (isMobile.value) collapsed.value = true
}

function handleSearch() {
  if (searchQuery.value.trim()) {
    router.push({ path: '/search', query: { q: searchQuery.value.trim() } })
    searchQuery.value = ''
  }
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function handleUserSelect(key) {
  if (key === 'logout') {
    authStore.logout()
    router.push('/auth/login')
  } else if (key === 'settings') {
    router.push('/settings')
  }
}

function sendMessage() {
  if (!userInput.value.trim()) return
  chatMessages.value.push({ type: 'user', content: userInput.value })
  setTimeout(() => {
    chatMessages.value.push({ type: 'assistant', content: '我正在为您查找相关信息，请稍候...' })
  }, 1000)
  userInput.value = ''
}

onMounted(() => {
  document.documentElement.classList.toggle('dark', isDark.value)
  if (isMobile.value) collapsed.value = true
})
</script>

<style>
.fade-enter-active { transition: opacity 0.2s ease, transform 0.2s ease; }
.fade-leave-active { transition: opacity 0.15s ease, transform 0.15s ease; }
.fade-enter-from { opacity: 0; transform: translateY(6px); }
.fade-leave-to { opacity: 0; transform: translateY(-6px); }
</style>
