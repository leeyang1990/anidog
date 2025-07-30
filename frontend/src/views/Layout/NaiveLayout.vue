<template>
  <n-config-provider :theme="theme">
    <n-layout has-sider style="height: 100vh">
      <!-- 侧边栏 -->
      <n-layout-sider
        bordered
        collapse-mode="width"
        :collapsed-width="64"
        :width="240"
        :collapsed="collapsed"
        show-trigger
        @collapse="collapsed = true"
        @expand="collapsed = false"
        :native-scrollbar="false"
      >
        <div class="flex flex-col h-full">
          <!-- Logo区域 -->
          <div class="flex items-center h-14 px-3">
            <div v-if="collapsed" class="w-full flex justify-center">
              <n-avatar round size="large" src="https://07akioni.oss-cn-beijing.aliyuncs.com/07akioni.jpeg" />
            </div>
            <div v-else class="flex items-center">
              <n-avatar round size="small" src="https://07akioni.oss-cn-beijing.aliyuncs.com/07akioni.jpeg" />
              <div class="ml-3">
                <h1 class="text-lg font-bold">
                  <span class="text-primary">御宅</span>
                  <span class="text-blue-500">追番</span>
                </h1>
              </div>
            </div>
          </div>

          <!-- 导航菜单 -->
          <n-menu
            :collapsed="collapsed"
            :collapsed-width="64"
            :collapsed-icon-size="18"
            :options="menuOptions"
            :render-label="renderMenuLabel"
            :render-icon="renderMenuIcon"
            :value="activeKey"
            @update:value="handleUpdateValue"
          />

          <!-- 底部用户信息 -->
          <div class="mt-auto p-3 border-t">
            <n-dropdown :options="userOptions" @select="handleUserSelect">
              <div class="flex items-center cursor-pointer">
                <n-avatar round :size="collapsed ? 'large' : 'small'" src="https://07akioni.oss-cn-beijing.aliyuncs.com/07akioni.jpeg" />
                <div v-if="!collapsed" class="ml-3 flex-1">
                  <p class="text-sm font-medium">{{ authStore.user?.username || '用户' }}</p>
                  <n-tag size="small" type="success">在线</n-tag>
                </div>
              </div>
            </n-dropdown>
          </div>
        </div>
      </n-layout-sider>

      <n-layout>
        <!-- 顶部栏 -->
        <n-layout-header bordered style="height: 56px; padding: 0 20px" class="flex items-center justify-between">
          <div class="flex items-center">
            <n-button quaternary circle @click="collapsed = !collapsed">
              <template #icon>
                <n-icon>
                  <MenuOutline v-if="collapsed" />
                  <CloseOutline v-else />
                </n-icon>
              </template>
            </n-button>
            <div class="ml-4 w-[240px]">
              <n-input v-model:value="searchQuery" placeholder="搜索动漫、角色或声优...">
                <template #prefix>
                  <n-icon><SearchOutline /></n-icon>
                </template>
              </n-input>
            </div>
          </div>

          <div class="flex items-center space-x-4">
            <n-badge dot>
              <n-button quaternary circle>
                <template #icon><n-icon><NotificationsOutline /></n-icon></template>
              </n-button>
            </n-badge>
            <n-button quaternary circle @click="toggleTheme">
              <template #icon>
                <n-icon>
                  <SunnyOutline v-if="isDark" />
                  <MoonOutline v-else />
                </n-icon>
              </template>
            </n-button>
          </div>
        </n-layout-header>

        <!-- 内容区域 -->
        <n-layout-content
          content-style="padding: 20px;"
          :native-scrollbar="false"
        >
          <n-card>
            <template #header>
              <div class="flex items-center justify-between">
                <div>
                  <h2 class="text-xl font-bold">{{ pageTitle }}</h2>
                  <p class="text-sm text-gray-500">{{ pageDescription }}</p>
                </div>
                <div class="flex items-center space-x-2">
                  <n-button>筛选</n-button>
                  <n-button type="primary">添加动漫</n-button>
                </div>
              </div>
            </template>
            
            <!-- 页面内容 -->
            <router-view v-slot="{ Component }">
              <transition name="fade" mode="out-in">
                <component :is="Component" />
              </transition>
            </router-view>
          </n-card>
        </n-layout-content>
      </n-layout>
    </n-layout>

    <!-- AI 助手按钮 -->
    <n-affix :bottom="30" :right="30">
      <n-button type="primary" circle size="large" @click="showChat = !showChat">
        <template #icon>
          <n-icon>
            <ChatbubbleEllipsesOutline v-if="!showChat" />
            <CloseOutline v-else />
          </n-icon>
        </template>
      </n-button>
    </n-affix>

    <!-- AI 助手对话框 -->
    <n-modal
      v-model:show="showChat"
      preset="card"
      style="width: 400px"
      :title="'AI 助手'"
      :bordered="false"
      size="huge"
      :segmented="{ content: true, footer: 'soft' }"
      :auto-focus="false"
      :mask-closable="false"
      transform-origin="right bottom"
    >
      <div class="h-[400px] overflow-y-auto p-2 space-y-4">
        <div v-for="(message, index) in chatMessages" :key="index" 
             :class="['flex', message.type === 'user' ? 'justify-end' : 'justify-start']">
          <n-thing :title="message.type === 'user' ? '你' : 'AI 助手'" :title-extra="message.time">
            <n-tag :type="message.type === 'user' ? 'primary' : 'default'" size="large">
              {{ message.content }}
            </n-tag>
          </n-thing>
        </div>
      </div>
      <template #footer>
        <div class="flex items-center space-x-2">
          <n-input v-model:value="userInput" type="text" placeholder="输入消息..." @keydown.enter="sendMessage" />
          <n-button type="primary" @click="sendMessage">发送</n-button>
        </div>
      </template>
    </n-modal>
  </n-config-provider>
