<template>
  <n-modal :show="show" @update:show="$emit('update:show', $event)" preset="card"
    :style="{ width: '820px' }" :bordered="false"
    :title="`手动选择种子 · ${animeTitle}${episode ? ' · 第 ' + String(episode).padStart(2,'0') + ' 集' : ''}`">

    <div class="space-y-3">
      <!-- 搜索栏 -->
      <div class="flex items-center gap-2 flex-wrap">
        <input v-model="keyword"
          @keydown.enter="runSearch"
          placeholder="搜索关键词（番剧名）"
          class="h-9 flex-1 min-w-[200px] rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" />

        <select v-model="episodeFilter"
          class="h-9 rounded-md border border-input bg-background px-2 text-sm">
          <option :value="0">所有集</option>
          <option v-for="n in 50" :key="n" :value="n">第 {{ String(n).padStart(2,'0') }} 集</option>
        </select>

        <button @click="runSearch" :disabled="loading"
          class="h-9 px-4 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50">
          {{ loading ? '搜索中...' : '聚合搜索' }}
        </button>
      </div>

      <!-- 源筛选 chips -->
      <div class="flex flex-wrap gap-2 items-center">
        <span class="text-xs text-muted-foreground">源:</span>
        <button v-for="ix in indexerOptions" :key="ix.value"
          type="button"
          class="px-2.5 py-1 rounded-full text-xs font-medium border transition-colors"
          :class="selectedIndexers.includes(ix.value)
            ? 'bg-primary text-primary-foreground border-primary'
            : 'bg-background text-muted-foreground border-border hover:border-primary/50'"
          @click="toggleIndexer(ix.value)">
          {{ ix.label }}
        </button>
      </div>

      <!-- 结果 -->
      <div v-if="!candidates.length && !loading" class="py-10 text-center text-sm text-muted-foreground">
        {{ searched ? '未找到结果，换个关键词？' : '点"聚合搜索"开始' }}
      </div>

      <n-spin v-else-if="loading" :show="loading">
        <div class="py-10"></div>
      </n-spin>

      <div v-else class="max-h-[55vh] overflow-y-auto -mr-2 pr-2 space-y-2">
        <div v-for="(c, idx) in candidates" :key="idx"
          class="p-3 rounded-md border hover:border-primary/50 transition-colors">
          <div class="flex items-start gap-3">
            <!-- rank -->
            <div class="shrink-0 w-8 h-8 rounded-full bg-muted text-muted-foreground flex items-center justify-center text-xs font-mono font-bold">
              {{ idx + 1 }}
            </div>

            <div class="flex-1 min-w-0 space-y-1">
              <!-- meta -->
              <div class="flex items-center gap-2 flex-wrap text-xs">
                <span v-if="c.parsed?.group"
                  class="inline-flex items-center rounded px-1.5 py-0.5 font-bold bg-primary/20 text-primary">
                  {{ c.parsed.group }}
                </span>
                <span v-if="c.parsed?.episode_num"
                  class="inline-flex items-center rounded px-1.5 py-0.5 bg-emerald-500/15 text-emerald-700 dark:text-emerald-400">
                  EP {{ String(c.parsed.episode_num).padStart(2,'0') }}
                </span>
                <span v-if="c.parsed?.is_batch"
                  class="inline-flex items-center rounded px-1.5 py-0.5 bg-violet-500/15 text-violet-700 dark:text-violet-400">
                  合集 {{ c.parsed.batch_start }}–{{ c.parsed.batch_end }}
                </span>
                <span v-if="c.parsed?.quality" class="text-muted-foreground">{{ c.parsed.quality }}</span>
                <span v-if="c.parsed?.source" class="text-muted-foreground">· {{ c.parsed.source }}</span>
                <span v-if="c.parsed?.lang?.length" class="text-muted-foreground">
                  · {{ c.parsed.lang.map(l => langLabel(l)).join('/') }}
                </span>
                <span class="ml-auto text-muted-foreground inline-flex items-center gap-2">
                  <span class="uppercase">{{ c.source_name }}</span>
                  <span v-if="c.score !== 0">· Score {{ c.score.toFixed(1) }}</span>
                </span>
              </div>

              <!-- title -->
              <div class="text-sm truncate" :title="c.title">{{ c.title }}</div>

              <!-- info -->
              <div class="flex items-center gap-3 text-xs text-muted-foreground">
                <span>{{ formatSize(c.size) }}</span>
                <span v-if="c.seeders > 0">👥 {{ c.seeders }}</span>
                <span v-if="c.leechers > 0">⬇ {{ c.leechers }}</span>
                <span v-if="c.pub_date">{{ formatDate(c.pub_date) }}</span>
              </div>
            </div>

            <!-- action -->
            <button @click="download(c)" :disabled="downloading === c.info_hash || downloading === c.title"
              class="shrink-0 h-8 px-3 rounded-md bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 disabled:opacity-50">
              {{ downloading === c.info_hash || downloading === c.title ? '下载中...' : '下载' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </n-modal>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useMessage, NModal, NSpin } from 'naive-ui'
import { get, post } from '@/utils/api'

const props = defineProps({
  show: Boolean,
  animeId: { type: Number, default: null },
  animeTitle: { type: String, default: '' },
  episode: { type: Number, default: 0 },
})

const emit = defineEmits(['update:show', 'downloaded'])

const message = useMessage()

const keyword = ref('')
const episodeFilter = ref(0)
const candidates = ref([])
const loading = ref(false)
const searched = ref(false)
const downloading = ref(null)
const selectedIndexers = ref(['mikan', 'dmhy', 'bangumimoe'])

const indexerOptions = [
  { label: 'Mikan', value: 'mikan' },
  { label: 'Dmhy', value: 'dmhy' },
  { label: 'BangumiMoe', value: 'bangumimoe' },
  { label: 'Nyaa', value: 'nyaa' },
]

function toggleIndexer(v) {
  const i = selectedIndexers.value.indexOf(v)
  if (i >= 0) selectedIndexers.value.splice(i, 1)
  else selectedIndexers.value.push(v)
}

function langLabel(l) {
  return { simplified: '简中', traditional: '繁中', japanese: '日文', english: '英文' }[l] || l
}

function formatSize(b) {
  if (!b) return '—'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(Math.floor(Math.log(b) / Math.log(1024)), units.length - 1)
  return (b / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}
function formatDate(s) {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return ''
  return d.toLocaleDateString('zh-CN')
}

async function runSearch() {
  if (!keyword.value.trim()) {
    message.warning('请输入搜索关键词')
    return
  }
  loading.value = true
  searched.value = true
  try {
    const resp = await post('/indexer/search', {
      keyword: keyword.value.trim(),
      indexers: selectedIndexers.value,
      target_episode: episodeFilter.value || 0,
    })
    candidates.value = resp.candidates || []
  } catch (e) {
    message.error(e.message || '搜索失败')
    candidates.value = []
  } finally {
    loading.value = false
  }
}

async function download(c) {
  const url = c.magnet_url || c.torrent_url
  if (!url) {
    message.error('候选缺少磁链 / 种子 URL')
    return
  }
  downloading.value = c.info_hash || c.title
  try {
    const ep = c.parsed?.episode_num || (episodeFilter.value || null)
    await post('/downloads/', {
      url,
      title: c.title,
      name: c.title,
      download_type: 'torrent',
      source: 'bt',
      anime_id: props.animeId || undefined,
      episode_number: ep || undefined,
    })
    message.success('已加入下载队列')
    emit('downloaded', c)
    // 轻微延迟再关闭，便于用户看到反馈
    setTimeout(() => emit('update:show', false), 500)
  } catch (e) {
    message.error(e.message || '下载失败')
  } finally {
    downloading.value = null
  }
}

// 打开对话框时自动搜索
watch(() => props.show, (v) => {
  if (v) {
    keyword.value = props.animeTitle || ''
    episodeFilter.value = props.episode || 0
    candidates.value = []
    searched.value = false
    if (keyword.value) {
      runSearch()
    }
  }
})
</script>
