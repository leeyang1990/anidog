<template>
  <div class="min-h-screen bg-gradient-to-br from-slate-50 via-blue-50 to-indigo-50 dark:from-gray-900 dark:via-slate-900 dark:to-indigo-950 transition-colors duration-500">
    
    <!-- 页面头部 -->
    <div class="relative overflow-hidden">
      <div class="absolute inset-0 bg-gradient-to-r from-blue-600/10 via-purple-600/5 to-pink-600/10"></div>
      <div class="relative px-6 py-12">
        <div class="max-w-7xl mx-auto">
          <div class="text-center mb-8">
            <h1 class="text-5xl font-black text-gray-900 dark:text-white mb-4">
              <span class="bg-gradient-to-r from-blue-600 via-purple-600 to-pink-600 bg-clip-text text-transparent">
                Anime
              </span>
              <span class="text-gray-800 dark:text-gray-200">Collection</span>
            </h1>
            <p class="text-xl text-gray-600 dark:text-gray-400 font-light">
              探索无限精彩的动漫世界 ✨
            </p>
            <div class="mt-4 text-sm text-gray-500 dark:text-gray-500">
              共收录 <span class="font-semibold text-blue-600 dark:text-blue-400">{{ totalAnimes }}</span> 部优质作品
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 主要内容区域 -->
    <div class="max-w-7xl mx-auto px-6 pb-12">
      <!-- 加载状态 -->
      <n-spin :show="loading" size="large">
        <template #description>
          <div class="text-center mt-4">
            <p class="text-lg font-medium text-gray-700 dark:text-gray-300 mb-2">加载中...</p>
            <p class="text-gray-500 dark:text-gray-500">正在为您寻找精彩内容</p>
          </div>
        </template>

        <!-- 海报墙 -->
        <div class="poster-wall">
          <div
            v-for="(anime, index) in testAnimeData"
            :key="anime.id || index"
            class="poster-item"
            @click="handleAnimeClick(anime)"
          >
            <!-- 海报卡片 -->
            <div class="poster-card">
              <!-- 海报图片 -->
              <div class="relative overflow-hidden" style="aspect-ratio: 2/3; min-height: 200px;">
                <img 
                  :src="anime.cover_url || 'https://via.placeholder.com/200x300/1f2937/6b7280?text=No+Image'" 
                  :alt="anime.title"
                  class="w-full h-full object-cover"
                  loading="lazy"
                >
              </div>
              
              <!-- 底部标题栏 -->
              <div class="p-2 bg-white dark:bg-gray-800">
                <h3 class="font-medium text-gray-900 dark:text-white text-center text-sm line-clamp-2 leading-tight">
                  {{ anime.title }}
                </h3>
              </div>
            </div>
          </div>
        </div>

        <!-- 空状态 -->
        <div v-if="false" class="text-center py-24">
          <div class="text-gray-400 mb-4">
            <svg class="w-24 h-24 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M7 4V2a1 1 0 011-1h8a1 1 0 011 1v2h4a1 1 0 110 2h-1v12a2 2 0 01-2 2H6a2 2 0 01-2-2V6H3a1 1 0 110-2h4zM9 6h6v10H9V6z"/>
            </svg>
          </div>
          <h3 class="text-xl font-medium text-gray-700 dark:text-gray-300 mb-2">暂无动漫</h3>
          <p class="text-gray-500 dark:text-gray-500">还没有添加任何动漫，去发现一些精彩内容吧！</p>
        </div>
      </n-spin>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed, h } from 'vue'
import { useRouter } from 'vue-router'
import { 
  NCard, 
  NSpace, 
  NInput, 
  NSelect, 
  NButtonGroup, 
  NButton, 
  NIcon, 
  NStatistic, 
  NAlert, 
  NSpin, 
  NPagination, 
  NEmpty, 
  NDataTable, 
  NTag, 
  NAffix,
  useMessage 
} from 'naive-ui'
import {
  SearchOutline,
  GridOutline,
  ListOutline,
  LibraryOutline,
  PlayCircleOutline,
  CheckmarkCircleOutline,
  TimeOutline,
  FilmOutline,
  AddOutline,
  RadioButtonOnOutline,
  StarOutline
} from '@vicons/ionicons5'
import AnimeCard from '../../components/Anime/AnimeCard.vue'
import { get } from '../../utils/api'

