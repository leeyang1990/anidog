<template>
  <div class="anime-list-container">
    <PageHeader title="我的追番" subtitle="管理您的追番收藏">
      <template #actions>
        <n-button-group v-if="!isMobile">
          <n-button :type="viewMode === 'grid' ? 'primary' : 'default'" @click="viewMode = 'grid'">
            <template #icon><n-icon><GridOutline /></n-icon></template>
            网格
          </n-button>
          <n-button :type="viewMode === 'list' ? 'primary' : 'default'" @click="viewMode = 'list'">
            <template #icon><n-icon><ListOutline /></n-icon></template>
            列表
          </n-button>
        </n-button-group>
      </template>
    </PageHeader>

    <div class="filter-section">
      <n-space align="center" :wrap="true" :size="isMobile ? 8 : 12">
        <n-input
          v-model:value="searchQuery"
          :placeholder="isMobile ? '搜索...' : '搜索番剧...'"
          clearable
          :style="{ width: isMobile ? '100%' : '220px' }"
          @keydown.enter="fetchAnimeList"
        >
          <template #prefix><n-icon><SearchOutline /></n-icon></template>
        </n-input>

        <n-select
          v-model:value="statusFilter"
          placeholder="状态"
          clearable
          :options="statusOptions"
          :style="{ width: isMobile ? 'calc(50% - 4px)' : '130px' }"
        />

        <n-select
          v-model:value="sortBy"
          placeholder="排序"
          :options="sortOptions"
          :style="{ width: isMobile ? 'calc(50% - 4px)' : '150px' }"
        />

        <n-button type="primary" @click="fetchAnimeList" :loading="loading">
          <template #icon><n-icon><SearchOutline /></n-icon></template>
          <span v-if="!isMobile">搜索</span>
        </n-button>
      </n-space>
    </div>

    <n-spin :show="loading">
      <div v-if="animeList.length === 0 && !loading" class="empty-state">
        <n-empty description="暂无番剧" size="large">
          <template #extra>
            <n-button type="primary" @click="$router.push('/search')">搜索添加</n-button>
          </template>
        </n-empty>
      </div>

      <template v-else>
        <!-- 网格视图 -->
        <div v-if="viewMode === 'grid' || isMobile" :class="gridClasses">
          <naive-anime-card
            v-for="anime in animeList"
            :key="anime.id"
            :anime="anime"
            @click="handleAnimeClick(anime)"
            @delete="handleDelete"
          />
        </div>

        <!-- 列表视图 -->
        <div v-else>
          <n-data-table
            :columns="columns"
            :data="animeList"
            :pagination="false"
            :bordered="false"
            :row-class-name="rowClassName"
            :scroll-x="800"
          />
        </div>
      </template>
    </n-spin>

    <n-pagination
      v-if="totalAnime > pagination.pageSize"
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :item-count="totalAnime"
      :page-sizes="[10, 20, 30, 40]"
      show-size-picker
      class="list-pagination"
      @update:page="handlePageChange"
      @update:page-size="handlePageSizeChange"
    />
  </div>
</template>

<script setup>
import { ref, computed, h, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, NTag, NButton, NSpace, NIcon, NPopconfirm } from 'naive-ui'
import {
  SearchOutline, GridOutline, ListOutline,
  PlayOutline, HeartOutline, TrashOutline
} from '@vicons/ionicons5'
import { get, post, del } from '@/utils/api'
import { useResponsive } from '@/composables/useResponsive'
import PageHeader from '@/components/Common/PageHeader.vue'
import NaiveAnimeCard from '@/components/Anime/NaiveAnimeCard.vue'

const router = useRouter()
const message = useMessage()
const { isMobile, isTablet, isDesktop } = useResponsive()

const loading = ref(false)
const searchQuery = ref('')
const statusFilter = ref(null)
const sortBy = ref('updated_at_desc')
const viewMode = ref('grid')
const animeList = ref([])
const totalAnime = ref(0)

const pagination = ref({
  page: 1,
  pageSize: 20
})

const statusOptions = [
  { label: '连载中', value: 'ongoing' },
  { label: '已完结', value: 'completed' },
  { label: '即将开播', value: 'upcoming' },
  { label: '已弃番', value: 'dropped' }
]

const sortOptions = [
  { label: '最近更新', value: 'updated_at_desc' },
  { label: '首字母 A-Z', value: 'title_asc' },
  { label: '首字母 Z-A', value: 'title_desc' },
  { label: '评分最高', value: 'rating_desc' }
]

const gridClasses = computed(() => {
  if (isMobile.value) return 'anime-grid mobile'
  if (isTablet.value) return 'anime-grid tablet'
  return 'anime-grid desktop'
})

