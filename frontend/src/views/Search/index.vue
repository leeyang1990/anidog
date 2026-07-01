<template>
  <div>
    <AcPageHeader title="🔍 资源搜索" subtitle="跨站点搜索可下载的番剧资源" />

    <AcTabs v-model="activeTab" :tabs="tabs" />

    <!-- BT -->
    <template v-if="activeTab === 'torrent'">
      <div class="max-w-3xl mx-auto mb-4 mt-4">
        <div class="flex gap-2">
          <div class="flex-1">
            <AcInput v-model="keyword" placeholder="输入番剧名搜索 BT 资源..." size="lg" @keyup-enter="doSearch">
              <template #prefix><SearchOutline class="size-4" /></template>
            </AcInput>
          </div>
          <AcButton variant="primary" size="lg" :loading="searching" @click="doSearch">搜索</AcButton>
        </div>

        <div class="flex flex-wrap gap-2 items-center mt-3">
          <span class="text-xs text-muted-foreground font-bold">站点：</span>
          <button v-for="ix in indexerOptions" :key="ix.value"
            type="button"
            class="px-3 py-1 rounded-full text-xs font-bold border-2 transition-all"
            :class="selectedIndexers.includes(ix.value)
              ? 'bg-ac-grass text-white border-ac-grass-dark shadow-sm'
              : 'bg-card text-muted-foreground border-ac-sand-dark hover:border-ac-grass'"
            @click="toggleIndexer(ix.value)">{{ ix.label }}</button>
        </div>
      </div>

      <div v-if="searching" class="flex justify-center py-12"><AcSpinner :size="48" /></div>
      <div v-else-if="results.length" class="space-y-2 max-w-5xl mx-auto">
        <AcCard v-for="(item, idx) in results" :key="item.info_hash || idx" hoverable padding="sm" rounded="2xl">
          <div class="flex items-start gap-3">
            <div class="shrink-0 size-9 rounded-2xl bg-ac-sand text-ac-wood-dark flex items-center justify-center text-xs font-num font-bold">
              {{ idx + 1 }}
            </div>
            <div class="flex-1 min-w-0 space-y-1.5">
              <div class="flex items-center gap-1.5 flex-wrap text-xs">
                <AcTag v-if="item.parsed?.group" variant="grass">{{ item.parsed.group }}</AcTag>
                <AcTag v-if="item.parsed?.episode_num" variant="leaf">EP {{ String(item.parsed.episode_num).padStart(2,'0') }}</AcTag>
                <AcTag v-if="item.parsed?.is_batch" variant="sun">合集</AcTag>
                <span v-if="item.parsed?.quality" class="text-muted-foreground">{{ item.parsed.quality }}</span>
                <span v-if="item.parsed?.source" class="text-muted-foreground">· {{ item.parsed.source }}</span>
                <span v-if="item.parsed?.lang?.length" class="text-muted-foreground">· {{ item.parsed.lang.map(l => langLabel(l)).join('/') }}</span>
                <span class="ml-auto text-[10px] uppercase font-bold text-ac-wood-dark px-2 py-0.5 rounded-full bg-ac-sand">{{ item.source_name }}</span>
              </div>
              <div class="text-sm leading-relaxed line-clamp-2" :title="item.title">{{ item.title }}</div>
              <div class="flex items-center gap-3 text-xs text-muted-foreground font-num">
                <span>{{ formatSize(item.size) }}</span>
                <span v-if="item.seeders > 0">👥 {{ item.seeders }}</span>
                <span v-if="item.leechers > 0">⬇ {{ item.leechers }}</span>
                <span v-if="item.pub_date">{{ formatDate(item.pub_date) }}</span>
                <a v-if="item.detail_url" :href="item.detail_url" target="_blank" rel="noopener" class="ml-auto text-ac-grass-dark font-bold hover:underline">查看详情 →</a>
              </div>
            </div>
            <div class="shrink-0 flex flex-col gap-1.5">
              <AcButton size="sm" variant="primary" :loading="actingHash === item.info_hash" @click="subscribeAndDownload(item)">追番并下载</AcButton>
              <AcButton size="sm" variant="outline" :loading="actingHash === item.info_hash" @click="downloadOnly(item)">仅下载</AcButton>
            </div>
          </div>
        </AcCard>
      </div>
      <AcEmpty v-else-if="searched" title="未找到相关资源" description="试试换个关键词，或选择更多站点 🌿" class="py-8" />
      <AcEmpty v-else title="开始搜索吧" description="输入番剧名，从 Mikan / DMHY / BangumiMoe 等站点查找资源" class="py-8" />
    </template>

    <!-- Stream -->
    <template v-if="activeTab === 'stream'">
      <div class="max-w-3xl mx-auto mb-6 mt-4">
        <div class="flex gap-2">
          <AcSelect v-model="selectedRule" :options="ruleOptions" :block="false" size="lg" placeholder="所有规则" class="!w-44" />
          <div class="flex-1">
            <AcInput v-model="streamKeyword" placeholder="输入关键词搜索流媒体资源..." size="lg" @keyup-enter="doStreamSearch">
              <template #prefix><SearchOutline class="size-4" /></template>
            </AcInput>
          </div>
          <AcButton variant="primary" size="lg" :loading="streamSearching" @click="doStreamSearch">搜索</AcButton>
        </div>
        <p v-if="!streamRules.length" class="mt-2 text-xs text-ac-sun-dark">
          还没有启用的流媒体规则。请先到 <a class="underline cursor-pointer font-bold" @click="$router.push('/stream-rules')">流媒体规则</a> 添加并启用至少一个。
        </p>
      </div>

      <div v-if="streamSearching" class="flex justify-center py-12"><AcSpinner :size="48" /></div>
      <div v-else-if="streamResults.length" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 max-w-5xl mx-auto">
        <AcCard v-for="(item, idx) in streamResults" :key="streamKey(item) + '|' + idx" hoverable padding="md" rounded="2xl">
          <div class="text-sm font-bold line-clamp-2 mb-2" :title="item.name">{{ item.name }}</div>
          <div class="flex flex-wrap gap-1 mb-3">
            <AcTag v-if="item.rule_name" variant="grass">{{ item.rule_name }}</AcTag>
            <AcTag v-if="item.year" variant="default">{{ item.year }}</AcTag>
          </div>
          <AcButton size="sm" variant="primary" block :loading="actingHash === streamKey(item)" @click="subscribeStream(item)">追番并下载</AcButton>
        </AcCard>
      </div>
      <AcEmpty v-else-if="streamSearched" title="未找到相关流媒体资源" class="py-8" />
      <AcEmpty v-else title="开始搜索吧" description="默认搜索所有启用的规则" class="py-8" />
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from '../../composables/useToast'
import { get, post } from '@/utils/api'
import { SearchOutline } from '@vicons/ionicons5'
import { AcPageHeader, AcInput, AcButton, AcSpinner, AcCard, AcTag, AcEmpty, AcSelect, AcTabs } from '../../components/ac'

