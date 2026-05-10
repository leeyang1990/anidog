<template>
  <!-- idle: 未初始化（极短暂） -->
  <div v-if="phase === 'idle'" class="py-8 text-center">
    <p class="text-sm text-muted-foreground">准备中...</p>
  </div>

  <!-- searching: 自动匹配中 -->
  <div v-else-if="phase === 'searching'" class="py-12 text-center">
    <n-spin size="large" />
    <p class="text-sm text-muted-foreground mt-4">正在匹配下载源...</p>
  </div>

  <!-- selecting: 源 + 候选 + 清单选择 -->
  <div v-else-if="phase === 'selecting'" class="space-y-5">
    <!-- 无结果 -->
    <div v-if="!matches.length && !isSearching" class="py-8 text-center">
      <p class="text-sm text-muted-foreground mb-3">未找到匹配的下载源</p>
      <button class="text-sm text-primary hover:underline" @click="startAutoMatch">重试</button>
    </div>

    <template v-else>
      <!-- 来源 chips -->
      <div>
        <div class="text-xs text-muted-foreground mb-2">来源</div>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="m in matches"
            :key="m.rule_id"
            class="px-3 py-1.5 rounded-full text-sm font-medium border transition-colors"
            :class="activeMatch?.rule_id === m.rule_id
              ? 'bg-primary text-primary-foreground border-primary'
              : 'bg-background text-muted-foreground border-border hover:border-primary/50'"
            @click="selectSource(m)"
          >
            {{ m.rule_name }}
            <span class="text-xs opacity-70 ml-1">({{ m.results.length }})</span>
          </button>
        </div>
      </div>

      <!-- 候选列表（多选：可勾选多个季度） -->
      <div v-if="activeMatch?.results.length">
        <div class="flex items-center justify-between mb-2">
          <div class="text-xs text-muted-foreground">
            季度 <span class="opacity-60">（可多选）</span>
          </div>
          <div v-if="selectedCandidates.length > 1" class="text-xs text-primary">
            已选 {{ selectedCandidates.length }} 个
          </div>
        </div>
        <div class="space-y-2 max-h-60 overflow-y-auto">
          <button
            v-for="(r, idx) in activeMatch.results"
            :key="r.url"
            class="w-full flex items-center gap-3 p-3 rounded-md border bg-background hover:border-primary hover:bg-accent/30 transition-colors text-left"
            :class="isCandidateSelected(r) ? 'border-primary bg-accent/30' : ''"
            @click="toggleCandidate(r)"
          >
            <!-- 多选勾选框 -->
            <div class="w-5 h-5 rounded border-2 flex items-center justify-center shrink-0 transition-colors"
              :class="isCandidateSelected(r) ? 'bg-primary border-primary' : 'border-border bg-background'">
              <n-icon v-if="isCandidateSelected(r)" size="12" class="text-primary-foreground">
                <CheckmarkCircleOutline />
              </n-icon>
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-medium truncate">
                {{ r.name }}
                <span v-if="isPrimaryCandidate(r)" class="ml-1 text-xs text-primary">· 主（预览中）</span>
              </div>
            </div>
          </button>
        </div>
      </div>

      <!-- 加载剧集 -->
      <div v-if="loadingEpisodes" class="py-4 text-center text-sm text-muted-foreground">
        加载剧集中...
      </div>

      <!-- 清单选择 -->
      <div v-if="roads.length && selectedCandidate">
        <div class="text-xs text-muted-foreground mb-2">清单</div>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="(road, idx) in roads"
            :key="idx"
            class="px-3 py-1.5 rounded-full text-sm font-medium border transition-colors"
            :class="selectedRoadIndex === idx
              ? 'bg-primary text-primary-foreground border-primary'
              : 'bg-background text-muted-foreground border-border hover:border-primary/50'"
            @click="selectedRoadIndex = idx"
          >
            {{ road.name }} ({{ road.episodes.length }}集)
          </button>
        </div>
      </div>

      <!-- 确认按钮 -->
      <div v-if="selectedCandidate && roads.length > 0 && !loadingEpisodes" class="pt-2 space-y-2">
        <div v-if="isSwitchingSource" class="text-xs text-amber-700 dark:text-amber-400 bg-amber-500/10 border border-amber-500/30 rounded px-3 py-2">
          切换源后会按新源重新下载（已下载的集跳过）
        </div>
        <div v-if="selectedCandidates.length > 1"
          class="text-xs text-primary bg-primary/5 border border-primary/20 rounded px-3 py-2">
          将同时下载 {{ selectedCandidates.length }} 个季度（共约 {{ estimatedTotalEpisodes }} 集）
        </div>
        <button
          class="w-full h-10 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors disabled:opacity-50"
          :disabled="confirming"
          @click="confirmAndDownload"
        >
          {{ confirming ? '处理中...' : confirmButtonLabel }}
        </button>
      </div>
    </template>
  </div>

  <!-- ready: 三层选择器 + 统一确认按钮 -->
  <div v-else-if="phase === 'ready'" class="space-y-4">
    <!-- 当前已应用配置 -->
    <div class="flex items-center gap-2 flex-wrap bg-primary/5 border border-primary/20 rounded-md px-4 py-3 text-sm">
      <n-icon size="16" class="text-primary shrink-0"><CheckmarkCircleOutline /></n-icon>
      <span class="text-muted-foreground">已追:</span>
      <span class="font-medium">{{ appliedRuleName || '—' }}</span>
      <span class="text-muted-foreground">/</span>
      <span class="font-medium">{{ seasonLabel(appliedCandidateName) }}</span>
      <span class="text-muted-foreground">/</span>
      <span class="font-medium text-primary">{{ appliedRoadName || '默认线路' }}</span>
      <span class="text-xs text-muted-foreground">· {{ downloadedCount }}/{{ selectedRoadEpisodes.length }} 集</span>
      <!-- 源健康状态 badge -->
      <span v-if="healthStatus && healthStatus !== 'healthy'"
        class="ml-auto inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium"
        :class="healthBadgeClass"
        :title="healthNote">
        <span>{{ healthLabel }}</span>
      </span>
      <span v-else-if="healthStatus === 'healthy'"
        class="ml-auto inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium bg-emerald-500/15 text-emerald-700 dark:text-emerald-400"
        :title="healthNote">
        <n-icon size="12"><CheckmarkCircleOutline /></n-icon> 源健康
      </span>
    </div>

    <!-- 1. 源（规则）切换：点击立即切换，下面层级跟着更新 -->
    <div v-if="matches.length > 0">
      <div class="text-xs text-muted-foreground mb-2">源 ({{ matches.length }})</div>
      <div class="flex flex-wrap gap-2">
        <button v-for="m in matches" :key="m.rule_id"
          class="px-3 py-1.5 rounded-md text-xs font-medium border transition-colors inline-flex items-center gap-1"
          :class="isDraftChanged('source', m.rule_id)
            ? 'bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/50'
            : activeMatch?.rule_id === m.rule_id
              ? 'bg-primary text-primary-foreground border-primary'
              : 'bg-background text-foreground border-border hover:border-primary/50'"
          @click="switchSource(m)"
          :disabled="switchingRoad">
          <n-icon v-if="activeMatch?.rule_id === m.rule_id && !isDraftChanged('source', m.rule_id)" size="12">
            <CheckmarkCircleOutline />
          </n-icon>
          {{ m.rule_name }}
          <span class="opacity-70">({{ m.results.length }})</span>
        </button>
      </div>
    </div>

    <!-- 2. 季度（候选）切换：点击立即切换，拉取剧集更新清单列表 -->
    <div v-if="activeMatch?.results?.length > 0">
      <div class="text-xs text-muted-foreground mb-2">季度 ({{ activeMatch.results.length }})</div>
      <div class="flex flex-wrap gap-2">
        <button v-for="r in activeMatch.results" :key="r.url"
          class="px-3 py-1.5 rounded-md text-xs font-medium border transition-colors inline-flex items-center gap-1"
          :class="isDraftChanged('candidate', r.url)
            ? 'bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/50'
            : selectedCandidate?.url === r.url
              ? 'bg-primary text-primary-foreground border-primary'
              : 'bg-background text-foreground border-border hover:border-primary/50'"
          @click="switchCandidate(r)"
          :disabled="switchingRoad">
          <n-icon v-if="selectedCandidate?.url === r.url && !isDraftChanged('candidate', r.url)" size="12">
            <CheckmarkCircleOutline />
          </n-icon>
          {{ seasonLabel(r.name) }}
        </button>
      </div>
    </div>

    <!-- 3. 清单切换：点击立即切换（只改视觉） -->
    <div v-if="roads.length > 0">
      <div class="text-xs text-muted-foreground mb-2">清单 ({{ roads.length }})</div>
      <div class="flex flex-wrap gap-2">
        <button v-for="(road, idx) in roads" :key="idx"
          class="px-3 py-1.5 rounded-md text-xs font-medium border transition-colors inline-flex items-center gap-1"
          :class="isDraftChanged('road', road.name)
            ? 'bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/50'
            : selectedRoadIndex === idx
              ? 'bg-primary text-primary-foreground border-primary'
              : 'bg-background text-foreground border-border hover:border-primary/50'"
          @click="selectedRoadIndex = idx"
          :disabled="switchingRoad">
          <n-icon v-if="selectedRoadIndex === idx && !isDraftChanged('road', road.name)" size="12">
            <CheckmarkCircleOutline />
          </n-icon>
          {{ road.name }}
          <span class="opacity-70">({{ road.episodes.length }}集)</span>
        </button>
      </div>
    </div>

    <!-- 统一的"确认更新"按钮：只在有改动时显示 -->
    <div v-if="hasDraftChanges"
      class="flex items-center justify-between gap-3 bg-amber-500/10 border border-amber-500/30 rounded-md px-3 py-2">
      <div class="text-xs text-amber-800 dark:text-amber-300">
        <span class="font-medium">配置已改变：</span>{{ draftSummary }}
      </div>
      <div class="flex gap-2 shrink-0">
        <button class="h-7 px-3 rounded text-xs text-muted-foreground hover:bg-background border border-border"
          @click="resetDraft" :disabled="switchingRoad">撤销</button>
        <button class="h-7 px-3 rounded text-xs bg-amber-500 text-white hover:bg-amber-600 disabled:opacity-50"
          @click="applyConfig" :disabled="switchingRoad">
          {{ switchingRoad ? '处理中...' : '确认更新并下载' }}
        </button>
      </div>
    </div>

    <!-- 剧集加载中提示 -->
    <div v-if="loadingEpisodes" class="py-4 text-center text-sm text-muted-foreground">
      加载剧集中...
    </div>

    <!-- 剧集网格 -->
    <div v-else class="grid grid-cols-5 sm:grid-cols-6 md:grid-cols-8 gap-2">
      <button
        v-for="(ep, idx) in selectedRoadEpisodes"
        :key="idx"
        class="aspect-square rounded-md flex flex-col items-center justify-center text-xs font-medium border transition-colors"
        :class="isDownloaded(idx + 1)
          ? 'bg-green-500/10 border-green-500/30 text-green-600'
          : 'bg-background border-border hover:border-primary hover:bg-accent/30 text-foreground'"
        @click="downloadSingle(ep, idx + 1)"
      >
        <span class="font-mono">{{ String(idx + 1).padStart(2, '0') }}</span>
        <n-icon v-if="isDownloaded(idx + 1)" size="14" class="mt-0.5"><CheckmarkCircleOutline /></n-icon>
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useMessage, NIcon, NSpin } from 'naive-ui'
import { CheckmarkCircleOutline, RefreshOutline } from '@vicons/ionicons5'
import { get, post, put } from '@/utils/api'