const columns = computed(() => [
  {
    title: '封面',
    key: 'cover',
    width: 80,
    render: (row) => h('img', {
      src: row.cover_image || '',
      alt: row.title,
      style: 'width: 48px; height: 64px; object-fit: cover; border-radius: 4px; background: rgba(var(--color-primary), 0.08);'
    })
  },
  {
    title: '标题',
    key: 'title',
    render: (row) => h('div', { style: 'display:flex; flex-direction:column' }, [
      h('span', { style: 'font-weight:600; color:var(--text-primary)' }, row.title),
      row.original_title ? h('span', { style: 'font-size:12px; color:var(--text-tertiary)' }, row.original_title) : null
    ])
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render: (row) => {
      const map = {
        ongoing: { type: 'success', text: '连载中' },
        completed: { type: 'info', text: '已完结' },
        upcoming: { type: 'warning', text: '即将开播' },
        dropped: { type: 'error', text: '已弃番' }
      }
      const s = map[row.status] || { type: 'default', text: '未知' }
      return h(NTag, { type: s.type, size: 'small', bordered: false }, { default: () => s.text })
    }
  },
  {
    title: '集数',
    key: 'episodes',
    width: 80,
    render: (row) => row.episode_count ? `${row.episode_count} 集` : '—'
  },
  {
    title: '评分',
    key: 'rating',
    width: 70,
    render: (row) => row.bangumi_rating ? `${row.bangumi_rating}` : '—'
  },
  {
    title: '操作',
    key: 'actions',
    width: 160,
    render: (row) => h(NSpace, { size: 'small' }, [
      h(NButton, {
        tertiary: true, size: 'small', type: 'primary',
        onClick: () => handleAnimeClick(row)
      }, { default: () => '详情' }),
      h(NButton, {
        tertiary: true, size: 'small',
        type: row.is_subscribed ? 'success' : 'info',
        onClick: () => row.is_subscribed ? handleUnsubscribe(row) : handleSubscribe(row)
      }, { default: () => row.is_subscribed ? '已订阅' : '订阅' }),
      h(NPopconfirm, {
        onPositiveClick: () => handleDelete(row),
        negativeText: '取消', positiveText: '删除'
      }, {
        trigger: () => h(NButton, {
          tertiary: true, size: 'small', type: 'error'
        }, { default: () => '删除' }),
        default: () => '确定要删除这个番剧吗？'
      })
    ])
  }
])

const rowClassName = (row) => row.is_subscribed ? 'subscribed-row' : ''

async function fetchAnimeList() {
  loading.value = true
  try {
    const params = {
      page: pagination.value.page,
      per_page: pagination.value.pageSize,
      subscribed: true
    }
    if (statusFilter.value) params.status = statusFilter.value

    const data = await get('/anime', { params })
    animeList.value = data.items || []
    totalAnime.value = data.total || 0
  } catch (e) {
    message.error('获取番剧列表失败')
    animeList.value = []
    totalAnime.value = 0
  } finally {
    loading.value = false
  }
}

function handlePageChange(page) {
  pagination.value.page = page
  fetchAnimeList()
}

function handlePageSizeChange(pageSize) {
  pagination.value.pageSize = pageSize
  pagination.value.page = 1
  fetchAnimeList()
}

function handleAnimeClick(anime) {
  router.push(`/anime/${anime.id}`)
}

async function handleSubscribe(anime) {
  try {
    await post(`/anime/${anime.id}/subscribe`)
    anime.is_subscribed = true
    message.success(`已订阅《${anime.title}》`)
  } catch { message.error('订阅失败') }
}

async function handleUnsubscribe(anime) {
  try {
    await post(`/anime/${anime.id}/unsubscribe`)
    anime.is_subscribed = false
    message.success(`已取消订阅《${anime.title}》`)
  } catch { message.error('取消订阅失败') }
}

async function handleFavorite(anime) {
  anime.is_favorite = !anime.is_favorite
  message.success(anime.is_favorite ? `已收藏《${anime.title}》` : `已取消收藏`)
}

async function handleDelete(anime) {
  try {
    await del(`/anime/${anime.id}`)
    animeList.value = animeList.value.filter(a => a.id !== anime.id)
    totalAnime.value--
    message.success(`已删除《${anime.title}》`)
  } catch { message.error('删除失败') }
}

function handleDownload(anime) { message.info(`正在下载《${anime.title}》`) }
function handleShare() { message.info('分享链接已复制到剪贴板') }
function handleReport() { message.info('已收到您的问题报告') }

watch([statusFilter], () => {
  pagination.value.page = 1
  fetchAnimeList()
})

onMounted(fetchAnimeList)
</script>

<style scoped>
.anime-list-container { }

.filter-section {
  background: var(--card-bg);
  border-radius: var(--radius-md);
  padding: 16px 20px;
  border: 1px solid rgba(var(--color-border), var(--color-border-opacity, 0.06));
  margin-bottom: 20px;
}

.anime-grid {
  display: grid; width: 100%;
}
.anime-grid.mobile {
  grid-template-columns: repeat(2, 1fr); gap: 12px;
}
.anime-grid.tablet {
  grid-template-columns: repeat(3, 1fr); gap: 16px;
}
.anime-grid.desktop {
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 20px;
}

.empty-state { padding: 3rem 0; text-align: center; }

.list-pagination {
  margin-top: 24px; display: flex; justify-content: center;
}

:deep(.subscribed-row) {
  background: rgba(var(--color-primary), 0.03);
}

:deep(.n-data-table) {
  background: var(--card-bg);
  border-radius: var(--radius-md);
  overflow: hidden;
}

:deep(.n-data-table th) {
  background: rgba(var(--color-primary), 0.06);
  color: var(--text-primary);
  font-weight: 600;
  padding: 14px 16px;
}

:deep(.n-data-table td) {
  padding: 12px 16px;
}

:deep(.n-data-table tr:hover td) {
  background: rgba(var(--color-primary), 0.03);
}

@media (max-width: 768px) {
  .filter-section { padding: 12px 16px; }
  .empty-state { padding: 2rem 0; }
  :deep(.n-pagination) { justify-content: center; }
}
</style>