const router = useRouter()
const message = useMessage()
const loading = ref(false)
const error = ref('')
const animeList = ref([])
const searchQuery = ref('')
const statusFilter = ref('')
const viewMode = ref('grid')
const pagination = ref({
  page: 1,
  pageSize: 24
})

// 统计数据
const totalAnimes = ref(0)
const ongoingCount = ref(0)
const finishedCount = ref(0)
const upcomingCount = ref(0)

// 状态选项
const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '连载中', value: 'ongoing' },
  { label: '已完结', value: 'finished' },
  { label: '即将播出', value: 'upcoming' },
  { label: '未知状态', value: 'unknown' }
]

// 表格分页配置
const tablePagination = ref({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [10, 20, 30, 50],
  showQuickJumper: true
})

// 表格列配置
const tableColumns = [
  {
    title: '封面',
    key: 'cover',
    width: 80,
    render: (row) => h('img', {
      src: row.cover_url || 'https://via.placeholder.com/60x80/1f2937/6b7280?text=No+Image',
      alt: row.title,
      style: 'width: 60px; height: 80px; object-fit: cover; border-radius: 8px;'
    })
  },
  {
    title: '标题',
    key: 'title',
    minWidth: 200,
    render: (row) => h('div', { class: 'font-medium' }, row.title)
  },
  {
    title: '状态',
    key: 'status',
    width: 120,
    render: (row) => {
      const statusMap = {
        'ongoing': { type: 'success', text: '连载中' },
        'finished': { type: 'info', text: '已完结' },
        'upcoming': { type: 'warning', text: '即将播出' },
        'unknown': { type: 'default', text: '未知' }
      }
      const status = statusMap[row.status] || { type: 'default', text: '未知' }
      return h(NTag, { type: status.type, size: 'small', round: true }, { default: () => status.text })
    }
  },
  {
    title: '集数',
    key: 'episodes',
    width: 120,
    render: (row) => {
      const current = row.current_episode || 0
      const total = row.episode_count
      return total ? `${current}/${total}` : (current ? `${current}` : '暂无')
    }
  },
  {
    title: '年份',
    key: 'year',
    width: 80,
    render: (row) => row.year || '未知'
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
        onClick: () => handleAnimeClick(row)
      }, { default: () => '详情' }),
      
      h(NButton, {
        tertiary: true,
        size: 'small',
        type: 'info',
        onClick: () => handleSubscribe(row)
      }, { default: () => '订阅' }),
      
      h(NButton, {
        tertiary: true,
        size: 'small',
        type: 'success',
        onClick: () => handlePlay(row)
      }, { default: () => '播放' })
    ])
  }
]

