<template>
  <div>
    <PageHeader title="番剧搜索" subtitle="跨站点搜索番剧资源" />

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

    <!-- Torrent search tab -->
    <template v-if="activeTab === 'torrent'">
      <!-- Search bar -->
      <div class="max-w-2xl mx-auto mb-8">
        <div class="flex gap-3">
          <div class="relative flex-1">
            <n-icon size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"><SearchOutline /></n-icon>
            <input
              v-model="keyword"
              class="h-12 w-full rounded-lg border border-input bg-background pl-10 pr-4 text-base placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
              placeholder="输入番剧名称搜索..."
              @keydown.enter="doSearch"
            />
          </div>
          <select
            v-model="site"
            class="h-12 rounded-lg border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <option v-for="opt in siteOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
          <button
            class="h-12 px-6 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors disabled:opacity-50"
            @click="doSearch"
            :disabled="searching"
          >
            搜索
          </button>
        </div>
      </div>

      <!-- Results -->
      <div v-if="searching" class="flex justify-center py-12">
        <n-spin size="large" />
      </div>
      <div v-else-if="results.length" class="space-y-3">
        <div
          v-for="item in results"
          :key="item.url"
          class="bg-card rounded-lg border p-4 hover:bg-muted/50 transition-colors"
        >
          <div class="flex items-center justify-between gap-4">
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium truncate">{{ item.name }}</div>
              <div class="flex gap-2 mt-1">
                <span v-if="item.size" class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-secondary text-secondary-foreground">{{ item.size }}</span>
                <span v-if="item.seeders" class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">{{ item.seeders }} 做种</span>
              </div>
            </div>
            <div class="flex gap-2 shrink-0">
              <button
                class="h-8 px-3 rounded-md bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 transition-colors"
                @click="downloadTorrent(item)"
              >
                下载
              </button>
              <button
                class="h-8 px-3 rounded-md border border-input bg-background text-xs font-medium hover:bg-accent transition-colors"
                @click="collectTorrent(item)"
              >
                补全
              </button>
            </div>
          </div>
        </div>
      </div>
      <div v-else-if="searched" class="py-12 text-center text-muted-foreground">未找到相关番剧</div>
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
            <option value="">选择规则</option>
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
          >
            搜索
          </button>
        </div>
      </div>

      <!-- Stream results -->
      <div v-if="streamSearching" class="flex justify-center py-12">
        <n-spin size="large" />
      </div>
      <div v-else-if="streamResults.length" class="space-y-3">
        <div
          v-for="item in streamResults"
          :key="item.id || item.url"
          class="bg-card rounded-lg border p-4 hover:bg-muted/50 transition-colors cursor-pointer"
          @click="goStreamDetail(item)"
        >
          <div class="flex items-center justify-between gap-4">
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium truncate">{{ item.title || item.name }}</div>
              <div class="flex gap-2 mt-1">
                <span v-if="item.quality" class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-secondary text-secondary-foreground">{{ item.quality }}</span>
                <span v-if="item.episode" class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">第{{ item.episode }}话</span>
              </div>
            </div>
            <n-icon size="16" class="text-muted-foreground shrink-0"><ChevronForwardOutline /></n-icon>
          </div>
        </div>
      </div>
      <div v-else-if="streamSearched" class="py-12 text-center text-muted-foreground">未找到相关流媒体资源</div>
      <div v-else class="py-12 text-center text-muted-foreground">选择规则并输入关键词开始搜索</div>
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage, NIcon, NSpin } from 'naive-ui'
import { get, post } from '@/utils/api'
import { SearchOutline, ChevronForwardOutline } from '@vicons/ionicons5'
import PageHeader from '@/components/Common/PageHeader.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()

const tabs = [
  { key: 'torrent', label: '种子搜索' },
  { key: 'stream', label: '流媒体搜索' },
]
const activeTab = ref('torrent')

// Torrent search state
const keyword = ref('')
const site = ref('mikan')
const searching = ref(false)
const searched = ref(false)
const results = ref([])

const siteOptions = [
  { label: 'Mikan', value: 'mikan' },
  { label: 'DMHY', value: 'dmhy' },
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

async function fetchStreamRules() {
  try {
    const data = await get('/stream-rules', { params: { enabled: true } })
    streamRules.value = data.items || data || []
  } catch (e) {
    // silently fail - rules may not be configured
  }
}

async function doSearch() {
  if (!keyword.value.trim()) {
    message.warning('请输入搜索关键词')
    return
  }
  searching.value = true
  searched.value = true
  try {
    const data = await get('/search', {
      params: { keyword: keyword.value, site: site.value }
    })
    results.value = data.items || []
  } catch (e) {
    message.error('搜索失败')
    results.value = []
  } finally {
    searching.value = false
  }
}

async function doStreamSearch() {
  if (!selectedRule.value) {
    message.warning('请选择流媒体规则')
    return
  }
  if (!streamKeyword.value.trim()) {
    message.warning('请输入搜索关键词')
    return
  }
  streamSearching.value = true
  streamSearched.value = true
  try {
    const data = await get('/stream/search', {
      params: { keyword: streamKeyword.value, rule_id: selectedRule.value }
    })
    streamResults.value = data.items || []
  } catch (e) {
    message.error('流媒体搜索失败')
    streamResults.value = []
  } finally {
    streamSearching.value = false
  }
}

function goStreamDetail(item) {
  if (item.id) {
    router.push({ name: 'BangumiDetail', params: { id: item.id } })
  }
}

async function downloadTorrent(item) {
  try {
    await post('/downloads', {
      magnet_link: item.url,
      title: item.name
    })
    message.success('已添加到下载队列')
  } catch (e) {
    message.error('添加下载失败')
  }
}

async function collectTorrent(item) {
  try {
    await post('/search/collect', {
      title: item.name,
      url: item.url
    })
    message.success('已添加补全任务')
  } catch (e) {
    message.error('添加补全任务失败')
  }
}
</script>
