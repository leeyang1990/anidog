<template>
  <div>
    <!-- 筛选工具栏 -->
    <n-space vertical :size="16">
      <n-card size="small" :bordered="false" class="filter-card">
        <n-space align="center" :wrap="true" :size="16">
          <n-input
            v-model:value="searchQuery"
            placeholder="搜索动漫..."
            clearable
            style="width: 200px"
          >
            <template #prefix>
              <n-icon><SearchOutline /></n-icon>
            </template>
          </n-input>
          
          <n-select
            v-model:value="statusFilter"
            placeholder="状态筛选"
            clearable
            :options="statusOptions"
            style="width: 140px"
          />
          
          <n-select
            v-model:value="sortBy"
            placeholder="排序方式"
            :options="sortOptions"
            style="width: 160px"
          />
          
          <n-button-group>
            <n-button
              :type="viewMode === 'grid' ? 'primary' : 'default'"
              @click="viewMode = 'grid'"
            >
              <template #icon><n-icon><GridOutline /></n-icon></template>
              网格
            </n-button>
            <n-button
              :type="viewMode === 'list' ? 'primary' : 'default'"
              @click="viewMode = 'list'"
            >
              <template #icon><n-icon><ListOutline /></n-icon></template>
              列表
            </n-button>
          </n-button-group>
        </n-space>
      </n-card>

      <!-- 加载状态 -->
      <n-spin :show="loading">
        <!-- 网格视图 -->
        <div v-if="viewMode === 'grid'">
          <div v-if="filteredAnimeList.length === 0" class="empty-state">
            <n-empty description="暂无动漫" size="large">
              <template #extra>
                <n-button type="primary" @click="handleAddAnime">添加动漫</n-button>
              </template>
            </n-empty>
          </div>
          <div v-else class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-4 xl:grid-cols-5 gap-8">
            <naive-anime-card
              v-for="anime in filteredAnimeList"
              :key="anime.id"
              :anime="anime"
              @click="handleAnimeClick(anime)"
              @subscribe="handleSubscribe(anime)"
              @unsubscribe="handleUnsubscribe(anime)"
              @favorite="handleFavorite(anime)"
              @download="handleDownload(anime)"
              @share="handleShare(anime)"
              @report="handleReport(anime)"
            />
          </div>
        </div>

        <!-- 列表视图 -->
        <div v-else>
          <n-data-table
            :columns="columns"
            :data="filteredAnimeList"
            :pagination="pagination"
            :bordered="false"
            :row-class-name="rowClassName"
            @update:page="handlePageChange"
          />
        </div>
      </n-spin>

      <!-- 分页 -->
      <n-pagination
        v-if="viewMode === 'grid' && filteredAnimeList.length > 0"
        v-model:page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :item-count="totalAnime"
        :page-sizes="[10, 20, 30, 40]"
        show-size-picker
        @update:page="handlePageChange"
        @update:page-size="handlePageSizeChange"
      />
    </n-space>
  </div>
</template>

<script setup>
import { ref, computed, h } from 'vue'
import { useRouter } from 'vue-router'
import { 
  NSpace, 
  NCard, 
  NInput, 
  NSelect, 
  NButtonGroup, 
  NButton, 
  NIcon, 
  NSpin, 
  NEmpty, 
  NDataTable, 
  NPagination, 
  NTag, 
  NPopconfirm,
  useMessage
} from 'naive-ui'
import { 
  SearchOutline, 
  GridOutline, 
  ListOutline, 
  PlayOutline, 
  HeartOutline, 
  TrashOutline, 
  PencilOutline 
} from '@vicons/ionicons5'
import NaiveAnimeCard from '../../components/Anime/NaiveAnimeCard.vue'

const router = useRouter()
const message = useMessage()

// 状态
const loading = ref(false)
const searchQuery = ref('')
const statusFilter = ref(null)
const sortBy = ref('updated_at_desc')
const viewMode = ref('grid')
const animeList = ref([])
const totalAnime = ref(0)

// 分页
const pagination = ref({
  page: 1,
  pageSize: 20,
  showSizePicker: true,
  pageSizes: [10, 20, 30, 40]
})

// 筛选选项
const statusOptions = [
  { label: '连载中', value: 'ongoing' },
  { label: '已完结', value: 'completed' },
  { label: '即将开播', value: 'upcoming' },
  { label: '已弃番', value: 'dropped' }
]

// 排序选项
const sortOptions = [
  { label: '最近更新', value: 'updated_at_desc' },
  { label: '首字母 A-Z', value: 'title_asc' },
  { label: '首字母 Z-A', value: 'title_desc' },
  { label: '评分最高', value: 'rating_desc' }
]

