<template>
  <n-config-provider :theme="theme" :theme-overrides="themeOverrides">
    <div class="app-container" :class="{ 'dark-mode': isDark, 'sidebar-collapsed': collapsed }">
      <!-- 侧边栏 -->
      <div class="sidebar" :class="{ 'collapsed': collapsed }">
        <!-- Logo区域 -->
        <div class="sidebar-header">
          <div v-if="collapsed" class="logo-collapsed">
            <div class="logo-icon">
              <n-icon size="28" color="#6366F1">
                <PlayOutline />
              </n-icon>
            </div>
          </div>
          <div v-else class="logo-expanded">
            <div class="logo-icon">
              <n-icon size="32" color="#6366F1">
                <PlayOutline />
              </n-icon>
            </div>
            <div class="logo-text">
              <h1 class="app-title">御宅追番</h1>
              <p class="app-subtitle">Anime Tracker</p>
            </div>
          </div>
        </div>

        <!-- 导航菜单 -->
        <div class="sidebar-menu">
          <div class="menu-section">
            <div v-if="!collapsed" class="menu-title">主要功能</div>
            <div class="menu-items">
              <router-link
                v-for="item in mainMenuItems"
                :key="item.key"
                :to="item.path"
                class="menu-item"
                :class="{ 'active': isActive(item.path) }"
              >
                <div class="menu-icon">
                  <n-icon :size="18">
                    <component :is="item.icon" />
                  </n-icon>
                </div>
                <div v-if="!collapsed" class="menu-content">
                  <span class="menu-label">{{ item.label }}</span>
                  <n-badge v-if="item.badge" :value="item.badge" class="menu-badge" />
                </div>
                <div v-if="!collapsed && item.description" class="menu-description">
                  {{ item.description }}
                </div>
              </router-link>
            </div>
          </div>

          <div class="menu-section">
            <div v-if="!collapsed" class="menu-title">工具</div>
            <div class="menu-items">
              <router-link
                v-for="item in toolMenuItems"
                :key="item.key"
                :to="item.path"
                class="menu-item"
                :class="{ 'active': isActive(item.path) }"
              >
                <div class="menu-icon">
                  <n-icon :size="18">
                    <component :is="item.icon" />
                  </n-icon>
                </div>
                <div v-if="!collapsed" class="menu-content">
                  <span class="menu-label">{{ item.label }}</span>
                </div>
              </router-link>
            </div>
          </div>
        </div>

        <!-- 用户信息 -->
        <div class="sidebar-footer">
          <n-dropdown :options="userOptions" @select="handleUserSelect" trigger="click">
            <div class="user-info" :class="{ 'collapsed': collapsed }">
              <n-avatar
                round
                :size="collapsed ? 40 : 36"
                :style="{ 
                  background: `linear-gradient(135deg, ${stringToColor(authStore.user?.username || '用户')}, ${adjustColor(stringToColor(authStore.user?.username || '用户'), -20)})`,
                  border: '2px solid var(--glass-border)'
                }"
                class="user-avatar"
              >
                {{ (authStore.user?.username || '用户')[0].toUpperCase() }}
              </n-avatar>
              <div v-if="!collapsed" class="user-details">
                <div class="user-name">{{ authStore.user?.username || '用户' }}</div>
                <div class="user-status">
                  <div class="status-dot"></div>
                  <span>在线</span>
                </div>
              </div>
              <div v-if="!collapsed" class="user-actions">
                <n-icon size="16" class="chevron-icon">
                  <ChevronDown />
                </n-icon>
              </div>
            </div>
          </n-dropdown>
        </div>

        <!-- 折叠按钮 -->
        <div class="collapse-button" @click="collapsed = !collapsed">
          <n-icon size="16">
            <ChevronForward v-if="collapsed" />
            <ChevronBack v-else />
          </n-icon>
        </div>
      </div>

      <!-- 主内容区 -->
      <div class="main-content">
        <!-- 顶部栏 -->
        <div class="top-bar">
          <div class="top-bar-left">
            <div class="breadcrumb">
              <span class="breadcrumb-current">{{ pageTitle }}</span>
            </div>
          </div>

          <div class="top-bar-center">
            <div class="search-container">
              <n-input
                v-model:value="searchQuery"
                placeholder="搜索动漫、角色、声优..."
                round
                size="large"
                class="search-input"
              >
                <template #prefix>
                  <n-icon size="18" class="search-icon">
                    <SearchOutline />
                  </n-icon>
                </template>
                <template #suffix>
                                      <n-button text class="search-button">
                      <n-icon size="16">
                        <CheckmarkOutline />
                      </n-icon>
                    </n-button>
                </template>
              </n-input>
            </div>
          </div>

          <div class="top-bar-right">
            <div class="action-buttons">
              <!-- 通知 -->
              <n-popover trigger="click" placement="bottom-end">
                <template #trigger>
                  <div class="action-button notification-btn">
                    <n-badge dot :show="true" processing>
                      <n-icon size="20">
                        <NotificationsOutline />
                      </n-icon>
                    </n-badge>
                  </div>
                </template>
                <div class="notifications-panel">
                  <div class="notifications-header">
                    <h3>通知</h3>
                    <n-button text size="small">全部已读</n-button>
                  </div>
                  <div class="notifications-list">
                    <div class="notification-item">
                      <div class="notification-icon">🎬</div>
                      <div class="notification-content">
                        <div class="notification-title">新番更新</div>
                        <div class="notification-text">《葬送的芙莉莲》第12集已更新</div>
                        <div class="notification-time">2分钟前</div>
                      </div>
                    </div>
                    <div class="notification-item">
                      <div class="notification-icon">📥</div>
                      <div class="notification-content">
                        <div class="notification-title">下载完成</div>
                        <div class="notification-text">《鬼灭之刃》第11集下载完成</div>
                        <div class="notification-time">10分钟前</div>
                      </div>
                    </div>
                  </div>
                </div>
              </n-popover>

              <!-- 主题切换 -->
              <div class="action-button theme-btn" @click="toggleTheme">
                <n-icon size="20">
                  <SunnyOutline v-if="isDark" />
                  <MoonOutline v-else />
                </n-icon>
              </div>

              <!-- 用户菜单 -->
              <n-dropdown :options="headerUserOptions" @select="handleUserSelect" trigger="click">
                <div class="action-button user-btn">
                  <n-avatar
                    round
                    size="medium"
                    :style="{ 
                      background: `linear-gradient(135deg, ${stringToColor(authStore.user?.username || '用户')}, ${adjustColor(stringToColor(authStore.user?.username || '用户'), -20)})`
                    }"
                  >
                    {{ (authStore.user?.username || '用户')[0].toUpperCase() }}
                  </n-avatar>
                </div>
              </n-dropdown>
            </div>
          </div>
        </div>

        <!-- 页面内容 -->
        <div class="page-content">
          <div class="content-container">
            <!-- 页面头部 -->
            <div class="page-header">
              <div class="page-title-section">
                <h1 class="page-title">{{ pageTitle }}</h1>
                <p class="page-description">{{ pageDescription }}</p>
              </div>
            </div>

            <!-- 内容区域 -->
            <div class="content-area">
              <router-view />
            </div>
          </div>
        </div>
      </div>

      <!-- AI 助手浮动按钮 -->
      <div class="ai-assistant-fab" @click="showChat = !showChat">
        <div class="fab-icon">
          <n-icon size="24">
            <ChatbubbleEllipsesOutline v-if="!showChat" />
            <CloseOutline v-else />
          </n-icon>
        </div>
        <div class="fab-pulse"></div>
      </div>

      <!-- AI 助手对话框 -->
      <n-modal
        v-model:show="showChat"
        preset="card"
        style="width: 460px; max-height: 80vh"
        title="AI 智能助手"
        size="huge"
        :mask-closable="false"
        class="ai-chat-modal"
      >
        <template #header-extra>
          <div class="chat-status">
            <div class="status-indicator"></div>
            <span>智能在线</span>
          </div>
        </template>

        <div class="chat-container">
          <div class="chat-messages" ref="chatContainer">
            <div v-for="(message, index) in chatMessages" :key="index" 
                 :class="['message', `message-${message.type}`]">
              <div v-if="message.type === 'system'" class="message-avatar">
                <div class="ai-avatar">
                  <n-icon size="20">
                    <HardwareChipOutline />
                  </n-icon>
                </div>
              </div>
              <div v-else class="message-avatar">
                <n-avatar size="small" :style="{ backgroundColor: '#6366F1' }">
                  {{ (authStore.user?.username || '用户')[0].toUpperCase() }}
                </n-avatar>
              </div>
              <div class="message-content">
                <div class="message-bubble">
                  {{ message.content }}
                </div>
                <div class="message-time">
                  {{ formatTime(message.time) }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <template #footer>
          <div class="chat-input-container">
            <n-input
              v-model:value="userInput"
              type="text"
              placeholder="输入你的问题..."
              @keydown.enter="sendMessage"
              size="large"
              round
              class="chat-input"
            >
              <template #suffix>
                <n-button
                  type="primary"
                  @click="sendMessage"
                  :disabled="!userInput.trim()"
                  circle
                  class="send-button"
                >
                  <template #icon>
                    <n-icon size="16">
                      <SendOutline />
                    </n-icon>
                  </template>
                </n-button>
              </template>
            </n-input>
          </div>
        </template>
      </n-modal>
    </div>
  </n-config-provider>
</template>

<script setup>
import { ref, onMounted, computed, nextTick, h } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  NConfigProvider,
  NIcon,
  NAvatar,
  NBadge,
  NDropdown,
  NInput,
  NButton,
  NPopover,
  NModal,
  darkTheme,
  useMessage
} from 'naive-ui'
import {
  PlayOutline,
  HomeOutline,
  VideocamOutline,
  DownloadOutline,
  ChevronDown,
  ChevronForward,
  ChevronBack,
  SearchOutline,
  CheckmarkOutline,
  NotificationsOutline,
  SunnyOutline,
  MoonOutline,
  ChatbubbleEllipsesOutline,
  CloseOutline,
  HardwareChipOutline,
  SendOutline,
  PersonOutline,
  LogOutOutline,
  SettingsOutline as SettingsIcon,
  LogoRss
} from '@vicons/ionicons5'

