<template>
  <div class="p-4">
    <n-card>
      <template #header>
        <div class="flex items-center justify-between">
          <h2 class="text-xl font-bold">{{ anime.name || '加载中...' }}</h2>
          <n-button @click="$router.back()">返回</n-button>
        </div>
      </template>

      <n-spin :show="loading">
        <n-space vertical>
          <!-- 基本信息 -->
          <n-descriptions label-placement="left" bordered>
            <n-descriptions-item label="状态">
              {{ anime.status }}
            </n-descriptions-item>
            <n-descriptions-item label="集数">
              {{ anime.episode }}
            </n-descriptions-item>
            <n-descriptions-item label="更新时间">
              {{ anime.updated_at }}
            </n-descriptions-item>
          </n-descriptions>

          <!-- 下载记录 -->
          <div>
            <h3 class="text-lg font-bold mb-4">下载记录</h3>
            <n-data-table
              :columns="downloadColumns"
              :data="downloads"
              :loading="loadingDownloads"
              :pagination="pagination"
              @update:page="handlePageChange"
            />
          </div>

          <!-- RSS订阅 -->
          <div>
            <h3 class="text-lg font-bold mb-4">RSS订阅</h3>
            <n-space>
              <n-input v-model:value="anime.rss_url" readonly placeholder="暂无RSS订阅" />
              <n-button
                v-if="anime.rss_url"
                @click="() => window.open(anime.rss_url, '_blank')"
              >
                访问RSS
              </n-button>
            </n-space>
          </div>
        </n-space>
      </n-spin>
    </n-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { get } from '@/utils/api'
import {
  NCard,
  NButton,
  NSpace,
  NSpin,
  NDescriptions,
  NDescriptionsItem,
  NDataTable,
  NInput
} from 'naive-ui'

const route = useRoute()
const animeId = route.params.id

const loading = ref(true)
const loadingDownloads = ref(false)
const anime = ref({})
const downloads = ref([])

const pagination = ref({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [10, 20, 30, 40],
  onChange: (page) => {
    pagination.value.page = page
  },
  onUpdatePageSize: (pageSize) => {
    pagination.value.pageSize = pageSize
    pagination.value.page = 1
  }
})

const downloadColumns = [
  {
    title: '文件名',
    key: 'filename',
    ellipsis: true
  },
  {
    title: '大小',
    key: 'size',
  },
  {
    title: '状态',
    key: 'status',
  },
  {
    title: '下载时间',
    key: 'created_at',
  }
]

async function fetchAnimeDetail() {
  loading.value = true
  try {
    const data = await get(`/anime/${animeId}`)
    anime.value = data
  } catch (error) {
    console.error('获取动画详情失败:', error)
  } finally {
    loading.value = false
  }
}

async function fetchDownloads(page = 1) {
  loadingDownloads.value = true
  try {
    const data = await get(`/anime/${animeId}/downloads?page=${page}&per_page=${pagination.value.pageSize}`)
    downloads.value = data.items
    pagination.value.itemCount = data.total
  } catch (error) {
    console.error('获取下载记录失败:', error)
  } finally {
    loadingDownloads.value = false
  }
}

function handlePageChange(page) {
  fetchDownloads(page)
}

onMounted(() => {
  fetchAnimeDetail()
  fetchDownloads()
})
</script> 