const props = defineProps({
  animeId: { type: Number, default: null },
  animeTitle: { type: String, default: '' },
  subscribed: { type: Boolean, default: false },
})

const message = useMessage()

// 状态机: idle → searching → selecting → ready
const phase = ref('idle')
const isSearching = ref(false)
const confirming = ref(false)
const loadingEpisodes = ref(false)
const isSwitchingSource = ref(false)

// 匹配数据
const matches = ref([]) // [{ rule_id, rule_name, results: [{ name, url }] }]
const activeMatch = ref(null)
// 多选候选：第一个为主（保存到 stream_preference 的那一个）
const selectedCandidates = ref([]) // [{ name, url }, ...]
// 向后兼容：selectedCandidate 作为 primary 的别名
const selectedCandidate = computed({
  get: () => selectedCandidates.value[0] || null,
  set: (v) => {
    if (!v) selectedCandidates.value = []
    else selectedCandidates.value = [v]
  },
})

// 剧集数据
const roads = ref([]) // [{ name, episodes }]
const selectedRoadIndex = ref(0)
const downloadedSet = ref(new Set())
const sourceName = ref('')
const switchingRoad = ref(false)

// 已保存到后端的配置快照（用于对比"是否有改动"）
const appliedConfig = ref({ rule_id: null, detail_url: '', road_name: '', rule_name: '', candidate_name: '' })