// 响应式状态
const collapsed = ref(false)
const isDark = ref(false)
const userInput = ref('')
const chatMessages = ref([
  { type: 'system', content: '你好！我是你的AI追番助手，可以帮你推荐动漫、查询信息或解答问题。', time: new Date() }
])
const chatContainer = ref(null)
const showChat = ref(false)
const searchQuery = ref('')
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const message = useMessage()

// 主题配置
const theme = computed(() => isDark.value ? darkTheme : null)
const themeOverrides = {
  common: {
    primaryColor: '#6366F1',
    primaryColorHover: '#4F46E5',
    primaryColorPressed: '#4338CA',
  }
}

// 菜单配置
const mainMenuItems = [
  {
    key: 'dashboard',
    label: '控制面板',
    description: '总览与统计',
    path: '/',
    icon: HomeOutline
  },
  {
    key: 'anime',
    label: '动漫库',
    description: '浏览和管理',
    path: '/anime',
    badge: 12,
    icon: VideocamOutline
  },
  {
    key: 'downloads',
    label: '下载中心',
    description: '任务管理',
    path: '/downloads',
    badge: 3,
    icon: DownloadOutline
  }
]

const toolMenuItems = [
  {
    key: 'rss',
    label: 'RSS订阅',
    path: '/rss',
    icon: LogoRss
  },
  {
    key: 'settings',
    label: '设置',
    path: '/settings',
    icon: SettingsIcon
  }
]

