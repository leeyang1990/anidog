<template>
  <div>
    <AcPageHeader :title="hasFilters ? t('pages.library.filteredTitle') : t('pages.library.trendingTitle')" :subtitle="hasFilters ? t('pages.library.filteredSubtitle') : t('pages.library.trendingSubtitle')" />

    <!-- 筛选面板 -->
    <AcCard padding="md" rounded="2xl" class="mb-6">
      <div class="space-y-4">
        <!-- 搜索 -->
        <div class="flex gap-3">
          <div class="flex-1">
            <AcInput v-model="filters.keyword" :placeholder="t('pages.library.search')" size="lg" @keyup-enter="discover">
              <template #prefix><SearchOutline class="size-4" /></template>
            </AcInput>
          </div>
          <AcButton variant="primary" size="lg" :loading="loading" @click="discover">
            {{ loading ? t('pages.library.loading') : t('pages.library.filter') }}
          </AcButton>
        </div>

        <!-- 筛选维度 -->
        <div class="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
          <div>
            <label class="block text-xs text-muted-foreground mb-1 font-bold">{{ t('pages.library.year') }}</label>
            <AcSelect v-model="filters.year" :options="yearOptions" />
          </div>
          <div>
            <label class="block text-xs text-muted-foreground mb-1 font-bold">{{ t('pages.library.seasons') }}</label>
            <div class="flex flex-wrap gap-1.5">
              <button v-for="s in seasonOptions" :key="s.value" type="button"
                class="px-2.5 py-1 rounded-full text-xs font-bold border-2 transition-colors"
                :class="filters.seasons.includes(s.value)
                  ? 'bg-ac-grass text-white border-ac-grass-dark'
                  : 'bg-card border-ac-sand text-muted-foreground hover:border-ac-grass'"
                :disabled="!filters.year"
                :style="!filters.year ? 'opacity:0.5;cursor:not-allowed' : ''"
                @click="toggleSeason(s.value)">
                {{ s.label }}
              </button>
            </div>
          </div>
          <div>
            <label class="block text-xs text-muted-foreground mb-1 font-bold">{{ t('pages.library.sort') }}</label>
            <AcSelect v-model="filters.sort" :options="sortOptions" />
          </div>
          <div>
            <label class="block text-xs text-muted-foreground mb-1 font-bold">{{ t('pages.library.minRating') }}</label>
            <AcSelect v-model="filters.min_rating" :options="ratingOptions" />
          </div>
        </div>

        <!-- 标签多选 -->
        <div>
          <label class="block text-xs text-muted-foreground mb-2 font-bold">{{ t('pages.library.tags') }}</label>
          <div class="flex flex-wrap gap-2">
            <button v-for="tag in tagOptions" :key="tag" type="button"
              class="px-3 py-1 rounded-full text-xs font-bold border-2 transition-colors"
              :class="filters.tags.includes(tag)
                ? 'bg-ac-grass text-white border-ac-grass-dark'
                : 'bg-card border-ac-sand text-muted-foreground hover:border-ac-grass'"
              @click="toggleTag(tag)">
              {{ tag }}
            </button>
          </div>
        </div>

        <div class="flex items-center justify-between text-xs text-muted-foreground pt-2 border-t-2 border-dashed border-ac-sand">
          <button type="button" class="hover:text-foreground font-bold" @click="resetFilters">{{ t('pages.library.reset') }}</button>
          <span v-if="total" class="font-num font-bold">{{ t('pages.library.count', { count: total }) }}</span>
        </div>
      </div>
    </AcCard>

    <!-- 结果列表 -->
    <div v-if="loading" class="flex justify-center py-12"><AcSpinner :size="48" /></div>
    <div v-else-if="results.length" class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8 gap-3">
      <AnimeCard v-for="item in results" :key="item.id" :item="item"
        @click="goToDetail(item)" @subscribe="subscribeBangumi(item)" />
    </div>
    <AcEmpty v-else :title="t('pages.library.empty')" :description="t('pages.library.emptyDesc')" class="py-12" />

    <!-- 分页 -->
    <div v-if="total > pageSize" class="flex justify-center items-center gap-3 mt-6 text-sm">
      <AcButton size="sm" variant="outline" :disabled="page <= 1" @click="changePage(page - 1)">{{ t('pages.library.previous') }}</AcButton>
      <span class="text-muted-foreground font-num font-bold">{{ t('pages.library.page', { page, total: totalPages }) }}</span>
      <AcButton size="sm" variant="outline" :disabled="page >= totalPages" @click="changePage(page + 1)">{{ t('pages.library.next') }}</AcButton>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { SearchOutline } from '@vicons/ionicons5'
import { get, post } from '@/utils/api'
import { AcPageHeader, AcCard, AcInput, AcButton, AcSelect, AcSpinner, AcEmpty } from '@/components/ac'
import AnimeCard from './AnimeCard.vue'
import { useI18n } from 'vue-i18n'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { t } = useI18n({ useScope: 'global' })