// 测试数据 - 使用真实的动漫海报
const testAnimeData = [
  {
    id: 1,
    title: '进击的巨人 最终季',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1948/120625.jpg',
    status: 'finished',
    current_episode: 16,
    episode_count: 16,
    year: 2023,
    description: '人类与巨人的最终决战'
  },
  {
    id: 2,
    title: '鬼灭之刃 刀匠村篇',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1765/135099.jpg',
    status: 'finished',
    current_episode: 11,
    episode_count: 11,
    year: 2023,
    description: '炭治郎前往刀匠村的新冒险'
  },
  {
    id: 3,
    title: '间谍过家家',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1441/122795.jpg',
    status: 'finished',
    current_episode: 12,
    episode_count: 12,
    year: 2022,
    description: '伪装家庭的温馨喜剧'
  },
  {
    id: 4,
    title: '咒术回战 第二季',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1792/138022.jpg',
    status: 'finished',
    current_episode: 23,
    episode_count: 23,
    year: 2023,
    description: '五条悟的过去与涉谷事变'
  },
  {
    id: 5,
    title: '葬送的芙莉莲',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1015/138006.jpg',
    status: 'ongoing',
    current_episode: 28,
    episode_count: 28,
    year: 2023,
    description: '精灵法师的千年之旅'
  },
  {
    id: 6,
    title: '药师少女的独白',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1708/138033.jpg',
    status: 'ongoing',
    current_episode: 24,
    episode_count: 24,
    year: 2023,
    description: '宫廷药师的推理故事'
  },
  {
    id: 7,
    title: '链锯人',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1806/126216.jpg',
    status: 'finished',
    current_episode: 12,
    episode_count: 12,
    year: 2022,
    description: '恶魔猎人的血腥冒险'
  },
  {
    id: 8,
    title: '国王排名',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1347/117616.jpg',
    status: 'finished',
    current_episode: 23,
    episode_count: 23,
    year: 2021,
    description: '聋哑王子的成长物语'
  },
  {
    id: 9,
    title: '无职转生',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1530/117776.jpg',
    status: 'finished',
    current_episode: 24,
    episode_count: 24,
    year: 2021,
    description: '废宅的异世界重生记'
  },
  {
    id: 10,
    title: '紫罗兰永恒花园',
    cover_url: 'https://cdn.myanimelist.net/images/anime/3/88097.jpg',
    status: 'finished',
    current_episode: 13,
    episode_count: 13,
    year: 2018,
    description: '自动手记人偶的感人故事'
  },
  {
    id: 11,
    title: '东京复仇者',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1839/122012.jpg',
    status: 'finished',
    current_episode: 24,
    episode_count: 24,
    year: 2021,
    description: '时间旅行拯救朋友'
  },
  {
    id: 12,
    title: '86 不存在的战区',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1987/117507.jpg',
    status: 'finished',
    current_episode: 23,
    episode_count: 23,
    year: 2021,
    description: '被遗忘的战士们的故事'
  },
  {
    id: 13,
    title: '死神 千年血战篇',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1908/135431.jpg',
    status: 'ongoing',
    current_episode: 26,
    episode_count: 52,
    year: 2022,
    description: '护庭十三队的最终决战'
  },
  {
    id: 14,
    title: '约定的梦幻岛',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1125/96929.jpg',
    status: 'finished',
    current_episode: 23,
    episode_count: 23,
    year: 2019,
    description: '孤儿院的逃脱计划'
  },
  {
    id: 15,
    title: '新世纪福音战士',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1314/108941.jpg',
    status: 'finished',
    current_episode: 26,
    episode_count: 26,
    year: 1995,
    description: '经典机甲动画'
  },
  {
    id: 16,
    title: '魔法少女小圆',
    cover_url: 'https://cdn.myanimelist.net/images/anime/8/21039.jpg',
    status: 'finished',
    current_episode: 12,
    episode_count: 12,
    year: 2011,
    description: '颠覆传统的魔法少女'
  },
  {
    id: 17,
    title: '你的名字',
    cover_url: 'https://cdn.myanimelist.net/images/anime/5/87048.jpg',
    status: 'finished',
    current_episode: 1,
    episode_count: 1,
    year: 2016,
    description: '新海诚的经典作品'
  },
  {
    id: 18,
    title: '天气之子',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1880/101146.jpg',
    status: 'finished',
    current_episode: 1,
    episode_count: 1,
    year: 2019,
    description: '操控天气的少年'
  },
  {
    id: 19,
    title: '铃芽之旅',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1598/127366.jpg',
    status: 'finished',
    current_episode: 1,
    episode_count: 1,
    year: 2022,
    description: '关闭災害之门的旅程'
  },
  {
    id: 20,
    title: 'JoJo的奇妙冒险 石之海',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1623/120634.jpg',
    status: 'finished',
    current_episode: 38,
    episode_count: 38,
    year: 2021,
    description: 'JOJO第六部完结篇'
  },
  {
    id: 21,
    title: '一拳超人',
    cover_url: 'https://cdn.myanimelist.net/images/anime/12/76049.jpg',
    status: 'finished',
    current_episode: 24,
    episode_count: 24,
    year: 2015,
    description: '无敌英雄的日常烦恼'
  },
  {
    id: 22,
    title: '我的英雄学院',
    cover_url: 'https://cdn.myanimelist.net/images/anime/10/78745.jpg',
    status: 'ongoing',
    current_episode: 138,
    episode_count: 150,
    year: 2016,
    description: '超能力英雄学院'
  },
  {
    id: 23,
    title: '转生史莱姆',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1694/95016.jpg',
    status: 'ongoing',
    current_episode: 48,
    episode_count: 60,
    year: 2018,
    description: '史莱姆的异世界建国记'
  },
  {
    id: 24,
    title: '盾之勇者成名录',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1490/101365.jpg',
    status: 'ongoing',
    current_episode: 38,
    episode_count: 50,
    year: 2019,
    description: '被冤枉的盾之勇者'
  },
  {
    id: 25,
    title: '海贼王',
    cover_url: 'https://cdn.myanimelist.net/images/anime/6/73245.jpg',
    status: 'ongoing',
    current_episode: 1090,
    episode_count: null,
    year: 1999,
    description: '寻找ONE PIECE的大冒险'
  },
  {
    id: 26,
    title: '火影忍者',
    cover_url: 'https://cdn.myanimelist.net/images/anime/13/17405.jpg',
    status: 'finished',
    current_episode: 720,
    episode_count: 720,
    year: 2002,
    description: '忍者世界的传奇故事'
  },
  {
    id: 27,
    title: '龙珠超',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1777/109948.jpg',
    status: 'finished',
    current_episode: 131,
    episode_count: 131,
    year: 2015,
    description: '悟空的新冒险'
  },
  {
    id: 28,
    title: '银魂',
    cover_url: 'https://cdn.myanimelist.net/images/anime/10/73274.jpg',
    status: 'finished',
    current_episode: 367,
    episode_count: 367,
    year: 2006,
    description: '江户时代的搞笑日常'
  },
  {
    id: 29,
    title: '怪物',
    cover_url: 'https://cdn.myanimelist.net/images/anime/10/18793.jpg',
    status: 'finished',
    current_episode: 74,
    episode_count: 74,
    year: 2004,
    description: '追捕连环杀手的心理悬疑'
  },
  {
    id: 30,
    title: '钢之炼金术师',
    cover_url: 'https://cdn.myanimelist.net/images/anime/1223/96541.jpg',
    status: 'finished',
    current_episode: 64,
    episode_count: 64,
    year: 2009,
    description: '炼金术的禁忌与救赎'
  }
]