const route = useRoute()
const router = useRouter()
const toast = useToast()

const tabs = [
  { key: 'torrent', label: 'BT 资源' },
  { key: 'stream', label: '流媒体资源' },
]
const activeTab = ref('torrent')

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

const streamKeyword = ref('')
const selectedRule = ref('')
const streamRules = ref([])
const streamSearching = ref(false)
const streamSearched = ref(false)
const streamResults = ref([])

const ruleOptions = computed(() => [
  { label: '所有规则', value: '' },
  ...streamRules.value.map(r => ({ label: r.name, value: r.id })),
])

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
function langLabel(l) { return { simplified: '简中', traditional: '繁中', japanese: '日文', english: '英文' }[l] || l }
function formatSize(b) {
  if (!b) return '—'
  const u = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(Math.floor(Math.log(b) / Math.log(1024)), u.length - 1)
  return (b / Math.pow(1024, i)).toFixed(1) + ' ' + u[i]
}
function formatDate(s) {
  if (!s) return ''
  const d = new Date(s)
  return isNaN(d.getTime()) ? '' : d.toLocaleDateString('zh-CN')
}
function streamKey(item) { return (item.rule_name || '') + '|' + (item.url || item.detail_url || '') }

async function fetchStreamRules() {
  try {
    const data = await get('/stream-rules', { params: { enabled: true } })
    streamRules.value = data.items || data || []
  } catch (e) { /* silent */ }
}

