<!-- AnimeCard.vue -->
<template>
  <div class="group relative">
    <!-- 卡片主体 -->
    <div class="relative bg-white/80 dark:bg-gray-800/80 backdrop-blur-sm rounded-2xl overflow-hidden shadow-lg hover:shadow-2xl transition-all duration-500 transform hover:-translate-y-2 hover:scale-105">
      <!-- 封面 -->
      <div class="relative aspect-[2/3] overflow-hidden bg-gradient-to-br from-gray-100 to-gray-200 dark:from-gray-700 dark:to-gray-800">
        <img 
          :src="anime.cover_image || defaultCover" 
          :alt="anime.title"
          class="w-full h-full object-cover transition-transform duration-500 group-hover:scale-110"
          @error="handleImageError"
          @load="handleImageLoad"
          loading="lazy"
        >
        
        <!-- 加载中状态 -->
        <div v-if="imageLoading" class="absolute inset-0 flex items-center justify-center bg-gray-100 dark:bg-gray-800">
          <div class="w-8 h-8 border-2 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
        </div>
        
        <!-- 渐变遮罩 -->
        <div class="absolute inset-0 bg-gradient-to-t from-black/80 via-black/30 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500"></div>
        
        <!-- 状态标签 -->
        <div class="absolute top-3 right-3 px-3 py-1 rounded-full text-xs font-bold backdrop-blur-md border border-white/30 shadow-lg" :class="statusClass">
          {{ statusText }}
        </div>
        
        <!-- 悬停信息 -->
        <div class="absolute inset-0 p-6 text-white flex flex-col justify-end transform translate-y-full group-hover:translate-y-0 transition-transform duration-500">
          <h3 class="font-bold text-xl mb-3 line-clamp-2 drop-shadow-lg">{{ anime.title }}</h3>
          <p v-if="anime.description" class="text-sm text-gray-200 line-clamp-3 mb-4 drop-shadow">{{ anime.description }}</p>
          <div class="flex items-center justify-between text-sm mb-4">
            <span class="font-semibold bg-white/20 px-3 py-1 rounded-full backdrop-blur-sm">{{ episodeText }}</span>
            <span v-if="anime.release_time" class="text-gray-300 bg-white/10 px-3 py-1 rounded-full backdrop-blur-sm">{{ anime.release_time }}</span>
          </div>
          
          <!-- 进度条 -->
          <div v-if="anime.total_episodes && anime.current_episode" class="w-full bg-white/20 rounded-full h-2 overflow-hidden mb-4">
            <div 
              class="h-full bg-gradient-to-r from-blue-400 to-purple-500 rounded-full transition-all duration-300"
              :style="{ width: progressPercentage + '%' }"
            ></div>
          </div>
          
          <!-- 操作按钮 -->
          <div class="flex gap-3">
            <button
              v-if="anime.is_subscribed"
              @click.stop="$emit('unsubscribe')"
              class="flex-1 py-3 px-4 bg-emerald-500/90 hover:bg-emerald-600 text-white rounded-xl text-sm font-bold transition-colors duration-300 backdrop-blur-sm"
            >
              ✓ 已订阅
            </button>
            <button
              v-else
              @click.stop="$emit('subscribe')"
              class="flex-1 py-3 px-4 bg-blue-600/90 hover:bg-blue-700 text-white rounded-xl text-sm font-bold transition-colors duration-300 backdrop-blur-sm"
            >
              + 订阅
            </button>
            
            <button 
              @click.stop="$emit('more')" 
              class="py-3 px-4 bg-white/20 hover:bg-white/30 text-white rounded-xl text-sm font-bold transition-colors duration-300 backdrop-blur-sm"
            >
              ⋯
            </button>
          </div>
        </div>
      </div>
      
      <!-- 简化的底部信息栏（仅显示标题，不悬停时显示） -->
      <div class="p-4 group-hover:opacity-0 transition-opacity duration-500">
        <h3 class="font-bold text-gray-900 dark:text-white line-clamp-2 text-center leading-tight">{{ anime.title }}</h3>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  anime: {
    type: Object,
    required: true
  }
})

const emit = defineEmits(['click', 'subscribe', 'unsubscribe', 'play', 'favorite', 'more'])

// 图片加载状态
const imageLoading = ref(true)