</template>

<script setup>
import { ref, computed, h } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { 
  MenuOutline, 
  CloseOutline, 
  SearchOutline, 
  NotificationsOutline, 
  SunnyOutline, 
  MoonOutline, 
  ChatbubbleEllipsesOutline,
  HomeOutline,
  FilmOutline,
  DownloadOutline,
  RssOutline,
  SettingsOutline
} from '@vicons/ionicons5'
import { 
  NConfigProvider, 
  NLayout, 
  NLayoutSider, 
  NLayoutHeader, 
  NLayoutContent,
  NMenu, 
  NAvatar, 
  NButton, 
  NInput, 
  NBadge, 
  NIcon, 
  NCard, 
  NTag, 
  NDropdown,
  NAffix,
  NModal,
  NThing,
  darkTheme
} from 'naive-ui'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

// 侧边栏折叠状态
const collapsed = ref(false)

// 搜索查询
const searchQuery = ref('')

// 聊天相关
const showChat = ref(false)
const userInput = ref('')
const chatMessages = ref([
  { type: 'assistant', content: '您好！我是您的AI助手，有什么可以帮您的吗？', time: '09:30' },
  { type: 'user', content: '我想找一部新番看', time: '09:31' }
])

// 发送消息
const sendMessage = () => {
  if (!userInput.value.trim()) return
  
  // 添加用户消息
  chatMessages.value.push({
    type: 'user',
    content: userInput.value,
    time: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  })
  
  // 模拟AI回复
  setTimeout(() => {
    chatMessages.value.push({
      type: 'assistant',
      content: '我正在为您查找相关信息，请稍候...',
      time: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    })
  }, 1000)
  
  userInput.value = ''
}

// 主题切换
const isDark = ref(false)
const theme = computed(() => isDark.value ? darkTheme : null)

const toggleTheme = () => {
  isDark.value = !isDark.value
}

// 菜单配置
const menuOptions = [
  {
    label: '首页',
    key: 'dashboard',
    icon: HomeOutline
  },
  {
    label: '动漫库',
    key: 'anime',
    icon: FilmOutline,
    children: [
      {
        label: '全部动漫',
        key: 'anime-list'
      },
      {
        label: '我的收藏',
        key: 'anime-favorites'
      }
    ]
  },
  {
    label: '下载管理',
    key: 'downloads',
    icon: DownloadOutline
  },
  {
    label: 'RSS订阅',
    key: 'rss',
    icon: RssOutline
  },
  {
    label: '设置',
    key: 'settings',
    icon: SettingsOutline
  }
]

// 用户下拉菜单选项
const userOptions = [
  {
    label: '个人资料',
    key: 'profile'
  },
  {
    label: '设置',
    key: 'settings'
  },
  {
    type: 'divider',
    key: 'd1'
  },
  {
    label: '退出登录',
    key: 'logout'
  }
]

// 菜单渲染函数
const renderMenuLabel = (option) => {
  return option.label
}

const renderMenuIcon = (option) => {
  return option.icon ? h(NIcon, null, { default: () => h(option.icon) }) : null
}

// 当前激活的菜单项
const activeKey = computed(() => {
  const path = route.path
  if (path.startsWith('/naive/anime')) return 'anime-list'
  if (path.startsWith('/naive/downloads')) return 'downloads'
  if (path.startsWith('/naive/rss')) return 'rss'
  if (path.startsWith('/naive/settings')) return 'settings'
  return 'dashboard'
})

// 页面标题和描述
const pageTitle = computed(() => {
  const path = route.path
  if (path.startsWith('/naive/anime')) return '动漫库'
  if (path.startsWith('/naive/downloads')) return '下载管理'
  if (path.startsWith('/naive/rss')) return 'RSS订阅'
  if (path.startsWith('/naive/settings')) return '设置'
  return '控制面板'
})

const pageDescription = computed(() => {
  const path = route.path
  if (path.startsWith('/naive/anime')) return '发现和管理你喜爱的动漫'
  if (path.startsWith('/naive/downloads')) return '管理你的下载任务'
  if (path.startsWith('/naive/rss')) return '管理你的订阅源'
  if (path.startsWith('/naive/settings')) return '自定义你的应用设置'
  return '查看你的追番情况'
})

// 菜单点击处理
const handleUpdateValue = (key) => {
  switch (key) {
    case 'dashboard':
      router.push('/naive')
      break
    case 'anime-list':
      router.push('/naive/anime')
      break
    case 'anime-favorites':
      router.push('/naive/anime/favorites')
      break
    case 'downloads':
      router.push('/naive/downloads')
      break
    case 'rss':
      router.push('/naive/rss')
      break
    case 'settings':
      router.push('/naive/settings')
      break
  }
}

// 用户菜单处理
const handleUserSelect = (key) => {
  if (key === 'logout') {
    authStore.logout()
    router.push('/login')
  } else if (key === 'profile') {
    router.push('/profile')
  } else if (key === 'settings') {
    router.push('/settings')
  }
}
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style> 