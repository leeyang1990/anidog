<template>
  <div v-if="loading" class="flex justify-center py-20">
    <n-spin size="large" />
  </div>

  <div v-else-if="detail">
    <button
      class="inline-flex items-center gap-1.5 h-9 px-3 mb-4 rounded-md border border-input bg-background text-sm font-medium hover:bg-accent transition-colors"
      @click="$router.back()"
    >
      &larr; 返回
    </button>

    <!-- 顶部：封面 + 基本信息 -->
    <div class="flex flex-col md:flex-row gap-6 mb-6">
      <div class="w-40 md:w-48 shrink-0 rounded-lg overflow-hidden bg-muted shadow-md mx-auto md:mx-0">
        <img v-if="detail.image" :src="toHighResImage(detail.image)" :alt="detail.name_cn || detail.name" class="w-full object-cover" />
      </div>
      <div class="flex-1 space-y-3">
        <div>
          <h1 class="text-2xl font-bold">{{ detail.name_cn || detail.name }}</h1>
          <p v-if="detail.name && detail.name !== detail.name_cn" class="text-sm text-muted-foreground mt-1">{{ detail.name }}</p>
        </div>

        <!-- 关键指标 -->
        <div class="flex flex-wrap items-center gap-4 text-sm">
          <span v-if="detail.rating_score" class="flex items-center gap-1">
            <span class="text-amber-500 font-bold text-lg">{{ detail.rating_score }}</span>
            <span class="text-muted-foreground">/ 10</span>
          </span>
          <span v-if="detail.rank" class="text-muted-foreground">#{{ detail.rank }}</span>
          <span v-if="detail.platform" class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-primary/10 text-primary">{{ detail.platform }}</span>
          <span v-for="tag in (detail.tags || []).slice(0, 5)" :key="tag"
            class="inline-flex items-center rounded-full px-2 py-0.5 text-xs bg-secondary text-secondary-foreground">{{ tag }}</span>
        </div>

        <!-- 快速信息（从 infobox 抽取常用几项）-->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-1.5 text-sm">
          <div v-if="quickInfo.airDate" class="flex">
            <span class="text-muted-foreground w-20 shrink-0">放送日期</span>
            <span>{{ quickInfo.airDate }}</span>
          </div>
          <div v-if="quickInfo.airWeekday" class="flex">
            <span class="text-muted-foreground w-20 shrink-0">放送星期</span>
            <span>{{ quickInfo.airWeekday }}</span>
          </div>
          <div v-if="quickInfo.episodes" class="flex">
            <span class="text-muted-foreground w-20 shrink-0">话数</span>
            <span>{{ quickInfo.episodes }}</span>
          </div>
          <div v-if="quickInfo.country" class="flex">
            <span class="text-muted-foreground w-20 shrink-0">国家/地区</span>
            <span>{{ quickInfo.country }}</span>
          </div>
          <div v-if="quickInfo.director" class="flex">
            <span class="text-muted-foreground w-20 shrink-0">导演</span>
            <span>{{ quickInfo.director }}</span>
          </div>
          <div v-if="quickInfo.original" class="flex">
            <span class="text-muted-foreground w-20 shrink-0">原作</span>
            <span>{{ quickInfo.original }}</span>
          </div>
          <div v-if="quickInfo.studio" class="flex">
            <span class="text-muted-foreground w-20 shrink-0">制作</span>
            <span>{{ quickInfo.studio }}</span>
          </div>
          <div v-if="quickInfo.website" class="flex">
            <span class="text-muted-foreground w-20 shrink-0">官网</span>
            <a :href="quickInfo.website" target="_blank" class="text-primary hover:underline truncate">{{ quickInfo.website }}</a>
          </div>
        </div>

        <!-- 追番按钮 -->
        <div class="pt-2">
          <button v-if="detail.is_subscribed"
            class="h-10 px-6 rounded-md bg-primary/10 text-primary text-sm font-medium border border-primary/20 hover:bg-primary/20 transition-colors inline-flex items-center gap-2"
            :disabled="subscribing" @click="handleUnsubscribe">
            <n-icon size="16"><CheckmarkCircleOutline /></n-icon>
            {{ subscribing ? '处理中...' : '已追番' }}
          </button>
          <button v-else
            class="h-10 px-6 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors inline-flex items-center gap-2"
            :disabled="subscribing" @click="handleSubscribe">
            <n-icon size="16"><AddOutline /></n-icon>
            {{ subscribing ? '追番中...' : '追番' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 简介 -->
    <div v-if="detail.summary" class="bg-card rounded-lg border p-5 mb-6">
      <h2 class="text-base font-semibold mb-3">简介</h2>
      <p class="text-sm text-muted-foreground leading-relaxed whitespace-pre-line">{{ detail.summary }}</p>
    </div>

    <!-- 角色 CV -->
    <div v-if="characters.length" class="bg-card rounded-lg border p-5 mb-6">
      <h2 class="text-base font-semibold mb-4">角色 & 声优</h2>
      <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
        <div v-for="c in characters.slice(0, 12)" :key="c.id" class="flex items-center gap-3">
          <div class="w-12 h-12 rounded-full overflow-hidden bg-muted shrink-0">
            <img v-if="c.image" :src="c.image" :alt="c.name" class="w-full h-full object-cover" @error="$event.target.style.display='none'" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="text-sm font-medium truncate">{{ c.name }}</div>
            <div class="text-xs text-muted-foreground truncate">{{ c.relation }}<span v-if="c.actor"> · CV {{ c.actor }}</span></div>
          </div>
        </div>
      </div>
      <button v-if="characters.length > 12 && !showAllChars"
        class="mt-3 text-sm text-primary hover:underline" @click="showAllChars = true">
        展开全部 {{ characters.length }} 个角色
      </button>
    </div>

    <!-- 完整制作信息 -->
    <div v-if="detail.infobox && detail.infobox.length" class="bg-card rounded-lg border p-5">
      <h2 class="text-base font-semibold mb-4">详细信息</h2>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-2 text-sm">
        <div v-for="(kv, i) in visibleInfobox" :key="i" class="flex">
          <span class="text-muted-foreground w-24 shrink-0">{{ kv.key }}</span>
          <span class="flex-1 break-words">{{ formatValue(kv) }}</span>
        </div>
      </div>
      <button v-if="detail.infobox.length > 10 && !showAllInfo"
        class="mt-3 text-sm text-primary hover:underline" @click="showAllInfo = true">
        展开全部信息
      </button>
    </div>
  </div>

  <div v-else class="py-20 text-center text-muted-foreground">
    <p>番剧信息获取失败</p>
    <button class="mt-3 text-primary text-sm hover:underline" @click="$router.back()">返回</button>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage, NIcon, NSpin } from 'naive-ui'
import { get, post, del } from '@/utils/api'
import { toHighResImage } from '@/utils/image'
import { AddOutline, CheckmarkCircleOutline } from '@vicons/ionicons5'

const props = defineProps({ id: { type: [String, Number], required: true } })
const route = useRoute()
const router = useRouter()
const message = useMessage()

const loading = ref(false)
const subscribing = ref(false)
const detail = ref(null)
const characters = ref([])
const showAllInfo = ref(false)
const showAllChars = ref(false)

// 从 infobox 里按 key 取值（按优先级）
function pickInfo(keys) {
  if (!detail.value?.infobox) return ''
  for (const key of keys) {
    const kv = detail.value.infobox.find(x => x.key === key)
    if (kv) return formatValue(kv)
  }
  return ''
}

function formatValue(kv) {
  if (kv.items && kv.items.length) return kv.items.join(', ')
  return kv.value || ''
}

const quickInfo = computed(() => {
  if (!detail.value) return {}
  return {
    airDate: pickInfo(['放送开始', '开始']) || detail.value.air_date,
    airWeekday: pickInfo(['放送星期']),
    episodes: pickInfo(['话数']) || (detail.value.total_episodes > 0 ? String(detail.value.total_episodes) : ''),
    country: pickInfo(['国家', '地区', '製作國家', '制作国家']) || (detail.value.tags?.includes('日本') ? '日本' : ''),
    director: pickInfo(['导演', '總導演', '总导演', '监督']),
    original: pickInfo(['原作']),
    studio: pickInfo(['动画制作', '製作', '制作', '动画公司']),
    website: pickInfo(['官方网站', '官网']),
  }
})

// 隐藏不太重要的 infobox 项（已在上方展示的）
const hideKeys = new Set([
  '中文名', '放送开始', '放送星期', '话数', '导演', '原作', '动画制作', '官方网站',
  '色彩设计', 'CG 导演', 'OP·ED 分镜', 'OP·ED 演出', '制作管理', '文艺制作', '制作进行',
])
const visibleInfobox = computed(() => {
  if (!detail.value?.infobox) return []
  const list = detail.value.infobox.filter(kv => !hideKeys.has(kv.key))
  return showAllInfo.value ? list : list.slice(0, 10)
})

async function fetchDetail() {
  loading.value = true
  try {
    const data = await get(`/bangumi/${props.id}`)
    detail.value = data
  } catch (e) {
    message.error('获取番剧详情失败')
  } finally {
    loading.value = false
  }
  // 异步加载角色
  fetchCharacters()
}

async function fetchCharacters() {
  try {
    const data = await get(`/bangumi/${props.id}/characters`)
    characters.value = Array.isArray(data) ? data : []
  } catch { characters.value = [] }
}

async function handleSubscribe() {
  subscribing.value = true
  try {
    const resp = await post(`/bangumi/${props.id}/subscribe`)
    message.success('追番成功')
    // 追番成功后跳转到 AnimeDetail（有观看源）
    if (resp.anime_id) {
      router.replace(`/anime/${resp.anime_id}`)
    } else {
      await fetchDetail()
    }
  } catch (e) {
    message.error(e.message || '追番失败')
  } finally { subscribing.value = false }
}

async function handleUnsubscribe() {
  subscribing.value = true
  try {
    await del(`/bangumi/${props.id}/subscribe`)
    message.success('已取消追番')
    await fetchDetail()
  } catch (e) {
    message.error(e.message || '取消追番失败')
  } finally { subscribing.value = false }
}

watch(() => props.id, fetchDetail)
onMounted(fetchDetail)
</script>
