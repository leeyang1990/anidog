<template>
  <div>
    <PageHeader title="仪表盘" subtitle="番剧下载管理概览" />

    <!-- Loading -->
    <div v-if="loading" class="flex items-center justify-center py-20">
      <n-spin size="large" />
    </div>

    <!-- Error -->
    <div v-else-if="error" class="flex flex-col items-center justify-center py-20 text-center">
      <p class="text-lg text-muted-foreground mb-4">{{ error }}</p>
      <button class="h-10 px-6 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors" @click="fetchDashboardData">重试</button>
    </div>

    <template v-else>
      <!-- Stats: 4 cards grid -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <div v-for="stat in statCards" :key="stat.label" class="bg-card text-card-foreground rounded-lg border p-6">
          <div class="flex items-center gap-2 mb-2">
            <div class="h-8 w-8 rounded-md bg-primary/10 flex items-center justify-center">
              <n-icon size="16" class="text-primary"><component :is="stat.icon" /></n-icon>
            </div>
            <span class="text-sm font-medium text-muted-foreground">{{ stat.label }}</span>
          </div>
          <div class="text-2xl font-bold tracking-tight">{{ stat.value }}</div>
        </div>
      </div>

      <!-- Charts: 2/3 + 1/3 bento grid -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4 mb-6">
        <div class="lg:col-span-2 bg-card text-card-foreground rounded-lg border p-6">
          <h3 class="text-lg font-semibold tracking-tight mb-4">最近7天下载统计</h3>
          <div class="h-64">
            <Line v-if="chartData" :data="chartData" :options="chartOptions" />
          </div>
        </div>
        <div class="bg-card text-card-foreground rounded-lg border p-6">
          <h3 class="text-lg font-semibold tracking-tight mb-4">下载状态分布</h3>
          <div class="h-64 flex items-center justify-center">
            <Doughnut v-if="doughnutData" :data="doughnutData" :options="doughnutOptions" />
          </div>
        </div>
      </div>

      <!-- Recent downloads table -->
      <div class="bg-card text-card-foreground rounded-lg border">
        <div class="p-6 pb-0">
          <h3 class="text-lg font-semibold tracking-tight">最近下载</h3>
        </div>
        <div class="mt-4">
          <table class="w-full">
            <thead>
              <tr class="border-b text-left text-sm text-muted-foreground">
                <th class="pb-3 pl-6 font-medium">文件名</th>
                <th class="pb-3 font-medium">大小</th>
                <th class="pb-3 font-medium">状态</th>
                <th class="pb-3 pr-6 font-medium">更新时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="dl in recentDownloads" :key="dl.id || dl.filename" class="border-b last:border-0 hover:bg-muted/50 transition-colors">
                <td class="py-3 pl-6 text-sm">{{ dl.name || dl.filename }}</td>
                <td class="py-3 text-sm text-muted-foreground">{{ dl.size }}</td>
                <td class="py-3 text-sm">
                  <span class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
                    :class="statusBadgeClass(dl.status)">
                    {{ statusText(dl.status) }}
                  </span>
                </td>
                <td class="py-3 pr-6 text-sm text-muted-foreground">{{ formatTime(dl.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
          <div v-if="!recentDownloads.length" class="py-8 text-center text-sm text-muted-foreground">暂无下载记录</div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useMessage, NIcon, NSpin } from 'naive-ui'
import {
  FilmOutline,
  LogoRss,
  DownloadOutline,
  CheckmarkCircleOutline
} from '@vicons/ionicons5'

import { Line, Doughnut } from 'vue-chartjs'
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
import dayjs from 'dayjs'
import { get } from '@/utils/api'
import PageHeader from '@/components/Common/PageHeader.vue'

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

const CHART_COLORS = {
  primary: '#667eea',
  chart2: '#10b981',
  chart3: '#f59e0b',
  chart4: '#f43f5e',
  chart5: '#8b5cf6'
}

const loading = ref(false)
const error = ref(null)
const stats = ref({})
const recentDownloads = ref([])

const message = useMessage()

const chartData = ref(null)
const doughnutData = ref(null)

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      display: false
    }
  },
  scales: {
    y: {
      beginAtZero: true,
      ticks: {
        stepSize: 1
      }
    }
  }
}

const doughnutOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      position: 'bottom'
    }
  }
}

const statCards = computed(() => [
  {
    label: '动画数量',
    value: stats.value.animeCount || 0,
    icon: FilmOutline
  },
  {
    label: 'RSS 订阅',
    value: stats.value.rssCount || 0,
    icon: LogoRss
  },
  {
    label: '正在下载',
    value: stats.value.downloadingCount || 0,
    icon: DownloadOutline
  },
  {
    label: '已完成下载',
    value: stats.value.completedCount || 0,
    icon: CheckmarkCircleOutline
  }
])

function statusBadgeClass(status) {
  const map = {
    completed: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
    downloading: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
    waiting: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
    failed: 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400'
  }
  return map[status] || 'bg-secondary text-secondary-foreground'
}

function statusText(status) {
  const map = {
    completed: '已完成',
    downloading: '下载中',
    waiting: '等待中',
    failed: '已失败'
  }
  return map[status] || status
}

function formatTime(time) {
  if (!time) return '—'
  return dayjs(time).format('YYYY-MM-DD HH:mm')
}

async function fetchDashboardData() {
  loading.value = true
  error.value = null

  try {
    const data = await get('/dashboard')

    stats.value = data.stats || {}

    if (data.downloadStats && Array.isArray(data.downloadStats.dates)) {
      chartData.value = {
        labels: data.downloadStats.dates,
        datasets: [{
          label: '下载数量',
          data: data.downloadStats.counts || [],
          fill: false,
          borderColor: CHART_COLORS.primary,
          backgroundColor: CHART_COLORS.primary + '33',
          tension: 0.1
        }]
      }
    } else {
      chartData.value = {
        labels: ['周一', '周二', '周三', '周四', '周五', '周六', '周日'],
        datasets: [{
          label: '下载数量',
          data: [0, 0, 0, 0, 0, 0, 0],
          fill: false,
          borderColor: CHART_COLORS.primary,
          backgroundColor: CHART_COLORS.primary + '33',
          tension: 0.1
        }]
      }
    }

    if (data.stats) {
      doughnutData.value = {
        labels: ['等待中', '下载中', '已完成', '已失败'],
        datasets: [{
          data: [
            data.stats.waitingCount || 0,
            data.stats.downloadingCount || 0,
            data.stats.completedCount || 0,
            data.stats.failedCount || 0
          ],
          backgroundColor: [
            CHART_COLORS.chart3,
            CHART_COLORS.primary,
            CHART_COLORS.chart2,
            CHART_COLORS.chart4
          ]
        }]
      }
    }

    recentDownloads.value = data.recentDownloads || []
  } catch (err) {
    const errorMsg = err.message || '加载数据失败'
    error.value = errorMsg
    message.error(errorMsg)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchDashboardData()
})
</script>
