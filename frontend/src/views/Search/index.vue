<template>
  <div>
    <PageHeader title="资源搜索" subtitle="跨站点搜索可下载的番剧资源" />

    <!-- Tabs -->
    <div class="mb-6 flex border-b border-border">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        class="px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px"
        :class="activeTab === tab.key
          ? 'border-primary text-primary'
          : 'border-transparent text-muted-foreground hover:text-foreground'"
        @click="activeTab = tab.key"
      >{{ tab.label }}</button>
    </div>

    <!-- Torrent (BT 资源) search tab -->
    <template v-if="activeTab === 'torrent'">
      <!-- Search bar -->
      <div class="max-w-3xl mx-auto mb-4">
        <div class="flex gap-3">
          <div class="relative flex-1">
            <n-icon size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"><SearchOutline /></n-icon>
            <input
              v-model="keyword"
              class="h-12 w-full rounded-lg border border-input bg-background pl-10 pr-4 text-base placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
              placeholder="输入番剧名搜索 BT 资源..."
              @keydown.enter="doSearch"
            />
          </div>
          <button
            class="h-12 px-6 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors disabled:opacity-50"
            @click="doSearch"
            :disabled="searching"
          >搜索</button>
        </div>

        <!-- 源选择 chips -->
        <div class="flex flex-wrap gap-2 items-center mt-3">
          <span class="text-xs text-muted-foreground">站点:</span>
          <button v-for="ix in indexerOptions" :key="ix.value"
            type="button"
            class="px-2.5 py-1 rounded-full text-xs font-medium border transition-colors"
            :class="selectedIndexers.includes(ix.value)
              ? 'bg-primary text-primary-foreground border-primary'
              : 'bg-background text-muted-foreground border-border hover:border-primary/50'"
            @click="toggleIndexer(ix.value)">{{ ix.label }}</button>
        </div>
      </div>

      <!-- Results -->
      <div v-if="searching" class="flex justify-center py-12">
        <n-spin size="large" />
      </div>
      <div v-else-if="results.length" class="space-y-2 max-w-5xl mx-auto">
        <div
          v-for="(item, idx) in results"
          :key="item.info_hash || idx"
          class="bg-card rounded-lg border p-3 hover:border-primary/50 transition-colors"
        >
          <div class="flex items-start gap-3">
            <div class="shrink-0 w-8 h-8 rounded-full bg-muted text-muted-foreground flex items-center justify-center text-xs font-mono font-bold">
              {{ idx + 1 }}
            </div>

            <div class="flex-1 min-w-0 space-y-1">
              <!-- meta chips -->
              <div class="flex items-center gap-2 flex-wrap text-xs">
                <span v-if="item.parsed?.group"
                  class="inline-flex items-center rounded px-1.5 py-0.5 font-bold bg-primary/20 text-primary">
                  {{ item.parsed.group }}
                </span>
                <span v-if="item.parsed?.episode_num"
                  class="inline-flex items-center rounded px-1.5 py-0.5 bg-emerald-500/15 text-emerald-700 dark:text-emerald-400">
                  EP {{ String(item.parsed.episode_num).padStart(2,'0') }}
                </span>
                <span v-if="item.parsed?.is_batch"
                  class="inline-flex items-center rounded px-1.5 py-0.5 bg-violet-500/15 text-violet-700 dark:text-violet-400">
                  合集
                </span>
                <span v-if="item.parsed?.quality" class="text-muted-foreground">{{ item.parsed.quality }}</span>
                <span v-if="item.parsed?.source" class="text-muted-foreground">· {{ item.parsed.source }}</span>
                <span v-if="item.parsed?.lang?.length" class="text-muted-foreground">
                  · {{ item.parsed.lang.map(l => langLabel(l)).join('/') }}
                </span>
                <span class="ml-auto text-muted-foreground inline-flex items-center gap-2">
                  <span class="uppercase text-[10px] px-1.5 py-0.5 rounded bg-secondary">{{ item.source_name }}</span>
                </span>
              </div>

              <!-- title -->
              <div class="text-sm line-clamp-2" :title="item.title">{{ item.title }}</div>

              <!-- info row -->
              <div class="flex items-center gap-3 text-xs text-muted-foreground">
                <span>{{ formatSize(item.size) }}</span>
                <span v-if="item.seeders > 0">👥 {{ item.seeders }}</span>
                <span v-if="item.leechers > 0">⬇ {{ item.leechers }}</span>
                <span v-if="item.pub_date">{{ formatDate(item.pub_date) }}</span>
                <a v-if="item.detail_url" :href="item.detail_url" target="_blank" rel="noopener"
                  class="ml-auto underline hover:text-foreground">查看详情</a>
              </div>
            </div>

            <!-- actions -->
            <div class="shrink-0 flex flex-col gap-1.5">
              <button
                class="h-8 px-3 rounded-md bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 disabled:opacity-50 whitespace-nowrap"
                @click="subscribeAndDownload(item)"
                :disabled="actingHash === item.info_hash"
              >{{ actingHash === item.info_hash ? '处理中...' : '追番并下载' }}</button>
              <button
                class="h-8 px-3 rounded-md border border-input bg-background text-xs font-medium hover:bg-accent disabled:opacity-50 whitespace-nowrap"
                @click="downloadOnly(item)"
                :disabled="actingHash === item.info_hash"
              >仅下载</button>
            </div>
          </div>
        </div>
      </div>
      <div v-else-if="searched" class="py-12 text-center text-muted-foreground">未找到相关资源</div>
      <div v-else class="py-12 text-center text-muted-foreground">输入关键词开始搜索</div>
    </template>

    <!-- Stream search tab -->
    <template v-if="activeTab === 'stream'">
      <!-- Stream search bar -->
      <div class="max-w-2xl mx-auto mb-8">
        <div class="flex gap-3">
          <select
            v-model="selectedRule"
            class="h-12 rounded-lg border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring min-w-[160px]"
          >
            <option value="">所有规则</option>
            <option v-for="rule in streamRules" :key="rule.id" :value="rule.id">{{ rule.name }}</option>
          </select>
          <div class="relative flex-1">
            <n-icon size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"><SearchOutline /></n-icon>
            <input
              v-model="streamKeyword"
              class="h-12 w-full rounded-lg border border-input bg-background pl-10 pr-4 text-base placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
              placeholder="输入关键词搜索流媒体资源..."
              @keydown.enter="doStreamSearch"
            />
          </div>
          <button
            class="h-12 px-6 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors disabled:opacity-50"
            @click="doStreamSearch"
            :disabled="streamSearching"
          >搜索</button>
        </div>
        <p v-if="!streamRules.length" class="mt-2 text-xs text-amber-600 dark:text-amber-400">
          还没有启用的流媒体规则。请先到 <a class="underline cursor-pointer" @click="$router.push('/stream-rules')">流媒体规则</a> 添加并启用至少一个。
        </p>
      </div>

      <!-- Stream results -->
      <div v-if="streamSearching" class="flex justify-center py-12">
        <n-spin size="large" />
      </div>
      <div v-else-if="streamResults.length" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 max-w-5xl mx-auto">
        <div
          v-for="(item, idx) in streamResults"
          :key="(item.rule_name || '') + '|' + (item.url || item.detail_url) + '|' + idx"
          class="bg-card rounded-lg border p-4 hover:border-primary/50 transition-colors"
        >
          <div class="text-sm font-medium line-clamp-2 mb-2" :title="item.name">{{ item.name }}</div>
          <div class="flex flex-wrap gap-1 mb-3">
            <span v-if="item.rule_name" class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-secondary text-secondary-foreground">{{ item.rule_name }}</span>
            <span v-if="item.year" class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-secondary text-secondary-foreground">{{ item.year }}</span>
          </div>
          <button
            class="w-full h-8 rounded-md bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 disabled:opacity-50"
            @click="subscribeStream(item)"
            :disabled="actingHash === streamKey(item)"
          >{{ actingHash === streamKey(item) ? '处理中...' : '追番并下载' }}</button>
        </div>
      </div>
      <div v-else-if="streamSearched" class="py-12 text-center text-muted-foreground">未找到相关流媒体资源</div>
      <div v-else class="py-12 text-center text-muted-foreground">输入关键词开始搜索（默认搜索所有启用的规则）</div>
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage, NIcon, NSpin } from 'naive-ui'
import { get, post } from '@/utils/api'
import { SearchOutline } from '@vicons/ionicons5'
import PageHeader from '@/components/Common/PageHeader.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()