// 源健康状态（从 anime.source_health_status 读）
const healthStatus = ref('')
const healthNote = ref('')

const selectedRoadEpisodes = computed(() => {
  return roads.value[selectedRoadIndex.value]?.episodes || []
})

const currentRoadName = computed(() => {
  return roads.value[selectedRoadIndex.value]?.name || ''
})

// 便捷访问 applied config 的字段（状态栏展示用）
const appliedRuleName = computed(() => appliedConfig.value.rule_name)
const appliedCandidateName = computed(() => appliedConfig.value.candidate_name)
const appliedRoadName = computed(() => appliedConfig.value.road_name)

// 当前 draft（用户当前选中）vs applied（已保存）对比
const draftRuleId = computed(() => activeMatch.value?.rule_id || null)
const draftDetailUrl = computed(() => selectedCandidate.value?.url || '')
const draftRoadName = computed(() => currentRoadName.value)

const hasDraftChanges = computed(() => {
  return draftRuleId.value !== appliedConfig.value.rule_id ||
         draftDetailUrl.value !== appliedConfig.value.detail_url ||
         draftRoadName.value !== appliedConfig.value.road_name
})

// ---- 候选多选辅助 ----
function isCandidateSelected(r) {
  return selectedCandidates.value.some(c => c.url === r.url)
}
function isPrimaryCandidate(r) {
  return selectedCandidates.value[0]?.url === r.url
}
async function toggleCandidate(r) {
  if (!activeMatch.value || switchingRoad.value) return
  const idx = selectedCandidates.value.findIndex(c => c.url === r.url)
  if (idx >= 0) {
    // 已选 → 移除
    selectedCandidates.value.splice(idx, 1)
    // 若移除的是 primary，且还有其他选中，切到新 primary 的剧集预览
    if (idx === 0 && selectedCandidates.value.length > 0) {
      await fetchEpisodes(activeMatch.value.rule_id, selectedCandidates.value[0].url)
    } else if (selectedCandidates.value.length === 0) {
      roads.value = []
    }
  } else {
    // 未选 → 添加
    selectedCandidates.value.push(r)
    // 若是第一个选中，拉剧集供预览
    if (selectedCandidates.value.length === 1) {
      await fetchEpisodes(activeMatch.value.rule_id, r.url)
    }
  }
}

