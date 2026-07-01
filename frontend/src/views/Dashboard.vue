<template>
  <div>
    <AcPageHeader title="🏝️ 仪表盘" subtitle="番剧下载管理概览">
      <template #actions>
        <AcButton variant="outline" size="md" :loading="loading" @click="fetchDashboardData">
          <template #icon><RefreshOutline class="size-4" /></template>
          刷新
        </AcButton>
      </template>
    </AcPageHeader>

    <!-- Loading -->
    <div v-if="loading && !stats.animeCount" class="flex items-center justify-center py-20">
      <AcSpinner :size="48" />
    </div>

    <!-- Error -->
    <div v-else-if="error" class="flex flex-col items-center justify-center py-20 text-center">
      <p class="text-base text-muted-foreground mb-4">{{ error }}</p>
      <AcButton variant="primary" @click="fetchDashboardData">重试</AcButton>
    </div>

    <template v-else>
      <!-- Stats: 4 cards -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <AcCard v-for="stat in statCards" :key="stat.label" hoverable padding="md" rounded="2xl">
          <div class="flex items-center gap-2 mb-3">
            <div class="size-9 rounded-2xl flex items-center justify-center" :class="stat.iconBg">
              <component :is="stat.icon" class="size-5" :class="stat.iconColor" />
            </div>
            <span class="text-sm font-bold text-muted-foreground">{{ stat.label }}</span>
          </div>
          <div class="text-3xl font-bold tracking-tight font-num text-foreground">{{ stat.value }}</div>
          <div v-if="stat.sub" class="text-xs text-muted-foreground mt-1">{{ stat.sub }}</div>
        </AcCard>
      </div>

      <!-- Charts -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4 mb-6">
        <AcCard padding="lg" rounded="2xl" class="lg:col-span-2">
          <h3 class="text-base font-bold tracking-tight mb-4 text-foreground">📈 最近 7 天下载</h3>
          <div class="h-64">
            <Line v-if="chartData" :data="chartData" :options="chartOptions" />
          </div>
        </AcCard>
        <AcCard padding="lg" rounded="2xl">
          <h3 class="text-base font-bold tracking-tight mb-4 text-foreground">🍩 状态分布</h3>
          <div class="h-64 flex items-center justify-center">
            <Doughnut v-if="doughnutData && hasAnyDownload" :data="doughnutData" :options="doughnutOptions" />
            <AcEmpty v-else title="暂无下载" description="还没有任何下载记录哦~" />
          </div>
        </AcCard>
      </div>

      <!-- Recent downloads -->
      <AcCard padding="none" rounded="2xl">
        <div class="px-6 pt-5 pb-3 flex items-center justify-between">
          <h3 class="text-base font-bold tracking-tight text-foreground">🌱 最近下载</h3>
          <router-link to="/downloads" class="text-xs text-ac-grass-dark hover:underline font-bold">
            查看全部 →
          </router-link>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full">
            <thead>
              <tr class="border-y-2 border-dashed border-ac-sand text-left text-xs text-muted-foreground bg-ac-sand/30">
                <th class="py-3 pl-6 font-bold">名称</th>
                <th class="py-3 font-bold">来源</th>
                <th class="py-3 font-bold">大小</th>
                <th class="py-3 font-bold">状态</th>
                <th class="py-3 pr-6 font-bold">更新时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="dl in recentDownloads" :key="dl.id" class="border-b-2 border-dashed border-ac-sand last:border-0 hover:bg-ac-cream/50 transition-colors">
                <td class="py-3 pl-6 text-sm max-w-[360px]">
                  <div class="truncate" :title="dl.name">{{ dl.name }}</div>
                </td>
                <td class="py-3 text-sm">
                  <AcTag variant="wood">{{ sourceShort(dl) }}</AcTag>
                </td>
                <td class="py-3 text-sm text-muted-foreground whitespace-nowrap font-num">{{ formatSize(dl.total_bytes || dl.downloaded_bytes) }}</td>
                <td class="py-3 text-sm">
                  <AcTag :variant="statusVariant(dl.status)">{{ statusText(dl.status) }}</AcTag>
                </td>
                <td class="py-3 pr-6 text-xs text-muted-foreground whitespace-nowrap font-num">{{ formatTime(dl.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
          <AcEmpty v-if="!recentDownloads.length" title="还没有下载记录" description="快去番剧库挑一部追起来吧 🐾" class="py-8" />
        </div>
      </AcCard>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useToast } from '../composables/useToast'
import {
  FilmOutline, LogoRss, DownloadOutline, CheckmarkCircleOutline, RefreshOutline,
} from '@vicons/ionicons5'
import { Line, Doughnut } from 'vue-chartjs'
import {
  Chart as ChartJS, CategoryScale, LinearScale, PointElement,
  LineElement, Title, Tooltip, Legend, ArcElement,
} from 'chart.js'
import dayjs from 'dayjs'
import { get } from '@/utils/api'
import { AcPageHeader, AcCard, AcButton, AcSpinner, AcTag, AcEmpty } from '../components/ac'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, ArcElement)

const CHART = {
  grass: '#7CB342',
  sun: '#FFB74D',
  sky: '#81D4FA',
  heart: '#E57373',
  leaf: '#66BB6A',
  wood: '#8D6E63',
}

const loading = ref(false)
const error = ref(null)
const stats = ref({ animeCount: 0, rssFeedCount: 0, downloading: 0, completed: 0, pending: 0, failed: 0, paused: 0, total: 0 })
const recentDownloads = ref([])
const toast = useToast()

const chartData = ref(null)
const doughnutData = ref(null)

const hasAnyDownload = computed(() => stats.value.total > 0)

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false } },
  scales: { y: { beginAtZero: true, ticks: { stepSize: 1 } } },
}
const doughnutOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { position: 'bottom' } },
}

