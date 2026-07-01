<template>
  <AcModal
    :show="show"
    :title="`手动选择种子 · ${animeTitle}${episode ? ' · 第 ' + String(episode).padStart(2,'0') + ' 集' : ''}`"
    :max-width="'820px'"
    @update:show="$emit('update:show', $event)"
  >
    <div class="space-y-3">
      <!-- 搜索栏 -->
      <div class="flex items-center gap-2 flex-wrap">
        <AcInput
          v-model="keyword"
          placeholder="搜索关键词（番剧名）"
          class="flex-1 min-w-[200px]"
          @keyup-enter="runSearch"
        />

        <AcSelect v-model="episodeFilter" :options="episodeOptions" class="w-32" />

        <AcButton variant="primary" :loading="loading" @click="runSearch">
          {{ loading ? '搜索中...' : '聚合搜索' }}
        </AcButton>
      </div>

      <!-- 源筛选 chips -->
      <div class="flex flex-wrap gap-2 items-center">
        <span class="text-xs text-muted-foreground font-bold">源:</span>
        <button v-for="ix in indexerOptions" :key="ix.value"
          type="button"
          class="px-2.5 py-1 rounded-full text-xs font-bold border-2 transition-colors"
          :class="selectedIndexers.includes(ix.value)
            ? 'bg-ac-grass text-white border-ac-grass-dark'
            : 'bg-card text-muted-foreground border-ac-sand hover:border-ac-grass'"
          @click="toggleIndexer(ix.value)">
          {{ ix.label }}
        </button>
      </div>

      <!-- 结果 -->
      <div v-if="!candidates.length && !loading" class="py-10 text-center text-sm text-muted-foreground">
        {{ searched ? '未找到结果，换个关键词？' : '点"聚合搜索"开始 🌱' }}
      </div>

      <div v-else-if="loading" class="py-10 flex justify-center"><AcSpinner :size="36" /></div>

      <div v-else class="max-h-[55vh] overflow-y-auto -mr-2 pr-2 space-y-2">
        <div v-for="(c, idx) in candidates" :key="idx"
          class="p-3 rounded-2xl border-2 border-ac-sand hover:border-ac-grass transition-colors">
          <div class="flex items-start gap-3">
            <!-- rank -->
            <div class="shrink-0 size-8 rounded-full bg-ac-cream text-ac-wood-dark flex items-center justify-center text-xs font-num font-bold border-2 border-ac-sand">
              {{ idx + 1 }}
            </div>

            <div class="flex-1 min-w-0 space-y-1">
              <!-- meta -->
              <div class="flex items-center gap-2 flex-wrap text-xs">
                <AcTag v-if="c.parsed?.group" variant="grass" size="sm">{{ c.parsed.group }}</AcTag>
                <AcTag v-if="c.parsed?.episode_num" variant="leaf" size="sm">EP {{ String(c.parsed.episode_num).padStart(2,'0') }}</AcTag>
                <AcTag v-if="c.parsed?.is_batch" variant="wood" size="sm">合集 {{ c.parsed.batch_start }}–{{ c.parsed.batch_end }}</AcTag>
                <span v-if="c.parsed?.quality" class="text-muted-foreground">{{ c.parsed.quality }}</span>
                <span v-if="c.parsed?.source" class="text-muted-foreground">· {{ c.parsed.source }}</span>
                <span v-if="c.parsed?.lang?.length" class="text-muted-foreground">
                  · {{ c.parsed.lang.map(l => langLabel(l)).join('/') }}
                </span>
                <span class="ml-auto text-muted-foreground inline-flex items-center gap-2">
                  <span class="uppercase font-bold">{{ c.source_name }}</span>
                  <span v-if="c.score !== 0" class="font-num">· Score {{ c.score.toFixed(1) }}</span>
                </span>
              </div>

              <!-- title -->
              <div class="text-sm truncate" :title="c.title">{{ c.title }}</div>

              <!-- info -->
              <div class="flex items-center gap-3 text-xs text-muted-foreground font-num">
                <span>{{ formatSize(c.size) }}</span>
                <span v-if="c.seeders > 0">👥 {{ c.seeders }}</span>
                <span v-if="c.leechers > 0">⬇ {{ c.leechers }}</span>
                <span v-if="c.pub_date">{{ formatDate(c.pub_date) }}</span>
              </div>
            </div>

            <!-- action -->
            <AcButton size="sm" variant="primary"
              :loading="downloading === c.info_hash || downloading === c.title"
              @click="download(c)">
              下载
            </AcButton>
          </div>
        </div>
      </div>
    </div>
  </AcModal>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { get, post } from '@/utils/api'
import { useToast } from '@/composables/useToast'
import { AcModal, AcButton, AcInput, AcSelect, AcSpinner, AcTag } from '@/components/ac'

const props = defineProps({
  show: Boolean,
  animeId: { type: Number, default: null },
  animeTitle: { type: String, default: '' },
  episode: { type: Number, default: 0 },
})

const emit = defineEmits(['update:show', 'downloaded'])

const toast = useToast()

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

const episodeOptions = computed(() => [
  { label: '所有集', value: 0 },
  ...Array.from({ length: 50 }, (_, i) => ({ label: `第 ${String(i + 1).padStart(2, '0')} 集`, value: i + 1 })),
])

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
    toast.warning('请输入搜索关键词')
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
    toast.error(e.message || '搜索失败')
    candidates.value = []
  } finally {
    loading.value = false
  }
}

async function download(c) {
  const url = c.magnet_url || c.torrent_url
  if (!url) {
    toast.error('候选缺少磁链 / 种子 URL')
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
    toast.success('已加入下载队列')
    emit('downloaded', c)
    setTimeout(() => emit('update:show', false), 500)
  } catch (e) {
    toast.error(e.message || '下载失败')
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