// 估算所有已选季度的总集数（仅 primary 的剧集已加载，其他用 0 或预估）
const estimatedTotalEpisodes = computed(() => {
  if (selectedCandidates.value.length <= 1) return selectedRoadEpisodes.value.length
  // primary 的集数已知，其他季度用 primary 的数量粗略估算
  return selectedRoadEpisodes.value.length * selectedCandidates.value.length
})

const confirmButtonLabel = computed(() => {
  if (selectedCandidates.value.length > 1) {
    return `确认并下载 ${selectedCandidates.value.length} 个季度`
  }
  return `确认并下载全部 ${selectedRoadEpisodes.value.length} 集`
})

// 某个 chip 是否和"已保存"不同（用于变色提示这是改动项）
function isDraftChanged(layer, key) {
  if (!hasDraftChanges.value) return false
  if (layer === 'source') {
    return activeMatch.value?.rule_id === key && appliedConfig.value.rule_id !== key
  }
  if (layer === 'candidate') {
    return selectedCandidate.value?.url === key && appliedConfig.value.detail_url !== key
  }
  if (layer === 'road') {
    return currentRoadName.value === key && appliedConfig.value.road_name !== key
  }
  return false
}

const draftSummary = computed(() => {
  const parts = []
  if (activeMatch.value?.rule_name && activeMatch.value.rule_name !== appliedConfig.value.rule_name) {
    parts.push(`源→${activeMatch.value.rule_name}`)
  }
  if (selectedCandidate.value?.url && selectedCandidate.value.url !== appliedConfig.value.detail_url) {
    parts.push(`季度→${seasonLabel(selectedCandidate.value.name)}`)
  }
  if (currentRoadName.value && currentRoadName.value !== appliedConfig.value.road_name) {
    parts.push(`清单→${currentRoadName.value}`)
  }
  return parts.join(' · ')
})