// 用户菜单选项
const userOptions = [
  {
    label: '个人资料',
    key: 'profile',
    icon: () => h(NIcon, null, { default: () => h(PersonOutline) })
  },
  {
    type: 'divider',
    key: 'd1'
  },
  {
    label: '退出登录',
    key: 'logout',
    icon: () => h(NIcon, null, { default: () => h(LogOutOutline) })
  }
]

const headerUserOptions = [...userOptions]

// 计算属性
const pageTitle = computed(() => {
  const path = route.path
  if (path.startsWith('/anime')) return '动漫库'
  if (path.startsWith('/downloads')) return '下载中心'
  if (path.startsWith('/rss')) return 'RSS订阅'
  if (path.startsWith('/settings')) return '设置'
  return '控制面板'
})

const pageDescription = computed(() => {
  const path = route.path
  if (path.startsWith('/anime')) return '发现、收藏和管理你喜爱的动漫作品'
  if (path.startsWith('/downloads')) return '管理下载任务和本地媒体文件'
  if (path.startsWith('/rss')) return '订阅和管理动漫更新源'
  if (path.startsWith('/settings')) return '个性化设置和偏好配置'
  return '追番数据总览和快速操作入口'
})

// 工具函数
const stringToColor = (str) => {
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash)
  }
  const colors = ['#FF6B6B', '#4ECDC4', '#45B7D1', '#96CEB4', '#FECA57', '#FF9FF3', '#54A0FF', '#5F27CD']
  return colors[Math.abs(hash) % colors.length]
}

