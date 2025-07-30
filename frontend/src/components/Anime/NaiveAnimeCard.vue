<template>
  <n-card
    class="anime-card"
    :bordered="false"
    size="small"
    hoverable
    @click="$emit('click')"
  >
    <div class="relative aspect-[2/3] overflow-hidden">
      <!-- 封面图片 -->
      <n-image
        :src="anime.cover_image || defaultCover"
        :alt="anime.title"
        object-fit="cover"
        preview-disabled
        class="w-full h-full transition-transform duration-500 hover:scale-110"
        @error="handleImageError"
      />
      
      <!-- 顶部状态标签 -->
      <div class="absolute top-3 right-3">
        <n-tag :type="statusTagType" size="small" round strong>
          {{ statusText }}
        </n-tag>
      </div>
      
      <!-- 评分标签 -->
      <div v-if="anime.rating" class="absolute top-3 left-3">
        <n-tag type="warning" size="small" round strong>
          <template #icon>
            <n-icon><StarOutline /></n-icon>
          </template>
          {{ anime.rating }}
        </n-tag>
      </div>
      
      <!-- 底部渐变遮罩 -->
      <div class="absolute inset-x-0 bottom-0 h-32 bg-gradient-to-t from-black/90 via-black/50 to-transparent"></div>
      
      <!-- 底部信息 -->
      <div class="absolute bottom-0 left-0 right-0 p-4">
        <h3 class="text-white font-bold text-lg mb-2 line-clamp-2 drop-shadow-lg">{{ anime.title }}</h3>
        <div class="flex items-center justify-between text-sm text-gray-200">
          <span class="bg-white/20 px-2 py-1 rounded-full backdrop-blur-sm font-medium">{{ episodeText }}</span>
          <span v-if="anime.release_time" class="bg-white/10 px-2 py-1 rounded-full backdrop-blur-sm">{{ anime.release_time }}</span>
        </div>
      </div>
      
      <!-- 悬停时显示的操作按钮 -->
      <div class="absolute inset-0 bg-black/50 opacity-0 hover:opacity-100 transition-opacity duration-300 flex items-center justify-center">
        <n-button-group>
          <n-button tertiary type="primary" strong @click.stop="$emit('play')">
            <template #icon><n-icon><PlayOutline /></n-icon></template>
            播放
          </n-button>
          <n-button tertiary type="info" strong @click.stop="$emit('detail')">
            <template #icon><n-icon><InformationCircleOutline /></n-icon></template>
            详情
          </n-button>
        </n-button-group>
      </div>
    </div>
    
    <!-- 底部操作区 -->
    <div class="pt-4 flex items-center justify-between">
      <div>
        <n-button
          v-if="anime.is_subscribed"
          @click.stop="$emit('unsubscribe')"
          size="small"
          type="success"
          strong
        >
          <template #icon><n-icon><CheckmarkOutline /></n-icon></template>
          已订阅
        </n-button>
        <n-button
          v-else
          @click.stop="$emit('subscribe')"
          size="small"
          type="primary"
          strong
        >
          <template #icon><n-icon><AddOutline /></n-icon></template>
          订阅
        </n-button>
      </div>
      
      <div class="flex items-center space-x-1">
        <n-button
          quaternary
          circle
          size="small"
          @click.stop="$emit('favorite')"
          :type="anime.is_favorite ? 'error' : 'default'"
        >
          <template #icon>
            <n-icon>
              <component :is="anime.is_favorite ? HeartFilled : HeartOutline" />
            </n-icon>
          </template>
        </n-button>
        <n-dropdown
          trigger="click"
          :options="dropdownOptions"
          @select="handleDropdownSelect"
          placement="bottom-end"
        >
          <n-button quaternary circle size="small" @click.stop>
            <template #icon><n-icon><EllipsisHorizontalOutline /></n-icon></template>
          </n-button>
        </n-dropdown>
      </div>
    </div>
  </n-card>
</template>

<script setup>
import { ref, computed, h } from 'vue'
import { 
  NCard, 
  NImage, 
  NTag, 
  NButton, 
  NButtonGroup, 
  NIcon, 
  NDropdown 
} from 'naive-ui'
import { 
  StarOutline, 
  PlayOutline, 
  InformationCircleOutline, 
  CheckmarkOutline, 
  AddOutline, 
  HeartOutline, 
  EllipsisHorizontalOutline,
  DownloadOutline,
  ShareSocial,
  Warning
} from '@vicons/ionicons5'
import { HeartFilled } from '@vicons/antd'

const props = defineProps({
  anime: {
    type: Object,
    required: true
  }
})

const emit = defineEmits([
  'click', 
  'subscribe', 
  'unsubscribe', 
  'play', 
  'detail', 
  'favorite', 
  'download', 
  'share', 
  'report'
])

// 默认封面
const defaultCover = ref('https://via.placeholder.com/300x400/1f2937/6b7280?text=No+Image')

// 处理图片加载错误
const handleImageError = (e) => {
  e.target.src = defaultCover.value
}

// 状态标签类型
const statusTagType = computed(() => {
  const status = props.anime.status
  const types = {
    'ongoing': 'success',
    'completed': 'info',
    'upcoming': 'warning',
    'dropped': 'error'
  }
  return types[status] || 'default'
})

// 状态文本
const statusText = computed(() => {
  const statusMap = {
    'ongoing': '连载中',
    'completed': '已完结',
    'upcoming': '即将开播',
    'dropped': '已弃番'
  }
  return statusMap[props.anime.status] || '未知'
})

// 集数文本
const episodeText = computed(() => {
  const current = props.anime.current_episode
  const total = props.anime.total_episodes
  if (total) {
    return `${current || 0}/${total}集`
  }
  return current ? `第${current}集` : '暂无'
})

// 下拉菜单选项
const dropdownOptions = [
  {
    label: '下载',
    key: 'download',
    icon: () => h(NIcon, null, { default: () => h(DownloadOutline) })
  },
  {
    label: '分享',
    key: 'share',
    icon: () => h(NIcon, null, { default: () => h(ShareSocial) })
  },
  {
    type: 'divider'
  },
  {
    label: '报告问题',
    key: 'report',
    icon: () => h(NIcon, null, { default: () => h(Warning) })
  }
]

// 处理下拉菜单选择
const handleDropdownSelect = (key) => {
  emit(key)
}
</script>

<style scoped>
.anime-card {
  transition: all 0.3s ease;
  overflow: hidden;
}

.anime-card:hover {
  transform: translateY(-8px) scale(1.02);
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.2);
}

.aspect-\[2\/3\] {
  aspect-ratio: 2/3;
}

.line-clamp-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style> 