// 健康状态 UI
const healthLabel = computed(() => {
  switch (healthStatus.value) {
    case 'degraded': return '⚠ 源不稳定'
    case 'broken': return '✗ 源异常，建议切换'
    default: return ''
  }
})
const healthBadgeClass = computed(() => {
  switch (healthStatus.value) {
    case 'degraded': return 'bg-amber-500/15 text-amber-700 dark:text-amber-400'
    case 'broken': return 'bg-red-500/15 text-red-600 dark:text-red-400'
    default: return ''
  }
})

const downloadedCount = computed(() => {
  let count = 0
  for (let i = 0; i < selectedRoadEpisodes.value.length; i++) {
    if (downloadedSet.value.has(i + 1)) count++
  }
  return count
})

// 监听追番状态 + 番剧标题，任一就绪时触发 init
// （未追番时从 bangumi 详情进来，animeTitle 异步加载，需要 watch）
watch(
  () => [props.subscribed, props.animeTitle, props.animeId],
  () => {
    if (phase.value === 'idle' && props.animeTitle) {
      init()
    }
  },
  { immediate: true }
)

// 监听 draft 任一层变化：实时刷新剧集下载状态
// （切源/季度/清单时，立即按新的清单查询 completed 集，UI 打钩状态就会同步）
watch(
  () => [activeMatch.value?.rule_id, selectedCandidate.value?.url, currentRoadName.value],
  () => {
    if (phase.value === 'ready' && props.animeId && selectedCandidate.value) {
      fetchDownloadStatus()
    }
  }
)

onMounted(() => {
  // 由上方 watch({ immediate: true }) 负责触发 init
})

// localStorage 缓存 key
function cacheKey() {
  return `stream-matches:${props.animeId || ''}:${props.animeTitle || ''}`
}

function readCachedMatches() {
  try {
    const raw = localStorage.getItem(cacheKey())
    if (!raw) return null
    const data = JSON.parse(raw)
    // 过期 1 小时
    if (Date.now() - (data.ts || 0) > 60 * 60 * 1000) return null
    return data.matches || null
  } catch {
    return null
  }
}

function writeCachedMatches(ms) {
  try {
    localStorage.setItem(cacheKey(), JSON.stringify({ ts: Date.now(), matches: ms }))
  } catch { /* ignore quota */ }
}

async function init() {
  // 如果有 animeId，检查是否已保存源偏好
  if (props.animeId) {
    try {
      const anime = await get(`/anime/${props.animeId}`)
      if (anime.stream_detail_url && anime.stream_rule_id) {
        sourceName.value = anime.stream_rule_name || ''
        healthStatus.value = anime.source_health_status || ''
        healthNote.value = anime.source_health_note || ''
        // 初始化 applied 快照
        const candidateName = anime.title || props.animeTitle || ''
        appliedConfig.value = {
          rule_id: anime.stream_rule_id,
          rule_name: anime.stream_rule_name || '',
          detail_url: anime.stream_detail_url,
          candidate_name: candidateName,
          road_name: anime.stream_road_name || '',
        }

        // 尝试读 localStorage 缓存的完整 matches（刷新体验好）
        const cached = readCachedMatches()
        if (cached && cached.length) {
          matches.value = cached
          const active = cached.find(m => m.rule_id === anime.stream_rule_id)
          if (active) {
            activeMatch.value = active
            const cand = active.results.find(r => r.url === anime.stream_detail_url)
            selectedCandidate.value = cand || {
              url: anime.stream_detail_url,
              name: candidateName,
            }
          } else {
            fallbackToSinglePlaceholder(anime, candidateName)
          }
        } else {
          // 没缓存：用单项占位，让三层 UI 立刻有东西显示
          fallbackToSinglePlaceholder(anime, candidateName)
        }

        await loadEpisodesFromURL(anime.stream_rule_id, anime.stream_detail_url, anime.stream_road_name)
        phase.value = 'ready'

        // 后台异步拉取最新 matches，更新 UI + 写缓存
        loadMatchesForSwitching(anime.stream_rule_id, anime.stream_detail_url)
        return
      }
    } catch { /* ignore */ }
  }
  startAutoMatch()
}