const pageSize = 24
const page = ref(1)
const total = ref(0)
const loading = ref(false)
const results = ref([])

const currentYear = new Date().getFullYear()
const years = Array.from({ length: 30 }, (_, i) => currentYear - i)

const yearOptions = computed(() => [
  { label: t('pages.library.unlimited'), value: 0 },
  ...years.map(y => ({ label: String(y), value: y })),
])

const sortOptions = computed(() => [
  { label: t('pages.library.heat'), value: 'heat' },
  { label: t('pages.library.rank'), value: 'rank' },
  { label: t('pages.library.score'), value: 'score' },
  { label: t('pages.library.match'), value: 'match' },
])

const ratingOptions = computed(() => [
  { label: t('pages.library.unlimited'), value: 0 },
  { label: '6+ 及格', value: 6 },
  { label: '7+ 良好', value: 7 },
  { label: '7.5+ 推荐', value: 7.5 },
  { label: '8+ 高分', value: 8 },
  { label: '8.5+ 神作', value: 8.5 },
])

const tagOptions = ['日常', '原创', '校园', '搞笑', '奇幻', '百合', '恋爱', '悬疑', '热血', '后宫', '机战', '轻改', '偶像', '治愈', '异世界']
const seasonOptions = [
  { label: '冬番（1-3月）', value: 'winter' },
  { label: '春番（4-6月）', value: 'spring' },
  { label: '夏番（7-9月）', value: 'summer' },
  { label: '秋番（10-12月）', value: 'autumn' },
]

const filters = ref({ keyword: '', sort: 'heat', year: 0, seasons: [], tags: [], min_rating: 0 })

function restoreFromQuery() {
  const q = route.query
  const legacyTag = typeof q.tag === 'string' ? q.tag : ''
  const asArray = (v) => (Array.isArray(v) ? v : v ? [v] : [])
  filters.value = {
    keyword: typeof q.keyword === 'string' ? q.keyword : '',
    sort: typeof q.sort === 'string' ? q.sort : 'heat',
    year: q.year ? Number(q.year) || 0 : 0,
    seasons: asArray(q.seasons),
    tags: asArray(q.tags).length ? asArray(q.tags) : (legacyTag ? [legacyTag] : []),
    min_rating: q.min_rating ? Number(q.min_rating) || 0 : 0,
  }
  page.value = q.page ? Number(q.page) || 1 : 1
}

function syncToQuery() {
  const f = filters.value
  const q = {}
  if (f.keyword) q.keyword = f.keyword
  if (f.sort && f.sort !== 'heat') q.sort = f.sort
  if (f.year) q.year = String(f.year)
  if (f.seasons.length) q.seasons = f.seasons
  if (f.tags.length) q.tags = f.tags
  if (f.min_rating) q.min_rating = String(f.min_rating)
  if (page.value > 1) q.page = String(page.value)
  router.replace({ query: q })
}

onMounted(() => { restoreFromQuery(); discover() })

const hasFilters = computed(() => {
  const f = filters.value
  return f.keyword || f.year || f.seasons.length > 0 || f.tags.length > 0 || f.min_rating > 0 || f.sort !== 'heat'
})

const totalPages = computed(() => Math.ceil(total.value / pageSize))

function toggleTag(tag) {
  const i = filters.value.tags.indexOf(tag)
  if (i >= 0) filters.value.tags.splice(i, 1)
  else filters.value.tags.push(tag)
}

function toggleSeason(s) {
  const i = filters.value.seasons.indexOf(s)
  if (i >= 0) filters.value.seasons.splice(i, 1)
  else filters.value.seasons.push(s)
}

function resetFilters() {
  filters.value = { keyword: '', sort: 'heat', year: 0, seasons: [], tags: [], min_rating: 0 }
  page.value = 1
  discover()
}

async function discover() {
  syncToQuery()
  loading.value = true
  try {
    if (!hasFilters.value) {
      const resp = await get('/bangumi/trending', {
        params: { limit: pageSize, offset: (page.value - 1) * pageSize }
      })
      results.value = resp.results || []
      total.value = resp.total || 0
    } else {
      const resp = await post('/bangumi/discover', {
        ...filters.value,
        limit: pageSize,
        offset: (page.value - 1) * pageSize
      })
      results.value = resp.results || []
      total.value = resp.total || 0
    }
  } catch (e) {
    toast.error(t('pages.library.loadFailed'))
    results.value = []
  } finally { loading.value = false }
}

function changePage(p) {
  page.value = p
  discover()
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function subscribeBangumi(item) {
  try {
    await post(`/bangumi/${item.id}/subscribe`)
    toast.success(t('pages.library.subscribed'))
    item.is_subscribed = true
  } catch (e) { toast.error(e.message || t('pages.library.subscribeFailed')) }
}

function goToDetail(item) { router.push(`/anime-library/${item.id}`) }
</script>
