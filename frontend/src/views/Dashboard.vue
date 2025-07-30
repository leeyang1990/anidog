<template>
  <div class="p-4">
    <n-grid :x-gap="12" :y-gap="12" :cols="24">
      <!-- 统计卡片 -->
      <n-grid-item span="6">
        <n-card>
          <template #header>
            <div class="flex items-center">
              <n-icon size="20" class="mr-2">
                <film-icon />
              </n-icon>
              动画数量
            </div>
          </template>
          <div class="text-2xl font-bold">{{ stats.animeCount || 0 }}</div>
        </n-card>
      </n-grid-item>

      <n-grid-item span="6">
        <n-card>
          <template #header>
            <div class="flex items-center">
              <n-icon size="20" class="mr-2">
                <rss-icon />
              </n-icon>
              RSS 订阅
            </div>
          </template>
          <div class="text-2xl font-bold">{{ stats.rssCount || 0 }}</div>
        </n-card>
      </n-grid-item>

      <n-grid-item span="6">
        <n-card>
          <template #header>
            <div class="flex items-center">
              <n-icon size="20" class="mr-2">
                <arrow-down-tray-icon />
              </n-icon>
              正在下载
            </div>
          </template>
          <div class="text-2xl font-bold">{{ stats.downloadingCount || 0 }}</div>
        </n-card>
      </n-grid-item>

      <n-grid-item span="6">
        <n-card>
          <template #header>
            <div class="flex items-center">
              <n-icon size="20" class="mr-2">
                <check-circle-icon />
              </n-icon>
              已完成下载
            </div>
          </template>
          <div class="text-2xl font-bold">{{ stats.completedCount || 0 }}</div>
        </n-card>
      </n-grid-item>

      <!-- 图表 -->
      <n-grid-item span="12">
        <n-card title="最近7天下载统计">
          <div v-if="chartDataReady">
            <line-chart :data="downloadStats" :options="lineChartOptions" />
          </div>
          <div v-else class="h-64 flex items-center justify-center">
            <n-spin />
          </div>
        </n-card>
      </n-grid-item>

      <n-grid-item span="12">
        <n-card title="下载状态分布">
          <div v-if="chartDataReady">
            <pie-chart :data="downloadStatusStats" :options="pieChartOptions" />
          </div>
          <div v-else class="h-64 flex items-center justify-center">
            <n-spin />
          </div>
        </n-card>
      </n-grid-item>

      <!-- 最近下载列表 -->
      <n-grid-item span="24">
        <n-card title="最近下载">
          <n-data-table
            :columns="columns"
            :data="recentDownloads"
            :loading="loading"
            :pagination="{ pageSize: 5 }"
          />
        </n-card>
      </n-grid-item>
    </n-grid>
  </div>
  <div class="mt-8 flex justify-center">
    <n-card title="体验新界面" class="max-w-md">
      <div class="text-center">
        <p class="mb-4">我们推出了全新的界面设计，更美观、更流畅的用户体验！</p>
        <n-button type="primary" @click="router.push('/naive')">
          立即体验新界面
        </n-button>
      </div>
    </n-card>
  </div>
</template>

<script setup>
import { ref, onMounted, h, computed } from 'vue'
import { NIcon, useMessage, NSpin } from 'naive-ui'
import {
  FilmIcon as FilmIconOutline,
  RssIcon as RssIconOutline,
  ArrowDownTrayIcon as ArrowDownTrayIconOutline,
  CheckCircleIcon as CheckCircleIconOutline
} from '@heroicons/vue/24/outline'
// 重命名导入的组件
const FilmIcon = FilmIconOutline
const RssIcon = RssIconOutline
const ArrowDownTrayIcon = ArrowDownTrayIconOutline
const CheckCircleIcon = CheckCircleIconOutline

import { Line as LineChart, Pie as PieChart } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  ArcElement
} from 'chart.js'
import { api } from '@/utils/api'

// 注册 Chart.js 组件
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  ArcElement
)

const loading = ref(false)
const chartDataReady = ref(false)
const stats = ref({})

// 图表配置
const lineChartOptions = {
  responsive: true,
  maintainAspectRatio: false
}

const pieChartOptions = {
  responsive: true,
  maintainAspectRatio: false
}

// 图表数据
const downloadStats = ref({
  labels: ['周一', '周二', '周三', '周四', '周五', '周六', '周日'],
  datasets: [{
    label: '下载数量',
    data: [0, 0, 0, 0, 0, 0, 0],
    fill: false,
    borderColor: '#3B82F6',
    tension: 0.1
  }]
})

const downloadStatusStats = ref({
  labels: ['等待中', '下载中', '已完成', '已失败'],
  datasets: [{
    data: [0, 0, 0, 0],
    backgroundColor: ['#FCD34D', '#60A5FA', '#34D399', '#F87171']
  }]
})

const recentDownloads = ref([])

const columns = [
  {
    title: '文件名',
    key: 'filename',
    ellipsis: true
  },
  {
    title: '大小',
    key: 'size',
    width: 100
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render(row) {
      const statusMap = {
        waiting: '等待中',
        downloading: '下载中',
        completed: '已完成',
        failed: '已失败'
      }
      return statusMap[row.status] || row.status
    }
  },
  {
    title: '更新时间',
    key: 'updated_at',
    width: 200
  }
]

const message = useMessage()

async function fetchDashboardData() {
  loading.value = true
  chartDataReady.value = false
  
  try {
    const data = await api.get('/dashboard')
    
    // 更新统计数据
    stats.value = data.stats || {}
    
    // 更新下载统计图表
    if (data.downloadStats && Array.isArray(data.downloadStats.dates)) {
      downloadStats.value = {
        labels: data.downloadStats.dates,
        datasets: [{
          label: '下载数量',
          data: data.downloadStats.counts || [],
          fill: false,
          borderColor: '#3B82F6',
          tension: 0.1
        }]
      }
    }
    
    // 更新状态分布图表
    if (data.stats) {
      downloadStatusStats.value = {
        labels: ['等待中', '下载中', '已完成', '已失败'],
        datasets: [{
          data: [
            data.stats.waitingCount || 0,
            data.stats.downloadingCount || 0,
            data.stats.completedCount || 0,
            data.stats.failedCount || 0
          ],
          backgroundColor: ['#FCD34D', '#60A5FA', '#34D399', '#F87171']
        }]
      }
    }
    
    // 更新最近下载列表
    recentDownloads.value = data.recentDownloads || []
    
    // 标记图表数据已准备好
    chartDataReady.value = true
  } catch (error) {
    message.error(error.message || '加载数据失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchDashboardData()
})
</script> 