<template>
  <div class="flex h-screen bg-background ac-grass-pattern">
    <!-- Mobile overlay -->
    <div
      v-if="isMobile && !collapsed"
      class="fixed inset-0 bg-ac-night/40 backdrop-blur-sm z-40 transition-opacity"
      @click="collapsed = true"
    />

    <!-- Sidebar -->
    <aside
      class="flex flex-col bg-sidebar shrink-0 z-50 transition-all duration-300 border-r-2 border-ac-sand"
      :class="[
        collapsed ? 'w-16' : 'w-64',
        isMobile ? 'fixed inset-y-0 left-0' : ''
      ]"
      :style="isMobile && collapsed ? { transform: 'translateX(-100%)' } : {}"
    >
      <!-- Logo -->
      <div class="h-16 flex items-center px-4 border-b-2 border-dashed border-ac-sand shrink-0">
        <div class="size-10 rounded-2xl bg-ac-grass-light/50 border-2 border-ac-grass flex items-center justify-center shrink-0 shadow-sm">
          <img src="@/assets/logo.svg" alt="AniDog" class="size-7" />
        </div>
        <span v-if="!collapsed" class="ml-3 text-lg font-bold text-sidebar-foreground whitespace-nowrap tracking-tight">AniDog</span>
      </div>

      <!-- Navigation -->
      <nav class="flex-1 py-3 overflow-y-auto px-2 space-y-1">
        <a
          v-for="item in menuItems"
          :key="item.key"
          class="flex items-center gap-3 h-11 px-3 rounded-2xl text-sm cursor-pointer transition-all duration-150 group"
          :class="activeKey === item.key
            ? 'bg-ac-grass text-white font-bold shadow-sm border-b-[3px] border-ac-grass-dark'
            : 'text-sidebar-foreground/80 hover:text-ac-grass-dark hover:bg-ac-grass-light/30'"
          @click="navigateTo(item.route)"
        >
          <component :is="item.icon" class="size-5 shrink-0 transition-transform group-hover:scale-110" />
          <span v-if="!collapsed" class="truncate">{{ item.label }}</span>
        </a>
      </nav>

      <!-- User section -->
      <div class="border-t-2 border-dashed border-ac-sand p-3 shrink-0">
        <AcDropdown :options="userOptions" placement="top-start" @select="handleUserSelect">
          <template #trigger>
            <div class="flex items-center cursor-pointer rounded-2xl p-1.5 hover:bg-ac-sand/60 transition-colors">
              <div class="size-9 rounded-full bg-ac-sun/40 border-2 border-ac-sun flex items-center justify-center text-ac-night text-sm font-bold shrink-0">
                {{ authStore.user?.username?.charAt(0)?.toUpperCase() || 'U' }}
              </div>
              <div v-if="!collapsed" class="ml-2.5 flex-1 min-w-0">
                <p class="text-sm font-bold text-sidebar-foreground truncate leading-tight">{{ authStore.user?.username || '用户' }}</p>
                <span class="text-xs text-sidebar-muted-foreground">{{ authStore.user?.is_admin ? '🌟 管理员' : '🌿 居民' }}</span>
              </div>
            </div>
          </template>
        </AcDropdown>
      </div>
    </aside>

    <!-- Main area -->
    <div class="flex-1 flex flex-col min-w-0">
      <!-- Top bar -->
      <header class="h-16 border-b-2 border-ac-sand bg-card/80 backdrop-blur-sm flex items-center gap-3 px-4 md:px-6 shrink-0">
        <button
          class="size-9 rounded-2xl hover:bg-ac-sand/60 text-muted-foreground transition-colors flex items-center justify-center"
          @click="collapsed = !collapsed"
        >
          <MenuOutline class="size-5" />
        </button>

        <h2 class="text-base font-bold text-foreground tracking-tight hidden sm:block">{{ pageTitle }}</h2>

        <div class="flex-1" />

        <!-- Search -->
        <div class="hidden md:block w-72">
          <AcInput
            v-model="searchQuery"
            placeholder="搜索番剧..."
            size="md"
            @keyup-enter="handleSearch"
          >
            <template #prefix><SearchOutline class="size-4" /></template>
          </AcInput>
        </div>

        <button
          class="size-9 rounded-2xl hover:bg-ac-sand/60 text-muted-foreground transition-colors flex items-center justify-center"
          @click="toggleTheme"
          :title="isDark ? '切换到白天' : '切换到夜晚'"
        >
          <SunnyOutline v-if="isDark" class="size-5 text-ac-sun" />
          <MoonOutline v-else class="size-5" />
        </button>
      </header>

      <!-- Content -->
      <main class="flex-1 overflow-y-auto p-4 md:p-6">
        <router-view v-slot="{ Component }">
          <transition name="ac-fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { useResponsive } from '../../composables/useResponsive'
import {
  MenuOutline, SearchOutline, SunnyOutline, MoonOutline,
  HomeOutline, FilmOutline, DownloadOutline, SettingsOutline,
  CalendarOutline, LogoRss, NotificationsOutline,
  LibraryOutline, CodeSlashOutline,
} from '@vicons/ionicons5'
import { AcInput, AcDropdown } from '../../components/ac'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { isMobile } = useResponsive()

const collapsed = ref(isMobile.value)
const searchQuery = ref('')

const isDark = ref(localStorage.getItem('theme') === 'dark')

const menuItems = [
  { label: '首页',     key: 'dashboard',      icon: HomeOutline,          route: '/' },
  { label: '我的追番', key: 'anime-list',     icon: FilmOutline,          route: '/anime' },
  { label: '下载管理', key: 'downloads',      icon: DownloadOutline,      route: '/downloads' },
  { label: 'RSS订阅',  key: 'rss',            icon: LogoRss,              route: '/rss' },
  { label: '放送日历', key: 'calendar',       icon: CalendarOutline,      route: '/calendar' },
  { label: '番剧库',   key: 'anime-library',  icon: LibraryOutline,       route: '/anime-library' },
  { label: '规则管理', key: 'stream-rules',   icon: CodeSlashOutline,     route: '/stream-rules' },
  { label: '资源搜索', key: 'search',         icon: SearchOutline,        route: '/search' },
  { label: '通知设置', key: 'notifications',  icon: NotificationsOutline, route: '/notifications' },
  { label: '设置',     key: 'settings',       icon: SettingsOutline,      route: '/settings' },
]

const userOptions = [
  { label: '设置', key: 'settings' },
  { type: 'divider', key: 'd1' },
  { label: '退出登录', key: 'logout', danger: true },
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
  return item?.label || 'AniDog'
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

onMounted(() => {
  document.documentElement.classList.toggle('dark', isDark.value)
  if (isMobile.value) collapsed.value = true
})
</script>