async function doSearch() {
  if (!keyword.value.trim()) return toast.warning('请输入搜索关键词')
  if (!selectedIndexers.value.length) return toast.warning('至少选择一个站点')
  searching.value = true; searched.value = true
  try {
    const resp = await post('/indexer/search', { keyword: keyword.value.trim(), indexers: selectedIndexers.value })
    results.value = resp.candidates || []
  } catch (e) {
    toast.error(e.message || '搜索失败')
    results.value = []
  } finally { searching.value = false }
}

async function doStreamSearch() {
  if (!streamKeyword.value.trim()) return toast.warning('请输入搜索关键词')
  streamSearching.value = true; streamSearched.value = true
  try {
    const params = { keyword: streamKeyword.value }
    if (selectedRule.value) params.rule_id = selectedRule.value
    const data = await get('/stream/search', { params })
    streamResults.value = Array.isArray(data) ? data : (data?.results || data?.items || [])
  } catch (e) {
    toast.error('流媒体搜索失败')
    streamResults.value = []
  } finally { streamSearching.value = false }
}

async function subscribeAndDownload(item) {
  const url = item.magnet_url || item.torrent_url
  if (!url) return toast.error('该候选缺少 magnet / torrent URL')
  actingHash.value = item.info_hash
  try {
    const animeName = item.parsed?.anime_name || keyword.value.trim()
    let animeId = null
    try {
      const bgmResp = await get('/bangumi/search', { params: { keyword: animeName } })
      const list = bgmResp?.results || bgmResp?.items || []
      if (list.length) {
        const subResp = await post(`/bangumi/${list[0].id}/subscribe`)
        animeId = subResp?.anime_id || null
      }
    } catch (e) { /* silent */ }
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
    toast.success(animeId ? '已追番并加入下载队列' : '已加入下载队列（未匹配到番剧）')
  } catch (e) {
    toast.error(e.message || '操作失败')
  } finally { actingHash.value = null }
}

async function downloadOnly(item) {
  const url = item.magnet_url || item.torrent_url
  if (!url) return toast.error('该候选缺少 magnet / torrent URL')
  actingHash.value = item.info_hash
  try {
    const ep = item.parsed?.episode_num || null
    await post('/downloads/', { url, title: item.title, name: item.title, download_type: 'torrent', source: 'bt', episode_number: ep || undefined })
    toast.success('已加入下载队列')
  } catch (e) {
    toast.error(e.message || '下载失败')
  } finally { actingHash.value = null }
}

async function subscribeStream(item) {
  actingHash.value = streamKey(item)
  try {
    let animeId = null
    try {
      const bgmResp = await get('/bangumi/search', { params: { keyword: item.name } })
      const list = bgmResp?.results || bgmResp?.items || []
      if (list.length) {
        const subResp = await post(`/bangumi/${list[0].id}/subscribe`)
        animeId = subResp?.anime_id || null
      }
    } catch (e) { /* silent */ }
    if (!animeId) {
      toast.warning('没匹配到番剧条目，无法触发流媒体下载')
      return
    }
    toast.success('已追番，Orchestrator 会自动从该流媒体源下载')
  } catch (e) {
    toast.error(e.message || '操作失败')
  } finally { actingHash.value = null }
}
</script>