// 只有单源单候选的最小占位（保证三层 UI 立即渲染，不闪）
function fallbackToSinglePlaceholder(anime, candidateName) {
  const singleCandidate = {
    url: anime.stream_detail_url,
    name: candidateName,
  }
  const singleMatch = {
    rule_id: anime.stream_rule_id,
    rule_name: anime.stream_rule_name || '',
    results: [singleCandidate],
  }
  matches.value = [singleMatch]
  activeMatch.value = singleMatch
  selectedCandidate.value = singleCandidate
}

// 已有源偏好时，后台加载所有源的候选，供用户切换
async function loadMatchesForSwitching(preferredRuleId, preferredUrl) {
  if (!props.animeTitle) return
  try {
    const resp = await get('/stream/auto-match', { params: { keyword: props.animeTitle } })
    const ms = resp.matches || []
    if (!ms.length) return
    matches.value = ms
    writeCachedMatches(ms)
    // 补齐 activeMatch 和 selectedCandidate 引用到完整列表的对象
    const active = ms.find(m => m.rule_id === preferredRuleId)
    if (active) {
      activeMatch.value = active
      sourceName.value = active.rule_name
      const cand = active.results.find(r => r.url === preferredUrl)
      if (cand) {
        selectedCandidate.value = cand
        // 补齐 applied 快照的 candidate_name
        appliedConfig.value = {
          ...appliedConfig.value,
          candidate_name: cand.name,
          rule_name: active.rule_name,
        }
      }
    }
  } catch { /* ignore */ }
}

async function startAutoMatch() {
  if (!props.animeTitle) return
  phase.value = 'searching'
  isSearching.value = true
  matches.value = []
  activeMatch.value = null
  selectedCandidate.value = null
  roads.value = []

  try {
    const resp = await get('/stream/auto-match', { params: { keyword: props.animeTitle } })
    matches.value = resp.matches || []

    if (matches.value.length) {
      const first = matches.value[0]
      selectSource(first)
      if (first.results.length) {
        selectedCandidate.value = first.results[0]
        await fetchEpisodes(first.rule_id, first.results[0].url)
      }
    }
  } catch (e) {
    message.error('搜索源失败')
  } finally {
    isSearching.value = false
    phase.value = 'selecting'
  }
}

function selectSource(match) {
  activeMatch.value = match
  selectedCandidate.value = null
  roads.value = []
}

async function selectCandidate(r) {
  if (selectedCandidate.value?.url === r.url) return
  selectedCandidate.value = r
  if (activeMatch.value) {
    await fetchEpisodes(activeMatch.value.rule_id, r.url)
  }
}

async function fetchEpisodes(ruleId, detailUrl) {
  loadingEpisodes.value = true
  try {
    const resp = await get('/stream/episodes', {
      params: { detail_url: detailUrl, rule_id: ruleId }
    })
    buildRoads(resp.episodes || [])
    await fetchDownloadStatus()
  } catch {
    roads.value = []
  } finally {
    loadingEpisodes.value = false
  }
}

function buildRoads(allEpisodes) {
  const roadMap = new Map()
  allEpisodes.forEach(ep => {
    const name = ep.road_name || '默认线路'
    if (!roadMap.has(name)) roadMap.set(name, [])
    roadMap.get(name).push(ep)
  })
  roads.value = Array.from(roadMap.entries()).map(([name, episodes]) => ({ name, episodes }))
  selectedRoadIndex.value = 0
}