const adjustColor = (color, amount) => {
  const usePound = color[0] === '#'
  const col = usePound ? color.slice(1) : color
  const num = parseInt(col, 16)
  let r = (num >> 16) + amount
  let g = (num >> 8 & 0x00FF) + amount
  let b = (num & 0x0000FF) + amount
  r = r > 255 ? 255 : r < 0 ? 0 : r
  g = g > 255 ? 255 : g < 0 ? 0 : g
  b = b > 255 ? 255 : b < 0 ? 0 : b
  return (usePound ? '#' : '') + (r << 16 | g << 8 | b).toString(16).padStart(6, '0')
}

const formatTime = (time) => {
  if (!time) return ''
  return new Date(time).toLocaleTimeString('zh-CN', { 
    hour: '2-digit', 
    minute: '2-digit' 
  })
}

const isActive = (path) => {
  if (path === '/') {
    return route.path === '/'
  }
  return route.path.startsWith(path)
}

// 事件处理
const toggleTheme = () => {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark')
}

const handleUserSelect = (key) => {
  if (key === 'logout') {
    authStore.logout()
    router.push('/login')
    message.success('已退出登录')
  } else if (key === 'profile') {
    router.push('/profile')
  }
}

const sendMessage = async () => {
  if (!userInput.value.trim()) return

  chatMessages.value.push({
    type: 'user',
    content: userInput.value,
    time: new Date()
  })

  const messageText = userInput.value
  userInput.value = ''

  await nextTick()
  if (chatContainer.value) {
    chatContainer.value.scrollTop = chatContainer.value.scrollHeight
  }

  // 模拟AI回复
  setTimeout(() => {
    const responses = [
      '根据你的喜好，我推荐你看看《葬送的芙莉莲》，这是一部非常优秀的奇幻冒险动画。',
      '这部动漫的评分很高呢！已经为你添加到关注列表了。',
      '看起来你对这类动漫很感兴趣，我可以为你推荐更多类似的作品。',
      '好的，我已经帮你设置了更新提醒，有新集数时会第一时间通知你。'
    ]
    
    chatMessages.value.push({
      type: 'system',
      content: responses[Math.floor(Math.random() * responses.length)],
      time: new Date()
    })

    nextTick(() => {
      if (chatContainer.value) {
        chatContainer.value.scrollTop = chatContainer.value.scrollHeight
      }
    })
  }, 1000)
}

// 生命周期
onMounted(() => {
  if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
})
</script>

<style scoped>
.app-container {
  display: flex;
  height: 100vh;
  background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
  transition: all 0.3s ease;
}

.dark-mode {
  background: linear-gradient(135deg, #0c0c0c 0%, #1a1a2e 100%);
}

/* 侧边栏样式 */
.sidebar {
  width: 280px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(20px);
  border-right: 1px solid rgba(255, 255, 255, 0.2);
  display: flex;
  flex-direction: column;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.1);
}

.dark-mode .sidebar {
  background: rgba(26, 32, 44, 0.95);
  border-right: 1px solid rgba(255, 255, 255, 0.1);
}

.sidebar.collapsed {
  width: 80px;
}

.sidebar-header {
  padding: 24px 20px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
}