// 添加调试信息
console.log('testAnimeData 初始化完成，长度:', testAnimeData.length)

// 获取番剧列表
async function fetchAnimeList() {
  loading.value = true
  error.value = ''
  console.log('fetchAnimeList 开始执行，testAnimeData:', testAnimeData.length)
  try {
    // 模拟网络延迟以展示加载效果
    await new Promise(resolve => setTimeout(resolve, 500))
    
    // 直接使用测试数据
    let filteredData = [...testAnimeData]
    console.log('复制测试数据完成，filteredData长度:', filteredData.length)
    
    // 应用状态过滤
    if (statusFilter.value) {
      filteredData = filteredData.filter(anime => anime.status === statusFilter.value)
    }
    
    // 应用搜索过滤
    if (searchQuery.value.trim()) {
      const keyword = searchQuery.value.toLowerCase().trim()
      filteredData = filteredData.filter(anime => 
        anime.title.toLowerCase().includes(keyword) ||
        anime.description.toLowerCase().includes(keyword)
      )
    }
    
    // 设置总数
    totalAnimes.value = filteredData.length
    
    // 显示所有数据，不进行分页（海报墙模式）
    animeList.value = filteredData
    console.log('设置animeList完成，animeList.value长度:', animeList.value.length)
    
    console.log('加载动漫数据:', {
      total: totalAnimes.value,
      currentPage: pagination.value.page,
      pageSize: pagination.value.pageSize,
      loaded: animeList.value.length,
      testDataLength: testAnimeData.length,
      filteredDataLength: filteredData.length,
      animeListFirst: animeList.value[0]?.title,
      statusFilter: statusFilter.value,
      searchQuery: searchQuery.value
    })
    
    // 更新统计数据
    updateStats(testAnimeData)
    
    /* 注释掉真实API调用，便于测试
    const params = {
      page: pagination.value.page,
      per_page: pagination.value.pageSize
    }
    
    if (statusFilter.value) {
      params.status = statusFilter.value
    }
    
    const response = await get('/anime', { params })
    animeList.value = response.items || []
    totalAnimes.value = response.total || 0
    
    // 更新统计数据
    await updateStats()
    */
  } catch (err) {
    console.error('获取番剧列表失败:', err)
    error.value = err.message || '获取动漫列表失败，请稍后重试'
    message.error('获取动漫列表失败，请稍后重试')
    animeList.value = []
    totalAnimes.value = 0
  } finally {
    loading.value = false
  }
}