// 默认封面
const defaultCover = ref('data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMzAwIiBoZWlnaHQ9IjQwMCIgdmlld0JveD0iMCAwIDMwMCA0MDAiIGZpbGw9Im5vbmUiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+CjxkZWZzPgo8bGluZWFyR3JhZGllbnQgaWQ9ImciIHgxPSIwJSIgeTE9IjAlIiB4Mj0iMTAwJSIgeTI9IjEwMCUiPgo8c3RvcCBvZmZzZXQ9IjAlIiBzdHlsZT0ic3RvcC1jb2xvcjojNjY2NjY2O3N0b3Atb3BhY2l0eToxIiAvPgo8c3RvcCBvZmZzZXQ9IjEwMCUiIHN0eWxlPSJzdG9wLWNvbG9yOiM5OTk5OTk7c3RvcC1vcGFjaXR5OjEiIC8+CjwvbGluZWFyR3JhZGllbnQ+CjwvZGVmcz4KPHJlY3Qgd2lkdGg9IjMwMCIgaGVpZ2h0PSI0MDAiIGZpbGw9InVybCgjZykiLz4KPHN2ZyB4PSI5MCIgeT0iMTUwIiB3aWR0aD0iMTIwIiBoZWlnaHQ9IjEwMCI+CjxyZWN0IHdpZHRoPSIxMjAiIGhlaWdodD0iMTAwIiByeD0iMTAiIGZpbGw9IiNmZmZmZmYyMCIvPgo8Y2lyY2xlIGN4PSI2MCIgY3k9IjQwIiByPSIyMCIgZmlsbD0iI2ZmZmZmZjQwIi8+Cjx0ZXh0IHg9IjYwIiB5PSI4NSIgZm9udC1mYW1pbHk9IkFyaWFsLCBzYW5zLXNlcmlmIiBmb250LXNpemU9IjEyIiBmaWxsPSIjZmZmZmZmODAiIHRleHQtYW5jaG9yPSJtaWRkbGUiPuaaguaXoDlhtuWDj+mZhOijpeeUqDwvdGV4dD4KPC9zdmc+Cjwvc3ZnPgo=')

// 处理图片加载
const handleImageLoad = () => {
  imageLoading.value = false
}

// 处理图片加载错误
const handleImageError = (e) => {
  e.target.src = defaultCover.value
  imageLoading.value = false
}

// 状态样式
const statusClass = computed(() => {
  const status = props.anime.status
  const classes = {
    'ongoing': 'bg-emerald-500/90 text-white',
    'finished': 'bg-blue-500/90 text-white',
    'upcoming': 'bg-amber-500/90 text-white',
    'dropped': 'bg-red-500/90 text-white',
    'unknown': 'bg-gray-500/90 text-white'
  }
  return classes[status] || 'bg-gray-500/90 text-white'
})

// 状态文本
const statusText = computed(() => {
  const statusMap = {
    'ongoing': '连载中',
    'finished': '已完结',
    'upcoming': '即将开播',
    'dropped': '已弃番',
    'unknown': '未知'
  }
  return statusMap[props.anime.status] || '未知'
})

// 集数文本
const episodeText = computed(() => {
  const current = props.anime.current_episode || 0
  const total = props.anime.total_episodes
  if (total) {
    return `${current}/${total}`
  }
  return current ? `第${current}集` : '暂无'
})

// 进度百分比
const progressPercentage = computed(() => {
  const current = props.anime.current_episode || 0
  const total = props.anime.total_episodes
  if (total && current) {
    return Math.min((current / total) * 100, 100)
  }
  return 0
})
</script>

<style scoped>
.aspect-\[2\/3\] {
  aspect-ratio: 2/3;
}

.line-clamp-1 {
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.line-clamp-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* 优化图片渲染 */
img {
  image-rendering: -webkit-optimize-contrast;
  image-rendering: crisp-edges;
}

/* 现代化的卡片效果 */
.group:hover {
  filter: brightness(1.05);
}

/* 阴影层次 */
.group > div {
  box-shadow: 
    0 4px 6px -1px rgba(0, 0, 0, 0.1),
    0 2px 4px -1px rgba(0, 0, 0, 0.06);
}

.group:hover > div {
  box-shadow: 
    0 20px 25px -5px rgba(0, 0, 0, 0.1),
    0 10px 10px -5px rgba(0, 0, 0, 0.04);
}

/* 状态标签的微妙动画 */
.group:hover .absolute.top-3 {
  transform: translateY(-2px) scale(1.05);
  transition: transform 0.3s ease;
}
</style> 