.dark-mode .sidebar-header {
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.logo-collapsed {
  display: flex;
  justify-content: center;
}

.logo-expanded {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 12px;
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
}

.app-title {
  font-size: 20px;
  font-weight: 700;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  margin: 0;
}

.app-subtitle {
  font-size: 12px;
  color: rgba(0, 0, 0, 0.5);
  margin: 0;
  font-weight: 500;
}

.dark-mode .app-subtitle {
  color: rgba(255, 255, 255, 0.5);
}

/* 菜单样式 */
.sidebar-menu {
  flex: 1;
  padding: 20px;
  overflow-y: auto;
}

.menu-section {
  margin-bottom: 32px;
}

.menu-title {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 1px;
  color: rgba(0, 0, 0, 0.4);
  margin-bottom: 12px;
  padding: 0 12px;
}

.dark-mode .menu-title {
  color: rgba(255, 255, 255, 0.4);
}

.menu-items {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.menu-item {
  display: flex;
  align-items: center;
  padding: 12px;
  border-radius: 12px;
  text-decoration: none;
  color: rgba(0, 0, 0, 0.7);
  transition: all 0.2s ease;
  position: relative;
  overflow: hidden;
}

.dark-mode .menu-item {
  color: rgba(255, 255, 255, 0.7);
}

.menu-item:hover {
  background: rgba(102, 126, 234, 0.1);
  color: #667eea;
  transform: translateX(4px);
}

.menu-item.active {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: white;
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
}

.menu-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  margin-right: 12px;
}

.collapsed .menu-item {
  justify-content: center;
  padding: 16px 12px;
}

.collapsed .menu-icon {
  margin-right: 0;
}

.menu-content {
  display: flex;
  align-items: center;
  flex: 1;
  gap: 8px;
}

.menu-label {
  font-weight: 500;
  font-size: 14px;
}

.menu-description {
  font-size: 11px;
  opacity: 0.6;
  margin-top: 2px;
}

.menu-badge {
  margin-left: auto;
}

/* 用户信息样式 */
.sidebar-footer {
  padding: 20px;
  border-top: 1px solid rgba(0, 0, 0, 0.05);
}

.dark-mode .sidebar-footer {
  border-top: 1px solid rgba(255, 255, 255, 0.05);
}

.user-info {
  display: flex;
  align-items: center;
  padding: 12px;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
  background: rgba(0, 0, 0, 0.02);
}

.dark-mode .user-info {
  background: rgba(255, 255, 255, 0.02);
}

.user-info:hover {
  background: rgba(102, 126, 234, 0.1);
}

.user-info.collapsed {
  justify-content: center;
}

.user-avatar {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.user-details {
  margin-left: 12px;
  flex: 1;
}

.user-name {
  font-weight: 600;
  font-size: 14px;
  color: rgba(0, 0, 0, 0.9);
}

.dark-mode .user-name {
  color: rgba(255, 255, 255, 0.9);
}

.user-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: rgba(0, 0, 0, 0.5);
  margin-top: 2px;
}

.dark-mode .user-status {
  color: rgba(255, 255, 255, 0.5);
}

.status-dot {
  width: 8px;
  height: 8px;
  background: #10b981;
  border-radius: 50%;
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.user-actions {
  margin-left: 8px;
}

.chevron-icon {
  opacity: 0.5;
}

/* 折叠按钮 */
.collapse-button {
  position: absolute;
  top: 50%;
  right: -12px;
  width: 24px;
  height: 24px;
  background: white;
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.dark-mode .collapse-button {
  background: #1a202c;
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.collapse-button:hover {
  transform: scale(1.1);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

/* 主内容区 */
.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  width: calc(100% - 280px);
}

.app-container.sidebar-collapsed .main-content {
  width: calc(100% - 80px);
}

/* 顶部栏 */
.top-bar {
  height: 80px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(20px);
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 32px;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.05);
}

.dark-mode .top-bar {
  background: rgba(26, 32, 44, 0.95);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.top-bar-left {
  flex: 1;
  padding-left: 0;
}

.breadcrumb-current {
  font-size: 24px;
  font-weight: 700;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.top-bar-center {
  flex: 2;
  display: flex;
  justify-content: center;
}

.search-container {
  width: 100%;
  max-width: 500px;
}

.search-input {
  width: 100%;
}

.search-icon {
  color: rgba(0, 0, 0, 0.4);
}

.dark-mode .search-icon {
  color: rgba(255, 255, 255, 0.4);
}

.search-button {
  padding: 0;
}

.top-bar-right {
  flex: 1;
  display: flex;
  justify-content: flex-end;
}

.action-buttons {
  display: flex;
  align-items: center;
  gap: 16px;
}

.action-button {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.03);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid rgba(0, 0, 0, 0.05);
}

.dark-mode .action-button {
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.action-button:hover {
  background: rgba(102, 126, 234, 0.1);
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(102, 126, 234, 0.2);
}

/* 页面内容 */
.page-content {
  flex: 1;
  overflow-y: auto;
  background: transparent;
}

.content-container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 32px;
}

.page-header {
  margin-bottom: 32px;
}

.page-title {
  font-size: 32px;
  font-weight: 800;
  color: rgba(0, 0, 0, 0.9);
  margin: 0 0 8px 0;
}

.dark-mode .page-title {
  color: rgba(255, 255, 255, 0.9);
}

.page-description {
  font-size: 16px;
  color: rgba(0, 0, 0, 0.6);
  margin: 0;
}

.dark-mode .page-description {
  color: rgba(255, 255, 255, 0.6);
}



/* 内容区域 */
.content-area {
  background: rgba(255, 255, 255, 0.7);
  backdrop-filter: blur(20px);
  border-radius: 20px;
  padding: 32px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.dark-mode .content-area {
  background: rgba(26, 32, 44, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.anime-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 24px;
}

/* AI助手 */
.ai-assistant-fab {
  position: fixed;
  bottom: 32px;
  right: 32px;
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.3s ease;
  box-shadow: 0 8px 25px rgba(102, 126, 234, 0.4);
  z-index: 1000;
}

.ai-assistant-fab:hover {
  transform: scale(1.1);
  box-shadow: 0 12px 35px rgba(102, 126, 234, 0.6);
}

.fab-icon {
  color: white;
  z-index: 2;
}

.fab-pulse {
  position: absolute;
  width: 100%;
  height: 100%;
  border-radius: 50%;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  animation: pulse-ring 2s infinite;
}

@keyframes pulse-ring {
  0% { transform: scale(1); opacity: 1; }
  100% { transform: scale(1.5); opacity: 0; }
}

/* 聊天界面 */
.chat-container {
  height: 400px;
  display: flex;
  flex-direction: column;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 16px 0;
  space-y: 16px;
}

.message {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.message-avatar {
  flex-shrink: 0;
}

.ai-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
}

.message-content {
  flex: 1;
}

.message-bubble {
  background: rgba(0, 0, 0, 0.05);
  padding: 12px 16px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.5;
}

.message-user .message-bubble {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
}

.dark-mode .message-bubble {
  background: rgba(255, 255, 255, 0.05);
}

.message-time {
  font-size: 11px;
  color: rgba(0, 0, 0, 0.4);
  margin-top: 4px;
}

.dark-mode .message-time {
  color: rgba(255, 255, 255, 0.4);
}

.chat-status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #10b981;
}

.status-indicator {
  width: 8px;
  height: 8px;
  background: #10b981;
  border-radius: 50%;
  animation: pulse 2s infinite;
}

.chat-input-container {
  display: flex;
  gap: 12px;
  align-items: center;
}

.chat-input {
  flex: 1;
}

.send-button {
  width: 40px;
  height: 40px;
}

/* 通知面板 */
.notifications-panel {
  width: 300px;
  max-height: 400px;
}

.notifications-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.1);
}

.dark-mode .notifications-header {
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.notifications-list {
  max-height: 300px;
  overflow-y: auto;
}

.notification-item {
  display: flex;
  gap: 12px;
  padding: 16px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  transition: background 0.2s ease;
}

.dark-mode .notification-item {
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.notification-item:hover {
  background: rgba(0, 0, 0, 0.02);
}

.dark-mode .notification-item:hover {
  background: rgba(255, 255, 255, 0.02);
}

.notification-icon {
  font-size: 20px;
}

.notification-content {
  flex: 1;
}

.notification-title {
  font-weight: 600;
  font-size: 14px;
  margin-bottom: 4px;
}

.notification-text {
  font-size: 13px;
  color: rgba(0, 0, 0, 0.6);
  margin-bottom: 4px;
}

.dark-mode .notification-text {
  color: rgba(255, 255, 255, 0.6);
}

.notification-time {
  font-size: 11px;
  color: rgba(0, 0, 0, 0.4);
}

.dark-mode .notification-time {
  color: rgba(255, 255, 255, 0.4);
}

/* 滚动条样式 */
::-webkit-scrollbar {
  width: 6px;
}

::-webkit-scrollbar-track {
  background: transparent;
}

::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.2);
  border-radius: 3px;
}

.dark-mode ::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.2);
}

::-webkit-scrollbar-thumb:hover {
  background: rgba(0, 0, 0, 0.3);
}

.dark-mode ::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.3);
}

/* 响应式设计 */
@media (max-width: 768px) {
  .top-bar,
  .page-content {
    margin-left: 0 !important;
  }
  
  .sidebar {
    position: fixed;
    left: -280px;
    z-index: 100;
  }
  
  .sidebar.collapsed {
    left: -80px;
  }
  
  .content-container {
    padding: 16px;
  }
  

  
  .top-bar-center {
    flex: 1;
  }
  
  .top-bar-left,
  .top-bar-right {
    flex: 0;
  }
}
</style> 