// 表格列配置
const columns = [
  {
    title: '封面',
    key: 'cover',
    width: 80,
    render: (row) => h('img', {
      src: row.cover_image || 'https://via.placeholder.com/60x80/1f2937/6b7280?text=No+Image',
      alt: row.title,
      style: 'width: 60px; height: 80px; object-fit: cover; border-radius: 4px;'
    })
  },
  {
    title: '标题',
    key: 'title',
    render: (row) => h('div', { class: 'flex flex-col' }, [
      h('span', { class: 'font-medium' }, row.title),
      h('span', { class: 'text-xs text-gray-500' }, row.original_title || '')
    ])
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render: (row) => {
      const statusMap = {
        'ongoing': { type: 'success', text: '连载中' },
        'completed': { type: 'info', text: '已完结' },
        'upcoming': { type: 'warning', text: '即将开播' },
        'dropped': { type: 'error', text: '已弃番' }
      }
      const status = statusMap[row.status] || { type: 'default', text: '未知' }
      return h(NTag, { type: status.type, size: 'small' }, { default: () => status.text })
    }
  },
  {
    title: '集数',
    key: 'episodes',
    width: 100,
    render: (row) => {
      const current = row.current_episode
      const total = row.total_episodes
      if (total) {
        return `${current || 0}/${total}`
      }
      return current ? `${current}` : '暂无'
    }
  },
  {
    title: '评分',
    key: 'rating',
    width: 80,
    render: (row) => row.rating ? `${row.rating}分` : '暂无'
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    render: (row) => h(NSpace, { size: 'small' }, [
      h(NButton, {
        tertiary: true,
        size: 'small',
        type: 'primary',
        onClick: () => handleAnimeClick(row),
        renderIcon: () => h(NIcon, null, { default: () => h(PlayOutline) })
      }, { default: () => '详情' }),
      
      h(NButton, {
        tertiary: true,
        size: 'small',
        type: row.is_subscribed ? 'success' : 'info',
        onClick: () => row.is_subscribed ? handleUnsubscribe(row) : handleSubscribe(row)
      }, { default: () => row.is_subscribed ? '已订阅' : '订阅' }),
      
      h(NButton, {
        tertiary: true,
        size: 'small',
        type: row.is_favorite ? 'error' : 'default',
        onClick: () => handleFavorite(row),
        renderIcon: () => h(NIcon, null, { default: () => h(HeartOutline) })
      }),
      
      h(NPopconfirm, {
        onPositiveClick: () => handleDelete(row),
        negativeText: '取消',
        positiveText: '删除'
      }, {
        trigger: () => h(NButton, {
          tertiary: true,
          size: 'small',
          type: 'error',
          renderIcon: () => h(NIcon, null, { default: () => h(TrashOutline) })
        }),
        default: () => '确定要删除这个动漫吗？'
      })
    ])
  }
]

// 计算过滤后的动漫列表
const filteredAnimeList = computed(() => {
  let result = [...animeList.value]
  
  // 搜索过滤
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(anime => 
      anime.title.toLowerCase().includes(query) || 
      (anime.original_title && anime.original_title.toLowerCase().includes(query))
    )
  }
  
  // 状态过滤
  if (statusFilter.value) {
    result = result.filter(anime => anime.status === statusFilter.value)
  }
  
  // 排序
  result.sort((a, b) => {
    switch (sortBy.value) {
      case 'title_asc':
        return a.title.localeCompare(b.title)
      case 'title_desc':
        return b.title.localeCompare(a.title)
      case 'rating_desc':
        return (b.rating || 0) - (a.rating || 0)
      case 'updated_at_desc':
      default:
        return new Date(b.updated_at || 0) - new Date(a.updated_at || 0)
    }
  })
  
  return result
})

// 行样式
const rowClassName = (row) => {
  return row.is_subscribed ? 'subscribed-row' : ''
}