async function loadEpisodesFromURL(ruleId, detailUrl, roadName) {
  loadingEpisodes.value = true
  try {
    const resp = await get('/stream/episodes', {
      params: { detail_url: detailUrl, rule_id: ruleId }
    })
    buildRoads(resp.episodes || [])
    // 选择已保存的清单
    if (roadName) {
      const idx = roads.value.findIndex(r => r.name === roadName)
      selectedRoadIndex.value = idx >= 0 ? idx : 0
    }
    await fetchDownloadStatus()
  } catch {
    startAutoMatch()
  } finally {
    loadingEpisodes.value = false
  }
}

async function fetchDownloadStatus() {
  if (!props.animeId) return
  const roadName = currentRoadName.value
  const detailURL = selectedCandidate.value?.url || ''
  // 没有清单名或候选（切源后未选）时清空，避免误打钩
  if (!roadName || !detailURL) {
    downloadedSet.value = new Set()
    return
  }
  try {
    const params = {
      anime_id: props.animeId,
      status: 'completed',
      road_name: roadName,
      detail_url: detailURL,
      page: 1,
      page_size: 1000,
    }
    const resp = await get('/downloads', { params })
    const tasks = resp.tasks || []
    downloadedSet.value = new Set(tasks.map(d => d.episode_number).filter(Boolean))
  } catch {
    downloadedSet.value = new Set()
  }
}

function isDownloaded(num) {
  return downloadedSet.value.has(num)
}

async function confirmAndDownload() {
  if (!selectedCandidate.value || !activeMatch.value) return
  confirming.value = true
  try {
    const ruleId = activeMatch.value.rule_id
    const candidates = selectedCandidates.value
    let totalQueued = 0

    // 对每个选中的季度：拉剧集 → 取首路线 → 批量下载
    for (let i = 0; i < candidates.length; i++) {
      const cand = candidates[i]
      let epsForCand = []
      if (i === 0) {
        // primary：已加载到 roads，直接用 selectedRoadEpisodes
        epsForCand = selectedRoadEpisodes.value
      } else {
        // 其他季度：现场拉剧集（取第一条路线）
        try {
          const resp = await get('/stream/episodes', {
            params: { detail_url: cand.url, rule_id: ruleId }
          })
          const allEps = resp.episodes || []
          // 按 road_name 分组，取第一个 road
          const byRoad = new Map()
          allEps.forEach(ep => {
            const n = ep.road_name || '默认线路'
            if (!byRoad.has(n)) byRoad.set(n, [])
            byRoad.get(n).push(ep)
          })
          const firstRoad = byRoad.values().next().value
          epsForCand = firstRoad || []
        } catch {
          epsForCand = []
        }
      }

      if (!epsForCand.length) continue

      const payload = epsForCand.map((ep, idx) => ({ url: ep.url, name: ep.name, number: idx + 1 }))
      await post('/stream/download/batch', {
        rule_id: ruleId,
        episodes: payload,
        anime_name: props.animeTitle,
        anime_id: props.animeId,
      })
      totalQueued += payload.length
    }

    // primary 季度的集数同步到 downloadedSet 显示已下载（UI 打钩）
    selectedRoadEpisodes.value.forEach((_, i) => downloadedSet.value.add(i + 1))

    // 保存 primary 的源偏好到后端
    if (props.animeId) {
      await put(`/anime/${props.animeId}/stream-preference`, {
        rule_id: ruleId,
        detail_url: selectedCandidate.value.url,
        road_name: roads.value[selectedRoadIndex.value]?.name || '',
        rule_name: activeMatch.value.rule_name,
      })
    }

    sourceName.value = activeMatch.value.rule_name
    // 更新 applied 快照（用 primary）
    appliedConfig.value = {
      rule_id: ruleId,
      rule_name: activeMatch.value.rule_name,
      detail_url: selectedCandidate.value.url,
      candidate_name: selectedCandidate.value.name || '',
      road_name: roads.value[selectedRoadIndex.value]?.name || '',
    }
    message.success(
      candidates.length > 1
        ? `已添加 ${totalQueued} 个下载任务（覆盖 ${candidates.length} 个季度）`
        : `已添加 ${totalQueued} 个下载任务`
    )
    phase.value = 'ready'
    isSwitchingSource.value = false
  } catch (e) {
    message.error(e.message || '操作失败')
  } finally {
    confirming.value = false
  }
}