const statCards = computed(() => [
  { label: '番剧数量',     value: stats.value.animeCount,   icon: FilmOutline,             iconBg: 'bg-ac-grass-light/40', iconColor: 'text-ac-grass-dark', sub: '' },
  { label: 'RSS 订阅',     value: stats.value.rssFeedCount, icon: LogoRss,                 iconBg: 'bg-ac-sun/30',         iconColor: 'text-ac-sun-dark',   sub: '' },
  { label: '正在下载',     value: stats.value.downloading,  icon: DownloadOutline,         iconBg: 'bg-ac-sky/40',         iconColor: 'text-ac-sky-dark',   sub: stats.value.pending ? `等待 ${stats.value.pending}` : '' },
  { label: '已完成',       value: stats.value.completed,    icon: CheckmarkCircleOutline,  iconBg: 'bg-ac-leaf/30',        iconColor: 'text-ac-leaf-dark',  sub: stats.value.failed ? `失败 ${stats.value.failed}` : '' },
])

function statusVariant(status) {
  return { completed: 'leaf', downloading: 'sky', pending: 'sun', paused: 'wood', failed: 'heart' }[status] || 'default'
}
function statusText(status) {
  return { completed: '已完成', downloading: '下载中', pending: '等待中', paused: '已暂停', failed: '已失败' }[status] || status
}
function sourceShort(dl) {
  if (dl.download_type === 'stream') return 'Stream'
  if (dl.download_type === 'torrent') return 'BT'
  return dl.source || '—'
}
function formatTime(t) { return t ? dayjs(t).format('MM-DD HH:mm') : '—' }
function formatSize(b) {
  if (!b || b <= 0) return '—'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(b) / Math.log(1024)), u.length - 1)
  return (b / Math.pow(1024, i)).toFixed(1) + ' ' + u[i]
}

async function fetchDashboardData() {
  loading.value = true
  error.value = null
  try {
    const data = await get('/dashboard')
    const rawStats = data.stats || {}
    const ds = rawStats.download_stats || {}
    stats.value = {
      animeCount: rawStats.anime_count || 0,
      rssFeedCount: rawStats.rss_feed_count || 0,
      total: ds.total || 0,
      pending: ds.pending || 0,
      downloading: ds.downloading || 0,
      completed: ds.completed || 0,
      failed: ds.failed || 0,
      paused: ds.paused || 0,
    }
    const points = Array.isArray(data.downloadStats) ? data.downloadStats : []
    chartData.value = {
      labels: points.map(p => dayjs(p.date).format('MM-DD')),
      datasets: [{
        label: '下载数量',
        data: points.map(p => p.count || 0),
        fill: true,
        borderColor: CHART.grass,
        backgroundColor: CHART.grass + '33',
        tension: 0.4,
        pointRadius: 4,
        pointBackgroundColor: CHART.grass,
        pointBorderColor: '#fff',
        pointBorderWidth: 2,
      }],
    }
    doughnutData.value = {
      labels: ['等待中', '下载中', '已完成', '已暂停', '已失败'],
      datasets: [{
        data: [stats.value.pending, stats.value.downloading, stats.value.completed, stats.value.paused, stats.value.failed],
        backgroundColor: [CHART.sun, CHART.sky, CHART.leaf, CHART.wood, CHART.heart],
        borderWidth: 4,
        borderColor: 'hsl(var(--card))',
      }],
    }
    recentDownloads.value = Array.isArray(data.recentDownloads) ? data.recentDownloads : []
  } catch (err) {
    const msg = err.message || '加载数据失败'
    error.value = msg
    toast.error(msg)
  } finally {
    loading.value = false
  }
}

onMounted(fetchDashboardData)
</script>