// 更新统计数据
function updateStats(data = testAnimeData) {
  try {
    ongoingCount.value = data.filter(anime => anime.status === 'ongoing').length
    finishedCount.value = data.filter(anime => anime.status === 'finished').length
    upcomingCount.value = data.filter(anime => anime.status === 'upcoming').length
  } catch (err) {
    console.error('获取统计数据失败:', err)
  }
}

// 格式化动漫数据以适配AnimeCard组件
function formatAnimeData(anime) {
  return {
    id: anime.id,
    title: anime.title,
    cover_image: anime.cover_url,
    status: anime.status,
    current_episode: anime.current_episode,
    total_episodes: anime.episode_count,
    release_time: anime.year ? `${anime.year}年` : '',
    description: anime.description || '', // 添加描述信息
    rating: null, // 后续可添加评分功能
    is_subscribed: false // 后续可添加订阅功能
  }
}

// 搜索处理
const searchTimeout = ref(null)
function handleSearch() {
  clearTimeout(searchTimeout.value)
  searchTimeout.value = setTimeout(async () => {
    // 重置到第一页并重新获取数据
    pagination.value.page = 1
    await fetchAnimeList()
  }, 500)
}

// 过滤器变化处理
function handleFilterChange() {
  pagination.value.page = 1
  fetchAnimeList()
}

// 分页处理
function goToPage(page) {
  if (page >= 1 && page <= Math.ceil(totalAnimes.value / pagination.value.pageSize)) {
    pagination.value.page = page
    fetchAnimeList()
  }
}

// 处理每页显示数量变化
function handlePageSizeChange(pageSize) {
  pagination.value.page = 1
  pagination.value.pageSize = pageSize
  fetchAnimeList()
}

// 事件处理函数
function handleAnimeClick(anime) {
  router.push(`/anime/${anime.id}`)
}

function handleSubscribe(anime) {
  console.log('订阅动漫:', anime.title)
  message.success(`已订阅《${anime.title}》`)
  // TODO: 实现订阅功能
}

function handleUnsubscribe(anime) {
  console.log('取消订阅动漫:', anime.title)
  message.info(`已取消订阅《${anime.title}》`)
  // TODO: 实现取消订阅功能
}

function handlePlay(anime) {
  console.log('播放动漫:', anime.title)
  message.info(`正在播放《${anime.title}》`)
  // TODO: 实现播放功能
}

function handleFavorite(anime) {
  console.log('收藏动漫:', anime.title)
  message.success(`已收藏《${anime.title}》`)
  // TODO: 实现收藏功能
}

function handleMore(anime) {
  console.log('更多操作:', anime.title)
  message.info(`《${anime.title}》更多操作`)
  // TODO: 实现更多操作
}

// 辅助函数：根据状态获取文本
function getStatusText(status) {
  switch (status) {
    case 'ongoing':
      return '连载中'
    case 'finished':
      return '已完结'
    case 'upcoming':
      return '即将播出'
    default:
      return '未知'
  }
}

