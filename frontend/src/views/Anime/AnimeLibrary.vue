<template>
  <div>
    <PageHeader :title="hasFilters ? '筛选结果' : '热门番剧'" :subtitle="hasFilters ? '多维发现番剧' : '来自 Bangumi 的实时热门趋势'" />

    <!-- 筛选面板 -->
    <div class="bg-card rounded-lg border p-4 mb-6 space-y-4">
      <!-- 搜索 -->
      <div class="flex gap-3">
        <div class="relative flex-1">
          <n-icon size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"><SearchOutline /></n-icon>
          <input v-model="filters.keyword" type="text" placeholder="搜索番剧名..."
            class="h-10 w-full rounded-md border border-input bg-background pl-9 pr-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
            @keydown.enter="discover" />
        </div>
        <button class="bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-10 px-6 text-sm font-medium transition-colors"
          @click="discover" :disabled="loading">
          {{ loading ? '加载中...' : '筛选' }}
        </button>
      </div>

      <!-- 筛选维度 -->
      <div class="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
        <!-- 年份 -->
        <div>
          <label class="block text-xs text-muted-foreground mb-1">年份</label>
          <select v-model="filters.year" class="h-9 w-full rounded-md border border-input bg-background px-2 text-sm">
            <option :value="0">不限</option>
            <option v-for="y in years" :key="y" :value="y">{{ y }}</option>
          </select>
        </div>
        <!-- 季度（多选） -->
        <div>
          <label class="block text-xs text-muted-foreground mb-1">季度（可多选）</label>
          <div class="flex flex-wrap gap-1.5">
            <button v-for="s in seasonOptions" :key="s.value"
              type="button"
              class="px-2.5 py-1 rounded-full text-xs font-medium border transition-colors"
              :class="filters.seasons.includes(s.value)
                ? 'bg-primary text-primary-foreground border-primary'
                : 'bg-background text-muted-foreground border-border hover:border-primary/50'"
              :disabled="!filters.year"
              :style="!filters.year ? 'opacity:0.5;cursor:not-allowed' : ''"
              @click="toggleSeason(s.value)">
              {{ s.label }}
            </button>
          </div>
        </div>
        <!-- 排序 -->
        <div>
          <label class="block text-xs text-muted-foreground mb-1">排序</label>
          <select v-model="filters.sort" class="h-9 w-full rounded-md border border-input bg-background px-2 text-sm">
            <option value="heat">热度</option>
            <option value="rank">排名</option>
            <option value="score">评分</option>
            <option value="match">匹配度</option>
          </select>
        </div>
        <!-- 评分门槛 -->
        <div>
          <label class="block text-xs text-muted-foreground mb-1">最低评分</label>
          <select v-model.number="filters.min_rating" class="h-9 w-full rounded-md border border-input bg-background px-2 text-sm">
            <option :value="0">不限</option>
            <option :value="6">6+ 及格</option>
            <option :value="7">7+ 良好</option>
            <option :value="7.5">7.5+ 推荐</option>
            <option :value="8">8+ 高分</option>
            <option :value="8.5">8.5+ 神作</option>
          </select>
        </div>
      </div>

      <!-- 标签多选 -->
      <div>
        <label class="block text-xs text-muted-foreground mb-2">标签（点击切换）</label>
        <div class="flex flex-wrap gap-2">
          <button v-for="tag in tagOptions" :key="tag"
            class="px-3 py-1 rounded-full text-xs font-medium border transition-colors"
            :class="filters.tags.includes(tag)
              ? 'bg-primary text-primary-foreground border-primary'
              : 'bg-background text-muted-foreground border-border hover:border-primary/50'"
            @click="toggleTag(tag)">
            {{ tag }}
          </button>
        </div>
      </div>

      <!-- 快捷操作 -->
      <div class="flex items-center justify-between text-xs text-muted-foreground pt-1 border-t">
        <button class="hover:text-foreground" @click="resetFilters">重置筛选</button>
        <span v-if="total">共 {{ total }} 部</span>
      </div>
    </div>

    <!-- 结果列表 -->
    <div v-if="loading" class="flex justify-center py-12"><n-spin size="large" /></div>
    <div v-else-if="results.length" class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8 gap-3">
      <AnimeCard v-for="item in results" :key="item.id" :item="item"
        @click="goToDetail(item)" @subscribe="subscribeBangumi(item)" />
    </div>
    <div v-else class="py-16 text-center text-sm text-muted-foreground">暂无匹配结果</div>

    <!-- 分页 -->
    <div v-if="total > pageSize" class="flex justify-center items-center gap-3 mt-6 text-sm">
      <button class="h-8 px-3 rounded-md border border-input bg-background hover:bg-accent transition-colors disabled:opacity-50"
        :disabled="page <= 1" @click="changePage(page - 1)">上一页</button>
      <span class="text-muted-foreground">第 {{ page }} 页 / 共 {{ totalPages }} 页</span>
      <button class="h-8 px-3 rounded-md border border-input bg-background hover:bg-accent transition-colors disabled:opacity-50"
        :disabled="page >= totalPages" @click="changePage(page + 1)">下一页</button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useMessage, NIcon, NSpin } from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'