// 获取动漫列表
const fetchAnimeList = async () => {
  loading.value = true
  
  try {
    // 模拟API请求
    await new Promise(resolve => setTimeout(resolve, 500))
    
    // 模拟数据
    animeList.value = [
      {
        id: 1,
        title: '进击的巨人 最终季',
        original_title: 'Shingeki no Kyojin: The Final Season',
        cover_image: 'https://img.3dmgame.com/uploads/images/news/20201207/1607304582_790291.jpg',
        status: 'completed',
        current_episode: 16,
        total_episodes: 16,
        rating: 9.8,
        is_subscribed: true,
        is_favorite: true,
        updated_at: '2023-05-15T10:30:00Z'
      },
      {
        id: 2,
        title: '鬼灭之刃 刀匠村篇',
        original_title: 'Kimetsu no Yaiba: Katanakaji no Sato-hen',
        cover_image: 'https://img.3dmgame.com/uploads/images/news/20230210/1676006885_148759.jpg',
        status: 'ongoing',
        current_episode: 8,
        total_episodes: 11,
        rating: 9.5,
        is_subscribed: true,
        is_favorite: false,
        updated_at: '2023-06-01T15:45:00Z'
      },
      {
        id: 3,
        title: '间谍过家家 第二季',
        original_title: 'Spy x Family Season 2',
        cover_image: 'https://img.3dmgame.com/uploads/images/news/20220423/1650677654_977349.jpg',
        status: 'upcoming',
        current_episode: 0,
        total_episodes: 12,
        rating: null,
        is_subscribed: false,
        is_favorite: false,
        updated_at: '2023-05-28T09:15:00Z'
      },
      {
        id: 4,
        title: '咒术回战 第二季',
        original_title: 'Jujutsu Kaisen Season 2',
        cover_image: 'https://img.3dmgame.com/uploads/images/news/20230706/1688607494_256815.jpg',
        status: 'ongoing',
        current_episode: 3,
        total_episodes: 24,
        rating: 9.7,
        is_subscribed: true,
        is_favorite: true,
        updated_at: '2023-06-05T12:00:00Z'
      },
      {
        id: 5,
        title: '海贼王',
        original_title: 'One Piece',
        cover_image: 'https://img.3dmgame.com/uploads/images/news/20230328/1679968367_256815.jpg',
        status: 'ongoing',
        current_episode: 1071,
        total_episodes: null,
        rating: 9.6,
        is_subscribed: true,
        is_favorite: true,
        updated_at: '2023-06-04T08:30:00Z'
      },
      {
        id: 6,
        title: '葬送的芙莉莲',
        original_title: 'Sousou no Frieren',
        cover_image: 'https://img.3dmgame.com/uploads/images/news/20230929/1695952401_977349.jpg',
        status: 'ongoing',
        current_episode: 10,
        total_episodes: 28,
        rating: 9.9,
        is_subscribed: false,
        is_favorite: false,
        updated_at: '2023-06-03T20:15:00Z'
      }
    ]
    
    // 设置总数
    totalAnime.value = animeList.value.length
  } catch (error) {
    console.error('获取动漫列表失败:', error)
    message.error('获取动漫列表失败')
  } finally {
    loading.value = false
  }
}

// 事件处理
const handlePageChange = (page) => {
  pagination.value.page = page
  fetchAnimeList()
}

const handlePageSizeChange = (pageSize) => {
  pagination.value.pageSize = pageSize
  pagination.value.page = 1
  fetchAnimeList()
}

const handleAnimeClick = (anime) => {
  router.push(`/naive/anime/${anime.id}`)
}

const handleSubscribe = (anime) => {
  anime.is_subscribed = true
  message.success(`已订阅《${anime.title}》`)
}

const handleUnsubscribe = (anime) => {
  anime.is_subscribed = false
  message.success(`已取消订阅《${anime.title}》`)
}

const handleFavorite = (anime) => {
  anime.is_favorite = !anime.is_favorite
  if (anime.is_favorite) {
    message.success(`已收藏《${anime.title}》`)
  } else {
    message.info(`已取消收藏《${anime.title}》`)
  }
}

const handleDelete = (anime) => {
  const index = animeList.value.findIndex(item => item.id === anime.id)
  if (index !== -1) {
    animeList.value.splice(index, 1)
    totalAnime.value--
    message.success(`已删除《${anime.title}》`)
  }
}

const handleDownload = (anime) => {
  message.info(`正在下载《${anime.title}》`)
}

const handleShare = (anime) => {
  message.info(`分享链接已复制到剪贴板`)
}

const handleReport = (anime) => {
  message.info(`已收到您对《${anime.title}》的问题报告`)
}

const handleAddAnime = () => {
  message.info('添加动漫功能正在开发中')
}

// 初始化
fetchAnimeList()
</script>

<style scoped>
.filter-card {
  background-color: rgba(255, 255, 255, 0.7);
  backdrop-filter: blur(10px);
}

.empty-state {
  padding: 3rem 0;
}

:deep(.subscribed-row) {
  background-color: rgba(0, 255, 0, 0.05);
}
</style> 