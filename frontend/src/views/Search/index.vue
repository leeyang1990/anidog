<template>
  <div>
    <AcPageHeader :title="t('pages.search.title')" :subtitle="t('pages.search.subtitle')" />

    <AcTabs v-model="activeTab" :tabs="tabs" />

    <!-- BT -->
    <template v-if="activeTab === 'torrent'">
      <div class="max-w-3xl mx-auto mb-4 mt-4">
        <div class="flex gap-2">
          <div class="flex-1">
            <AcInput v-model="keyword" :placeholder="t('pages.search.btPlaceholder')" size="lg" @keyup-enter="doSearch">
              <template #prefix><SearchOutline class="size-4" /></template>
            </AcInput>
          </div>
          <AcButton variant="primary" size="lg" :loading="searching" @click="doSearch">{{ t('common.search') }}</AcButton>
        </div>

        <div class="flex flex-wrap gap-2 items-center mt-3">
          <span class="text-xs text-muted-foreground font-bold">{{ t('pages.search.sites') }}</span>
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
                <AcTag v-if="item.parsed?.is_batch" variant="sun">{{ t('pages.search.batch') }}</AcTag>
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
                <a v-if="item.detail_url" :href="item.detail_url" target="_blank" rel="noopener" class="ml-auto text-ac-grass-dark font-bold hover:underline">{{ t('pages.search.details') }}</a>
              </div>
            </div>
            <div class="shrink-0 flex flex-col gap-1.5">
              <AcButton size="sm" variant="primary" :loading="actingHash === item.info_hash" @click="subscribeAndDownload(item)">{{ t('pages.search.followDownload') }}</AcButton>
              <AcButton size="sm" variant="outline" :loading="actingHash === item.info_hash" @click="downloadOnly(item)">{{ t('pages.search.downloadOnly') }}</AcButton>
            </div>
          </div>
        </AcCard>
      </div>
      <AcEmpty v-else-if="searched" :title="t('pages.search.notFound')" :description="t('pages.search.tryMore')" class="py-8" />
      <AcEmpty v-else :title="t('pages.search.start')" :description="t('pages.search.startBtDesc')" class="py-8" />
    </template>

    <!-- Stream -->
    <template v-if="activeTab === 'stream'">
      <div class="max-w-3xl mx-auto mb-6 mt-4">
        <div class="flex gap-2">
          <AcSelect v-model="selectedRule" :options="ruleOptions" :block="false" size="lg" :placeholder="t('pages.search.allRules')" class="!w-44" />
          <div class="flex-1">
            <AcInput v-model="streamKeyword" :placeholder="t('pages.search.streamPlaceholder')" size="lg" @keyup-enter="doStreamSearch">
              <template #prefix><SearchOutline class="size-4" /></template>
            </AcInput>
          </div>
          <AcButton variant="primary" size="lg" :loading="streamSearching" @click="doStreamSearch">{{ t('common.search') }}</AcButton>
        </div>
        <p v-if="!streamRules.length" class="mt-2 text-xs text-ac-sun-dark">
          {{ t('pages.search.noRulesBefore') }} <a class="underline cursor-pointer font-bold" @click="$router.push('/stream-rules')">{{ t('pages.search.rulesLink') }}</a> {{ t('pages.search.noRulesAfter') }}
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
          <AcButton size="sm" variant="primary" block :loading="actingHash === streamKey(item)" @click="subscribeStream(item)">{{ t('pages.search.followDownload') }}</AcButton>
        </AcCard>
      </div>
      <AcEmpty v-else-if="streamSearched" :title="t('pages.search.noStream')" class="py-8" />
      <AcEmpty v-else :title="t('pages.search.start')" :description="t('pages.search.startStreamDesc')" class="py-8" />
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
import { useI18n } from 'vue-i18n'

const route = useRoute()
const router = useRouter()
const toast = useToast()
const { t, locale } = useI18n({ useScope: 'global' })

const tabs = computed(() => [
  { key: 'torrent', label: t('pages.search.btTab') },
  { key: 'stream', label: t('pages.search.streamTab') },
])
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
  { label: t('pages.search.allRules'), value: '' },
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
function langLabel(l) { return t(`pages.search.languages.${l}`, l) }
function formatSize(b) {
  if (!b) return '—'
  const u = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(Math.floor(Math.log(b) / Math.log(1024)), u.length - 1)
  return (b / Math.pow(1024, i)).toFixed(1) + ' ' + u[i]
}
function formatDate(s) {
  if (!s) return ''
  const d = new Date(s)
  return isNaN(d.getTime()) ? '' : d.toLocaleDateString(locale.value)
}
function streamKey(item) { return (item.rule_name || '') + '|' + (item.url || item.detail_url || '') }

async function fetchStreamRules() {
  try {
    const data = await get('/stream-rules', { params: { enabled: true } })
    streamRules.value = data.items || data || []
  } catch (e) { /* silent */ }
}

async function doSearch() {
  if (!keyword.value.trim()) return toast.warning(t('pages.search.enterKeyword'))
  if (!selectedIndexers.value.length) return toast.warning(t('pages.search.selectSite'))
  searching.value = true; searched.value = true
  try {
    const resp = await post('/indexer/search', { keyword: keyword.value.trim(), indexers: selectedIndexers.value })
    results.value = resp.candidates || []
  } catch (e) {
    toast.error(e.message || t('pages.search.searchFailed'))
    results.value = []
  } finally { searching.value = false }
}

async function doStreamSearch() {
  if (!streamKeyword.value.trim()) return toast.warning(t('pages.search.enterKeyword'))
  streamSearching.value = true; streamSearched.value = true
  try {
    const params = { keyword: streamKeyword.value }
    if (selectedRule.value) params.rule_id = selectedRule.value
    const data = await get('/stream/search', { params })
    streamResults.value = Array.isArray(data) ? data : (data?.results || data?.items || [])
  } catch (e) {
    toast.error(t('pages.search.streamSearchFailed'))
    streamResults.value = []
  } finally { streamSearching.value = false }
}

async function subscribeAndDownload(item) {
  const url = item.magnet_url || item.torrent_url
  if (!url) return toast.error(t('pages.search.missingUrl'))
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
    toast.success(animeId ? t('pages.search.queuedFollow') : t('pages.search.queuedUnmatched'))
  } catch (e) {
    toast.error(e.message || t('pages.search.operationFailed'))
  } finally { actingHash.value = null }
}

async function downloadOnly(item) {
  const url = item.magnet_url || item.torrent_url
  if (!url) return toast.error(t('pages.search.missingUrl'))
  actingHash.value = item.info_hash
  try {
    const ep = item.parsed?.episode_num || null
    await post('/downloads/', { url, title: item.title, name: item.title, download_type: 'torrent', source: 'bt', episode_number: ep || undefined })
    toast.success(t('pages.search.queued'))
  } catch (e) {
    toast.error(e.message || t('pages.search.downloadFailed'))
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
      toast.warning(t('pages.search.noAnimeMatch'))
      return
    }
    toast.success(t('pages.search.streamSubscribed'))
  } catch (e) {
    toast.error(e.message || t('pages.search.operationFailed'))
  } finally { actingHash.value = null }
}
</script>
