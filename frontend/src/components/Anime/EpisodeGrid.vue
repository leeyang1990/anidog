<template>
  <div class="space-y-3">
    <!-- 头部：进度 + 操作 -->
    <div class="flex items-center justify-between flex-wrap gap-2">
      <div class="text-sm">
        <span class="font-medium">剧集进度</span>
        <span class="text-muted-foreground ml-2">
          {{ completedCount }} / {{ effectiveCount }} 集
          <span v-if="!episodeCount && effectiveCount > 0" class="text-amber-500 text-xs">（集数未知，按已下载动态显示）</span>
          <span v-if="downloadingCount > 0" class="text-primary">· 下载中 {{ downloadingCount }}</span>
          <span v-if="upcomingCount > 0" class="text-sky-500">· 待发布 {{ upcomingCount }}</span>
          <span v-if="noResourceCount > 0" class="text-amber-500">· 未命中 {{ noResourceCount }}</span>
        </span>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs text-muted-foreground">自动模式 <span class="text-emerald-600 font-medium">ON</span></span>
        <button @click="openManualSearch(0)"
          class="h-7 px-3 rounded-md border border-input bg-background hover:bg-accent text-xs font-medium transition-colors inline-flex items-center gap-1">
          🔍 手动选种
        </button>
        <button @click="triggerSearch" :disabled="triggering"
          class="h-7 px-3 rounded-md border border-input bg-background hover:bg-accent text-xs font-medium transition-colors disabled:opacity-50">
          {{ triggering ? '搜索中...' : '立即全量搜索' }}
        </button>
      </div>
    </div>

    <!-- 格子 -->
    <div v-if="effectiveCount > 0" class="grid grid-cols-5 sm:grid-cols-6 md:grid-cols-8 lg:grid-cols-10 gap-2">
      <button v-for="ep in episodeStatusList" :key="ep.episode_number"
        class="aspect-square rounded-md border flex flex-col items-center justify-center text-xs transition-colors relative"
        :class="cellClass(ep)"
        :title="tooltipText(ep)"
        @click="openDetail(ep.episode_number)">
        <span class="font-mono font-medium">{{ String(ep.episode_number).padStart(2, '0') }}</span>
        <!-- 状态图标 -->
        <n-icon v-if="ep.status === 'completed'" size="12" class="absolute top-0.5 right-0.5 text-emerald-600">
          <CheckmarkCircleOutline />
        </n-icon>
        <n-icon v-else-if="ep.status === 'downloading' || ep.status === 'pending'" size="12" class="absolute top-0.5 right-0.5 text-primary animate-pulse">
          <CloudDownloadOutline />
        </n-icon>
        <n-icon v-else-if="ep.status === 'no_resource'" size="12" class="absolute top-0.5 right-0.5 text-amber-500">
          <AlertCircleOutline />
        </n-icon>
        <n-icon v-else-if="ep.status === 'upcoming'" size="12" class="absolute top-0.5 right-0.5 text-sky-500">
          <TimeOutline />
        </n-icon>
        <!-- 来源 badge -->
        <span v-if="ep.status === 'completed'" class="absolute bottom-0.5 right-0.5 text-[9px] font-bold opacity-60">
          {{ sourceBadge(ep) }}
        </span>
        <!-- 待发布日期 -->
        <span v-else-if="ep.status === 'upcoming' && ep.air_date"
          class="absolute bottom-0.5 left-1/2 -translate-x-1/2 text-[9px] text-sky-600 dark:text-sky-400 whitespace-nowrap">
          {{ shortDate(ep.air_date) }}
        </span>
      </button>
    </div>

    <div v-else class="text-sm text-muted-foreground py-6 text-center">
      还没有集数信息
    </div>

    <!-- 详情抽屉 -->
    <EpisodeDetailDrawer
      v-model:show="showDrawer"
      :anime-id="animeId"
      :anime-title="animeTitle"
      :episode="selectedEp"
      :downloads="downloadsByEp[selectedEp] || []"
      :diagnosis="diagnosisByEp[selectedEp]"
      :ep-meta="epMetaByEp[selectedEp]"
      @refresh="refresh"
      @manual-search="openManualSearch"
    />

    <!-- 手动选种对话框 -->
    <ManualSearchDialog
      v-model:show="showManualSearch"
      :anime-id="animeId"
      :anime-title="animeTitle"
      :episode="manualSearchEp"
      @downloaded="refresh"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useMessage, NIcon } from 'naive-ui'
import {
  CheckmarkCircleOutline, CloudDownloadOutline, AlertCircleOutline, TimeOutline,
} from '@vicons/ionicons5'
import { get, post } from '@/utils/api'
import EpisodeDetailDrawer from './EpisodeDetailDrawer.vue'
import ManualSearchDialog from './ManualSearchDialog.vue'

const props = defineProps({
  animeId: { type: Number, required: true },
  animeTitle: { type: String, default: '' },
  episodeCount: { type: Number, default: 0 },
})

const message = useMessage()

const downloads = ref([]) // 该 anime 的所有 download 记录（详情抽屉要用）
const diagnosis = ref([]) // 诊断数据 { episode_number, sources: {bt: {...}, ...} }
const episodeStatus = ref([]) // /anime/:id/episode-status 返回的统一数组
const triggering = ref(false)