async function downloadSingle(ep, number) {
  if (isDownloaded(number) || !activeMatch.value) return
  try {
    await post('/stream/download', {
      rule_id: activeMatch.value.rule_id,
      episode_url: ep.url,
      anime_name: props.animeTitle,
      episode_number: number,
      anime_id: props.animeId,
    })
    downloadedSet.value = new Set([...downloadedSet.value, number])
    message.success(`已添加下载: 第${number}集`)
  } catch (e) {
    message.error(e.message || '下载失败')
  }
}

function reconfigure() {
  isSwitchingSource.value = true
  phase.value = 'selecting'
}

// ---- 分层 draft：切换只改 UI，点"确认更新"按钮统一保存+下载 ----

// 切换源（只改 draft）
function switchSource(m) {
  if (!m || switchingRoad.value) return
  if (activeMatch.value?.rule_id === m.rule_id) return
  activeMatch.value = m
  sourceName.value = m.rule_name
  // 清空候选和清单，等待用户选新源的候选
  selectedCandidate.value = null
  roads.value = []
  selectedRoadIndex.value = 0
}

// 切换候选/季度（只改 draft，拉取剧集供用户看清单列表）
async function switchCandidate(r) {
  if (!r || !activeMatch.value || switchingRoad.value) return
  if (selectedCandidate.value?.url === r.url) return
  selectedCandidate.value = r
  await fetchEpisodes(activeMatch.value.rule_id, r.url)
  if (roads.value.length > 0) {
    selectedRoadIndex.value = 0
  }
}

// 撤销 draft 改动：恢复到 applied 状态
function resetDraft() {
  const applied = appliedConfig.value
  if (!applied.rule_id) return
  const m = matches.value.find(x => x.rule_id === applied.rule_id)
  if (!m) return
  activeMatch.value = m
  sourceName.value = m.rule_name
  const r = m.results.find(x => x.url === applied.detail_url)
  if (r) {
    selectedCandidate.value = r
    // 如果 roads 不是这个候选的，重新拉
    if (roads.value.length === 0 || currentRoadName.value !== applied.road_name) {
      fetchEpisodes(applied.rule_id, applied.detail_url).then(() => {
        const idx = roads.value.findIndex(x => x.name === applied.road_name)
        if (idx >= 0) selectedRoadIndex.value = idx
      })
    } else {
      const idx = roads.value.findIndex(x => x.name === applied.road_name)
      if (idx >= 0) selectedRoadIndex.value = idx
    }
  }
}

// 统一应用配置：保存偏好 + 触发下载
async function applyConfig() {
  if (!props.animeId || !activeMatch.value || !selectedCandidate.value) return
  const road = roads.value[selectedRoadIndex.value]
  if (!road) {
    message.error('请选择清单')
    return
  }

  switchingRoad.value = true
  try {
    await put(`/anime/${props.animeId}/stream-preference`, {
      rule_id: activeMatch.value.rule_id,
      detail_url: selectedCandidate.value.url,
      road_name: road.name,
      rule_name: activeMatch.value.rule_name,
    })
    await post(`/anime/${props.animeId}/check-updates`)

    // 更新 applied 快照
    appliedConfig.value = {
      rule_id: activeMatch.value.rule_id,
      rule_name: activeMatch.value.rule_name,
      detail_url: selectedCandidate.value.url,
      candidate_name: selectedCandidate.value.name || '',
      road_name: road.name,
    }

    message.success('已更新配置，开始下载')
    await fetchDownloadStatus()
  } catch (e) {
    message.error(e.message || '应用配置失败')
  } finally {
    switchingRoad.value = false
  }
}

// 从候选标题提取友好的季度标签：
// "Re：从零开始的异世界生活 第四季 丧失篇" → "第四季 丧失篇"
// "Re：从零开始的异世界生活" → "第一季"
function seasonLabel(name) {
  if (!name) return '—'
  // 匹配"第X季"后面的所有内容
  const m = name.match(/(第[0-9一二三四五六七八九十]+[季部].*)$/)
  if (m) return m[1].trim()
  // 匹配 Season X
  const m2 = name.match(/(Season\s*\d+.*)$/i)
  if (m2) return m2[1].trim()
  // 没有季度信息 → 第一季
  return '第一季'
}

</script>