// 辅助函数：获取集数文本
function getEpisodeText(anime) {
  const current = anime.current_episode || 0
  const total = anime.episode_count
  if (total === 0) return '暂无集数'
  if (current === 0) return '更新中'
  return `${current}/${total}`
}

onMounted(() => {
  fetchAnimeList()
})
</script>

<style scoped>
.line-clamp-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* 统计卡片样式 */
.stats-card {
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  transition: all 0.3s ease;
}

.stats-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
}

/* 网格项目动画 */
.anime-grid-item {
  animation: fadeInUp 0.4s ease-out;
  animation-fill-mode: both;
}

.anime-grid-item:nth-child(odd) { animation-delay: 0.1s; }
.anime-grid-item:nth-child(even) { animation-delay: 0.2s; }

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* 玻璃态效果 */
.backdrop-blur-sm {
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

/* 动漫表格样式 */
.anime-table {
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(10px);
  border-radius: 12px;
  overflow: hidden;
}

/* 深色模式下的表格样式 */
@media (prefers-color-scheme: dark) {
  .anime-table {
    background: rgba(31, 41, 55, 0.8);
  }
}

/* 海报墙样式优化 */
.line-clamp-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.line-clamp-3 {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* 海报卡片优化 */
.aspect-\[2\/3\] {
  aspect-ratio: 2/3;
}

/* 自定义网格列数支持 */
.grid-cols-12 {
  grid-template-columns: repeat(12, minmax(0, 1fr));
}

.grid-cols-14 {
  grid-template-columns: repeat(14, minmax(0, 1fr));
}

/* 海报墙样式 */
.poster-wall {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
  padding: 20px 0;
}

/* 响应式调整 */
@media (max-width: 640px) {
  .poster-wall {
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 12px;
  }
}

@media (min-width: 1200px) {
  .poster-wall {
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 20px;
  }
}

.poster-item {
  position: relative;
  cursor: pointer;
  transition: transform 0.3s ease;
}

.poster-item:hover {
  transform: translateY(-4px) scale(1.02);
}

.poster-card {
  background: white;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  transition: all 0.3s ease;
  border: 1px solid rgba(0,0,0,0.05);
}

.poster-item:hover .poster-card {
  box-shadow: 0 12px 32px rgba(0,0,0,0.15);
  transform: translateY(-2px);
}

/* 深色模式适配 */
@media (prefers-color-scheme: dark) {
  .poster-card {
    background: #1f2937;
    border-color: rgba(255,255,255,0.1);
  }
}

/* 响应式优化 */
@media (max-width: 640px) {
  .grid {
    gap: 0.5rem;
  }
  
  /* 移动端搜索栏调整 */
  .n-space {
    flex-direction: column !important;
    align-items: stretch !important;
  }
  
  .n-space .n-input {
    min-width: auto !important;
    width: 100% !important;
  }
  
  .n-space .n-select {
    width: 100% !important;
  }
}

/* 海报墙优化 */
.aspect-\[2\/3\] {
  aspect-ratio: 2/3;
}

/* 确保海报在小屏幕上也能保持良好比例 */
@media (max-width: 768px) {
  .aspect-\[2\/3\] {
    aspect-ratio: 2/3;
    min-height: 120px;
  }
}

/* 平滑滚动 */
html {
  scroll-behavior: smooth;
}

/* 自定义滚动条 */
::-webkit-scrollbar {
  width: 8px;
}

::-webkit-scrollbar-track {
  background: rgba(0, 0, 0, 0.1);
  border-radius: 4px;
}

::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.2);
  border-radius: 4px;
}

::-webkit-scrollbar-thumb:hover {
  background: rgba(0, 0, 0, 0.3);
}

/* 深色模式滚动条 */
@media (prefers-color-scheme: dark) {
  ::-webkit-scrollbar-track {
    background: rgba(255, 255, 255, 0.1);
  }
  
  ::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
  }
  
  ::-webkit-scrollbar-thumb:hover {
    background: rgba(255, 255, 255, 0.3);
  }
}
</style> 