const tabs = [
  { key: 'torrent', label: 'BT 资源' },
  { key: 'stream', label: '流媒体资源' },
]
const activeTab = ref('torrent')

// Torrent (indexer) search state
const keyword = ref('')
const searching = ref(false)
const searched = ref(false)
const results = ref([])
const actingHash = ref(null)
const selectedIndexers = ref(['mikan', 'dmhy', 'bangumimoe'])

const indexerOptions = [
  { label: 'Mikan', value: 'mikan' },
  { label: 'DMHY', value: 'dmhy' },
  { label: 'BangumiMoe', value: 'bangumimoe' },
  { label: 'Nyaa', value: 'nyaa' },
]

// Stream search state
const streamKeyword = ref('')
const selectedRule = ref('')
const streamRules = ref([])
const streamSearching = ref(false)
const streamSearched = ref(false)
const streamResults = ref([])

onMounted(() => {
  if (route.query.q) {
    keyword.value = route.query.q
    doSearch()
  }
  fetchStreamRules()
})

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

function streamKey(item) {
  return (item.rule_name || '') + '|' + (item.url || item.detail_url || '')
}

async function fetchStreamRules() {
  try {
    const data = await get('/stream-rules', { params: { enabled: true } })
    streamRules.value = data.items || data || []
  } catch (e) {
    // silently fail
  }
}