import { get, post } from '@/utils/api'
import PageHeader from '@/components/Common/PageHeader.vue'
import AnimeCard from './AnimeCard.vue'

const router = useRouter()
const route = useRoute()
const message = useMessage()

const pageSize = 24
const page = ref(1)
const total = ref(0)
const loading = ref(false)
const results = ref([])

const currentYear = new Date().getFullYear()
const years = Array.from({ length: 30 }, (_, i) => currentYear - i)
const tagOptions = ['日常', '原创', '校园', '搞笑', '奇幻', '百合', '恋爱', '悬疑', '热血', '后宫', '机战', '轻改', '偶像', '治愈', '异世界']
const seasonOptions = [
  { label: '冬番（1-3月）', value: 'winter' },
  { label: '春番（4-6月）', value: 'spring' },
  { label: '夏番（7-9月）', value: 'summer' },
  { label: '秋番（10-12月）', value: 'autumn' },
]

const filters = ref({
  keyword: '',
  sort: 'heat',
  year: 0,
  seasons: [], // 多选
  tags: [],
  min_rating: 0
})

// 从 URL query 还原状态（返回时，路由 query 恢复即可还原筛选+页码）
function restoreFromQuery() {
  const q = route.query
  // tag（旧兼容，搜索详情页点击标签会传）
  const legacyTag = typeof q.tag === 'string' ? q.tag : ''
  // 多值 query 可能是 string 或 string[]
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

// 把当前 filters + page 同步到 URL（router.replace，不污染 history）
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

onMounted(() => {
  restoreFromQuery()
  discover()
})

// 判断是否有筛选条件（用来决定走 trending 还是 discover）
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
      // 无筛选：热门趋势（Kazumi 首页同款）
      const resp = await get('/bangumi/trending', {
        params: { limit: pageSize, offset: (page.value - 1) * pageSize }
      })
      results.value = resp.results || []
      total.value = resp.total || 0
    } else {
      // 有筛选：多维发现
      const resp = await post('/bangumi/discover', {
        ...filters.value,
        limit: pageSize,
        offset: (page.value - 1) * pageSize
      })
      results.value = resp.results || []
      total.value = resp.total || 0
    }
  } catch (e) {
    message.error('加载失败')
    results.value = []
  } finally {
    loading.value = false
  }
}

function changePage(p) {
  page.value = p
  discover()
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function subscribeBangumi(item) {
  try {
    await post(`/bangumi/${item.id}/subscribe`)
    message.success('追番成功')
    item.is_subscribed = true
  } catch (e) {
    message.error(e.message || '追番失败')
  }
}

function goToDetail(item) {
  router.push(`/anime-library/${item.id}`)
}

</script>