const showDrawer = ref(false)
const selectedEp = ref(0)

const showManualSearch = ref(false)
const manualSearchEp = ref(0)

let pollTimer = null

const downloadsByEp = computed(() => {
  const map = {}
  for (const d of downloads.value) {
    if (!d.episode_number) continue
    if (!map[d.episode_number]) map[d.episode_number] = []
    map[d.episode_number].push(d)
  }
  return map
})

const diagnosisByEp = computed(() => {
  const map = {}
  for (const e of diagnosis.value) {
    map[e.episode_number] = e
  }
  return map
})

const epMetaByEp = computed(() => {
  const map = {}
  for (const e of episodeStatus.value) {
    map[e.episode_number] = e
  }
  return map
})

const episodeStatusList = computed(() => episodeStatus.value)

const effectiveCount = computed(() => episodeStatus.value.length)

const completedCount = computed(() => episodeStatus.value.filter(e => e.status === 'completed').length)
const downloadingCount = computed(() => episodeStatus.value.filter(e => e.status === 'downloading' || e.status === 'pending').length)
const upcomingCount = computed(() => episodeStatus.value.filter(e => e.status === 'upcoming').length)
const noResourceCount = computed(() => episodeStatus.value.filter(e => e.status === 'no_resource').length)

function cellClass(ep) {
  switch (ep.status) {
    case 'completed':
      return 'bg-emerald-500/10 border-emerald-500/30 text-emerald-700 dark:text-emerald-400 hover:bg-emerald-500/20 cursor-pointer'
    case 'downloading':
    case 'pending':
      return 'bg-primary/10 border-primary/30 text-primary hover:bg-primary/20 cursor-pointer'
    case 'no_resource':
      return 'bg-amber-500/10 border-amber-500/30 text-amber-700 dark:text-amber-400 hover:bg-amber-500/20 cursor-pointer'
    case 'upcoming':
      return 'bg-sky-500/5 border-sky-500/30 text-sky-700 dark:text-sky-400 hover:bg-sky-500/10 cursor-pointer'
    default:
      return 'bg-background border-border text-muted-foreground hover:border-primary/50 hover:bg-accent/30 cursor-pointer'
  }
}

function sourceBadge(ep) {
  const map = { bt: 'BT', stream: 'Str', rss: 'RSS', bangumi: 'Str', manual: '手' }
  return map[ep.source] || ''
}

function shortDate(d) {
  // YYYY-MM-DD → MM/DD
  if (!d || d.length < 10) return d
  return `${d.slice(5, 7)}/${d.slice(8, 10)}`
}

function tooltipText(ep) {
  const titleHint = ep.name_cn || ep.title ? `\n${ep.name_cn || ep.title}` : ''
  switch (ep.status) {
    case 'completed':
      return `第 ${ep.episode_number} 集 · 已下载${titleHint}\n来源：${sourceName(ep.source)}\n点击查看详情`
    case 'downloading':
    case 'pending':
      return `第 ${ep.episode_number} 集 · 下载中${titleHint}`
    case 'no_resource':
      return `第 ${ep.episode_number} 集 · 暂未命中${titleHint}\n点击查看原因`
    case 'upcoming':
      return `第 ${ep.episode_number} 集 · 待发布（${ep.air_date || '日期未定'}）${titleHint}`
    default:
      return `第 ${ep.episode_number} 集 · 未下载${titleHint}`
  }
}

function sourceName(src) {
  return { bt: 'BT 种子', stream: '流媒体', rss: 'RSS', bangumi: '流媒体', manual: '手动' }[src] || src || '—'
}

function openDetail(n) {
  selectedEp.value = n
  showDrawer.value = true
}

function openManualSearch(n) {
  manualSearchEp.value = n || 0
  showManualSearch.value = true
}

async function triggerSearch() {
  triggering.value = true
  try {
    await post(`/anime/${props.animeId}/orchestrate`)
    message.success('已触发搜索，10-30 秒后刷新')
    setTimeout(refresh, 15000)
  } catch (e) {
    message.error(e.message || '触发失败')
  } finally {
    triggering.value = false
  }
}

async function refresh() {
  await Promise.all([fetchDownloads(), fetchDiagnosis(), fetchEpisodeStatus()])
}

async function fetchDownloads() {
  try {
    const resp = await get('/downloads', {
      params: { anime_id: props.animeId, page_size: 500 }
    })
    downloads.value = resp.tasks || resp.items || []
  } catch {
    downloads.value = []
  }
}

async function fetchDiagnosis() {
  try {
    const resp = await get(`/anime/${props.animeId}/diagnosis`)
    diagnosis.value = resp.episodes || []
  } catch {
    diagnosis.value = []
  }
}

async function fetchEpisodeStatus() {
  try {
    const resp = await get(`/anime/${props.animeId}/episode-status`)
    episodeStatus.value = resp.episodes || []
  } catch {
    episodeStatus.value = []
  }
}

watch(() => props.animeId, () => {
  if (props.animeId) refresh()
}, { immediate: true })

onMounted(() => {
  pollTimer = setInterval(() => {
    fetchDownloads()
    fetchEpisodeStatus()
  }, 5000)
})
onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})

defineExpose({ refresh })
</script>