async function doSearch() {
  if (!keyword.value.trim()) {
    message.warning('请输入搜索关键词')
    return
  }
  if (!selectedIndexers.value.length) {
    message.warning('至少选择一个站点')
    return
  }
  searching.value = true
  searched.value = true
  try {
    const resp = await post('/indexer/search', {
      keyword: keyword.value.trim(),
      indexers: selectedIndexers.value,
    })
    results.value = resp.candidates || []
  } catch (e) {
    message.error(e.message || '搜索失败')
    results.value = []
  } finally {
    searching.value = false
  }
}

async function doStreamSearch() {
  if (!streamKeyword.value.trim()) {
    message.warning('请输入搜索关键词')
    return
  }
  streamSearching.value = true
  streamSearched.value = true
  try {
    const params = { keyword: streamKeyword.value }
    if (selectedRule.value) params.rule_id = selectedRule.value
    const data = await get('/stream/search', { params })
    if (Array.isArray(data)) {
      streamResults.value = data
    } else {
      streamResults.value = data?.results || data?.items || []
    }
  } catch (e) {
    message.error('流媒体搜索失败')
    streamResults.value = []
  } finally {
    streamSearching.value = false
  }
}

// 追这部番并下载指定的 BT 资源：
//   1. 用解析出的 anime_name（或用户输入的 keyword）调 Bangumi 搜索拿 bangumi_id
//   2. 调 /bangumi/:id/subscribe 创建/订阅本地 anime
//   3. 用拿到的 anime_id + magnet 创建 download 任务
async function subscribeAndDownload(item) {
  const url = item.magnet_url || item.torrent_url
  if (!url) {
    message.error('该候选缺少 magnet / torrent URL')
    return
  }
  actingHash.value = item.info_hash
  try {
    const animeName = item.parsed?.anime_name || keyword.value.trim()
    let animeId = null

    // 尝试 Bangumi 搜索 → 订阅
    try {
      const bgmResp = await get('/bangumi/search', { params: { keyword: animeName } })
      const list = bgmResp?.results || bgmResp?.items || []
      if (list.length) {
        const subResp = await post(`/bangumi/${list[0].id}/subscribe`)
        animeId = subResp?.anime_id || null
      }
    } catch (e) {
      // 失败也没事，仅下载即可
    }

    const ep = item.parsed?.episode_num || null
    await post('/downloads/', {
      url,
      title: item.title,
      name: item.title,
      download_type: 'torrent',
      source: 'bt',
      anime_id: animeId || undefined,
      episode_number: ep || undefined,
    })

    if (animeId) {
      message.success('已追番并加入下载队列')
    } else {
      message.success('已加入下载队列（未匹配到 Bangumi 番剧条目）')
    }
  } catch (e) {
    message.error(e.message || '操作失败')
  } finally {
    actingHash.value = null
  }
}

async function downloadOnly(item) {
  const url = item.magnet_url || item.torrent_url
  if (!url) {
    message.error('该候选缺少 magnet / torrent URL')
    return
  }
  actingHash.value = item.info_hash
  try {
    const ep = item.parsed?.episode_num || null
    await post('/downloads/', {
      url,
      title: item.title,
      name: item.title,
      download_type: 'torrent',
      source: 'bt',
      episode_number: ep || undefined,
    })
    message.success('已加入下载队列')
  } catch (e) {
    message.error(e.message || '下载失败')
  } finally {
    actingHash.value = null
  }
}

// 流媒体追番并下载
async function subscribeStream(item) {
  actingHash.value = streamKey(item)
  try {
    const animeName = item.name
    let animeId = null
    try {
      const bgmResp = await get('/bangumi/search', { params: { keyword: animeName } })
      const list = bgmResp?.results || bgmResp?.items || []
      if (list.length) {
        const subResp = await post(`/bangumi/${list[0].id}/subscribe`)
        animeId = subResp?.anime_id || null
      }
    } catch (e) {
      // ignore
    }

    if (!animeId) {
      message.warning('没匹配到 Bangumi 番剧条目，无法触发流媒体下载')
      return
    }

    // 流媒体一般是整季/整页爬，先订阅让 Orchestrator 接手
    message.success('已追番，Orchestrator 会自动从该流媒体源下载')
  } catch (e) {
    message.error(e.message || '操作失败')
  } finally {
    actingHash.value = null
  }
}
</script>
