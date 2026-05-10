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
      <button v-for="n in effectiveCount" :key="n"
        class="aspect-square rounded-md border flex flex-col items-center justify-center text-xs transition-colors relative"
        :class="cellClass(n)"
        :title="tooltipText(n)"
        @click="openDetail(n)">
        <span class="font-mono font-medium">{{ String(n).padStart(2, '0') }}</span>
        <!-- 状态图标 -->
        <n-icon v-if="statusOf(n) === 'completed'" size="12" class="absolute top-0.5 right-0.5 text-emerald-600">
          <CheckmarkCircleOutline />
        </n-icon>
        <n-icon v-else-if="statusOf(n) === 'downloading'" size="12" class="absolute top-0.5 right-0.5 text-primary animate-pulse">
          <CloudDownloadOutline />
        </n-icon>
        <n-icon v-else-if="statusOf(n) === 'no_resource'" size="12" class="absolute top-0.5 right-0.5 text-amber-500">
          <AlertCircleOutline />
        </n-icon>
        <!-- 来源 badge -->
        <span v-if="statusOf(n) === 'completed'" class="absolute bottom-0.5 right-0.5 text-[9px] font-bold opacity-60">
          {{ sourceBadge(n) }}
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
  CheckmarkCircleOutline, CloudDownloadOutline, AlertCircleOutline,
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

const downloads = ref([]) // 该 anime 的所有 download 记录
const diagnosis = ref([]) // 诊断数据 { episode_number, sources: {bt: {...}, ...} }
const triggering = ref(false)

const showDrawer = ref(false)
const selectedEp = ref(0)

const showManualSearch = ref(false)
const manualSearchEp = ref(0)

let pollTimer = null

// 按 episode_number 分组下载
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

// 有效显示集数：优先 Bangumi 的 episode_count；缺则用"已见过的最大集数"动态扩展。
// 用这个避免 "0/0 集" 的糟糕体验（当 Bangumi 没收录集数时）
const effectiveCount = computed(() => {
  const fromBangumi = props.episodeCount || 0
  let maxEpFromDl = 0
  for (const d of downloads.value) {
    if (d.episode_number && d.episode_number > maxEpFromDl) maxEpFromDl = d.episode_number
  }
  for (const dg of diagnosis.value) {
    if (dg.episode_number && dg.episode_number > maxEpFromDl) maxEpFromDl = dg.episode_number
  }
  // 连一集都没见过 → 默认 12（一季番常见长度），给个占位
  if (fromBangumi <= 0 && maxEpFromDl <= 0) return 12
  return Math.max(fromBangumi, maxEpFromDl)
})

const completedCount = computed(() => {
  let n = 0
  for (let i = 1; i <= effectiveCount.value; i++) {
    if (statusOf(i) === 'completed') n++
  }
  return n
})

const downloadingCount = computed(() => {
  let n = 0
  for (let i = 1; i <= effectiveCount.value; i++) {
    if (statusOf(i) === 'downloading') n++
  }
  return n
})

const noResourceCount = computed(() => {
  let n = 0
  for (let i = 1; i <= effectiveCount.value; i++) {
    if (statusOf(i) === 'no_resource') n++
  }
  return n
})

function statusOf(n) {
  const dls = downloadsByEp.value[n] || []
  if (dls.some(d => d.status === 'completed')) return 'completed'
  if (dls.some(d => d.status === 'downloading' || d.status === 'pending')) return 'downloading'
  // 有诊断且无下载 → no_resource
  if (diagnosisByEp.value[n]) return 'no_resource'
  return 'idle'
}

function cellClass(n) {
  const s = statusOf(n)
  switch (s) {
    case 'completed':
      return 'bg-emerald-500/10 border-emerald-500/30 text-emerald-700 dark:text-emerald-400 hover:bg-emerald-500/20 cursor-pointer'
    case 'downloading':
      return 'bg-primary/10 border-primary/30 text-primary hover:bg-primary/20 cursor-pointer'
    case 'no_resource':
      return 'bg-amber-500/10 border-amber-500/30 text-amber-700 dark:text-amber-400 hover:bg-amber-500/20 cursor-pointer'
    default:
      return 'bg-background border-border text-muted-foreground hover:border-primary/50 hover:bg-accent/30 cursor-pointer'
  }
}

function sourceBadge(n) {
  const dls = downloadsByEp.value[n] || []
  const completed = dls.find(d => d.status === 'completed')
  if (!completed) return ''
  const map = { bt: 'BT', stream: 'Str', rss: 'RSS', bangumi: 'Str', manual: '手' }
  return map[completed.source] || ''
}

function tooltipText(n) {
  const s = statusOf(n)
  const dls = downloadsByEp.value[n] || []
  switch (s) {
    case 'completed': {
      const d = dls.find(x => x.status === 'completed')
      return `第 ${n} 集 · 已下载\n来源：${sourceName(d?.source)}\n点击查看详情`
    }
    case 'downloading': {
      const d = dls.find(x => x.status === 'downloading' || x.status === 'pending')
      return `第 ${n} 集 · 下载中 ${Math.round((d?.progress || 0) * 10) / 10}%`
    }
    case 'no_resource':
      return `第 ${n} 集 · 暂未命中\n点击查看原因`
    default:
      return `第 ${n} 集 · 等待检查`
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
  await Promise.all([fetchDownloads(), fetchDiagnosis()])
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

watch(() => props.animeId, () => {
  if (props.animeId) refresh()
}, { immediate: true })

onMounted(() => {
  pollTimer = setInterval(fetchDownloads, 5000)
})
onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})

defineExpose({ refresh })
</script>
