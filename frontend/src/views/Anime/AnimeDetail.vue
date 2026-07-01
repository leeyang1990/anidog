<template>
  <div>
    <!-- Hero banner -->
    <div class="relative h-64 md:h-80 bg-muted overflow-hidden -mx-4 md:-mx-6 -mt-4 md:-mt-6">
      <img v-if="coverImage" :src="toHighResImage(coverImage)" class="h-full w-full object-cover opacity-30" />
      <div class="absolute inset-0 bg-gradient-to-t from-background via-background/60 to-transparent" />

      <!-- 返回按钮（悬浮） -->
      <button
        class="absolute top-4 left-4 md:top-6 md:left-6 z-20 inline-flex items-center gap-1.5 h-9 px-4 rounded-2xl bg-card/85 backdrop-blur-sm border-2 border-ac-sand text-sm font-bold hover:bg-card hover:border-ac-grass transition-colors shadow-md"
        @click="$router.back()"
      >
        ← 返回
      </button>
    </div>

    <!-- Content overlapping hero -->
    <div class="-mt-32 relative z-10">
      <div v-if="loading" class="flex justify-center py-20"><AcSpinner :size="48" /></div>
      <template v-else>
        <div class="grid grid-cols-1 md:grid-cols-[200px_1fr] gap-6 md:gap-8">
          <!-- Cover -->
          <div class="flex justify-center md:justify-start">
            <div class="w-40 md:w-full rounded-3xl border-2 border-ac-sand overflow-hidden shadow-lg">
              <img :src="toHighResImage(coverImage) || ''" class="w-full aspect-[2/3] object-cover bg-ac-sand/40" />
            </div>
          </div>

          <!-- Info -->
          <div class="space-y-4">
            <div>
              <h1 class="text-2xl md:text-3xl font-bold tracking-tight">{{ displayTitle || '加载中...' }}</h1>
              <p v-if="displayOriginalTitle" class="text-muted-foreground mt-1">{{ displayOriginalTitle }}</p>
            </div>

            <div class="flex flex-wrap items-center gap-3 text-sm">
              <span v-if="anime.bangumi_rating" class="flex items-center gap-1">
                <span class="text-ac-sun-dark font-bold text-base font-num">{{ anime.bangumi_rating }}</span>
                <span class="text-muted-foreground font-num">/ 10</span>
              </span>
              <span v-if="bangumi.rank" class="text-muted-foreground font-num font-bold">#{{ bangumi.rank }}</span>
              <AcTag v-if="bangumi.platform" variant="grass">{{ bangumi.platform }}</AcTag>
            </div>

            <!-- 追番按钮 -->
            <div class="pt-1 flex gap-2 flex-wrap">
              <AcButton v-if="isSubscribed" variant="secondary" :loading="subscribing" @click="handleUnsubscribe">
                <template #icon><CheckmarkCircleOutline class="size-4" /></template>
                {{ subscribing ? '处理中...' : '已追番' }}
              </AcButton>
              <AcButton v-else variant="primary" :loading="subscribing" @click="handleSubscribe">
                <template #icon><AddOutline class="size-4" /></template>
                {{ subscribing ? '追番中...' : '追番' }}
              </AcButton>

              <!-- 检查更新按钮：已追番 + 已有 anime_id 才能用 -->
              <AcButton v-if="isSubscribed && animeId" variant="outline" :loading="checkingUpdates" @click="handleCheckUpdates">
                <template #icon><RefreshOutline class="size-4" /></template>
                {{ checkingUpdates ? '检查中...' : '检查更新' }}
              </AcButton>
            </div>

            <!-- Bangumi 原页面链接 -->
            <div v-if="anime.bangumi_id" class="mb-4">
              <a
                :href="`https://bgm.tv/subject/${anime.bangumi_id}`"
                target="_blank"
                class="inline-flex items-center gap-1.5 text-sm text-ac-grass-dark hover:underline font-bold"
              >
                <OpenOutline class="size-4" />
                在 Bangumi 查看原页面
              </a>
            </div>

            <!-- 快速信息栏 -->
            <div v-if="hasQuickInfo" class="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-1.5 text-sm">
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
                <span class="text-muted-foreground w-20 shrink-0">国家</span>
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
          </div>
        </div>

        <!-- 追番下载（新版本：源无关剧集网格） -->
        <div v-if="animeId" class="mt-6 bg-card text-card-foreground rounded-3xl border-2 border-ac-sand p-6 shadow-md">
          <EpisodeGrid
            :anime-id="animeId"
            :anime-title="anime.title || displayTitle"
            :episode-count="anime.episode_count || 0"
          />
        </div>

        <!-- 高级：手动源配置（默认折叠，旧版 StreamSetupCard） -->
        <details class="mt-4 bg-card text-card-foreground rounded-3xl border-2 border-ac-sand shadow-md">
          <summary class="px-6 py-3 cursor-pointer text-sm font-bold text-muted-foreground hover:text-foreground select-none flex items-center justify-between">
            <span>🔧 高级：手动源配置 / 流媒体观看</span>
            <span class="text-xs text-muted-foreground">点击展开</span>
          </summary>
          <div class="px-6 pb-6 pt-2">
            <StreamSetupCard
              :anime-id="animeId"
              :anime-title="displayTitle"
              :subscribed="isSubscribed"
            />
          </div>
        </details>

        <!-- 概述（单独卡片） -->
        <div v-if="summary" class="mt-6 bg-card text-card-foreground rounded-3xl border-2 border-ac-sand p-5 shadow-md">
          <h2 class="text-base font-bold mb-3">📝 概述</h2>
          <p class="text-sm text-muted-foreground leading-relaxed whitespace-pre-line">{{ summary }}</p>
        </div>

        <!-- 标签 -->
        <div v-if="bangumi.tags && bangumi.tags.length" class="mt-6 bg-card text-card-foreground rounded-3xl border-2 border-ac-sand p-5 shadow-md">
          <h2 class="text-base font-bold mb-3">🏷️ 标签</h2>
          <div class="flex flex-wrap gap-2">
            <button v-for="tag in (showAllTags ? bangumi.tags : bangumi.tags.slice(0, 10))" :key="tag"
              class="px-3 py-1.5 rounded-full text-xs font-bold border-2 border-ac-sand transition-colors hover:border-ac-grass hover:bg-ac-grass/10"
              @click="searchByTag(tag)">
              {{ tag }}
            </button>
          </div>
          <button v-if="bangumi.tags.length > 10 && !showAllTags"
            class="mt-3 text-sm text-ac-grass-dark hover:underline font-bold" @click="showAllTags = true">
            展开全部 {{ bangumi.tags.length }} 个标签
          </button>
        </div>

        <!-- 角色 CV -->
        <div v-if="characters.length" class="mt-6 bg-card text-card-foreground rounded-3xl border-2 border-ac-sand p-5 shadow-md">
          <h2 class="text-base font-bold mb-4">🎭 角色 &amp; 声优</h2>
          <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
            <button v-for="c in (showAllChars ? characters : characters.slice(0, 12))" :key="c.id"
              class="flex items-center gap-3 p-2 rounded-2xl hover:bg-ac-cream/40 transition-colors text-left"
              @click="showCharacterDetail(c)">
              <div class="size-12 rounded-full overflow-hidden bg-ac-sand/40 shrink-0 border-2 border-ac-sand">
                <img v-if="c.image" :src="c.image" :alt="c.name" class="w-full h-full object-cover" @error="$event.target.style.display='none'" />
              </div>
              <div class="min-w-0 flex-1">
                <div class="text-sm font-bold truncate">{{ c.name }}</div>
                <div class="text-xs text-muted-foreground truncate">{{ c.relation }}<span v-if="c.actor"> · CV {{ c.actor }}</span></div>
              </div>
            </button>
          </div>
          <button v-if="characters.length > 12 && !showAllChars"
            class="mt-3 text-sm text-ac-grass-dark hover:underline font-bold" @click="showAllChars = true">
            展开全部 {{ characters.length }} 个角色
          </button>
        </div>

        <!-- 完整制作信息 -->
        <div v-if="bangumi.infobox && bangumi.infobox.length" class="mt-6 bg-card text-card-foreground rounded-3xl border-2 border-ac-sand p-5 shadow-md">
          <h2 class="text-base font-bold mb-4">📋 详细信息</h2>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-2 text-sm">
            <div v-for="(kv, i) in visibleInfobox" :key="i" class="flex">
              <span class="text-muted-foreground w-24 shrink-0 font-bold">{{ kv.key }}</span>
              <span class="flex-1 break-words">{{ formatValue(kv) }}</span>
            </div>
          </div>
          <button v-if="bangumi.infobox.length > 10 && !showAllInfo"
            class="mt-3 text-sm text-ac-grass-dark hover:underline font-bold" @click="showAllInfo = true">
            展开全部信息
          </button>
        </div>

        <!-- 角色详情弹窗 -->
        <CharacterDetailModal v-model:show="showCharModal" :character="selectedChar" />
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { CheckmarkCircleOutline, OpenOutline, AddOutline, RefreshOutline } from '@vicons/ionicons5'
import { get, post, del } from '@/utils/api'
import { toHighResImage } from '@/utils/image'
import { AcButton, AcSpinner, AcTag } from '@/components/ac'
import CharacterDetailModal from '@/components/Anime/CharacterDetailModal.vue'
import StreamSetupCard from '@/components/Anime/StreamSetupCard.vue'
import EpisodeGrid from '@/components/Anime/EpisodeGrid.vue'

const route = useRoute()
const router = useRouter()
const toast = useToast()
const routeId = route.params.id

// 判断是从哪个路由进入的：/anime-library/:id 是 bangumi 模式，/anime/:id 是 anime 模式
const isBangumiMode = route.path.startsWith('/anime-library/')
const animeId = ref(null) // 实际的 anime 数据库 ID（追番后才有）
const bangumiId = ref(isBangumiMode ? parseInt(routeId) : null)

const loading = ref(true)
const subscribing = ref(false)
const checkingUpdates = ref(false)
const anime = ref({})
const bangumi = ref({})
const characters = ref([])
const showAllInfo = ref(false)
const showAllChars = ref(false)
const showAllTags = ref(false)
const showCharModal = ref(false)
const selectedChar = ref(null)

const coverImage = computed(() => anime.value.cover_url || anime.value.cover_image || bangumi.value.image || '')
const summary = computed(() => bangumi.value.summary || anime.value.description || '')
const isSubscribed = computed(() => bangumi.value.is_subscribed || anime.value.is_subscribed || false)
const displayTitle = computed(() => bangumi.value.name_cn || bangumi.value.name || anime.value.title || '')
const displayOriginalTitle = computed(() => {
  const cn = bangumi.value.name_cn || ''
  const orig = bangumi.value.name || ''
  if (cn && orig && cn !== orig) return orig
  return anime.value.original_title || ''
})

function formatValue(kv) {
  if (kv.items && kv.items.length) return kv.items.join(', ')
  return kv.value || ''
}

function pickInfo(keys) {
  if (!bangumi.value?.infobox) return ''
  for (const key of keys) {
    const kv = bangumi.value.infobox.find(x => x.key === key)
    if (kv) return formatValue(kv)
  }
  return ''
}

const quickInfo = computed(() => {
  const bgm = bangumi.value || {}
  return {
    airDate: pickInfo(['放送开始', '开始']) || bgm.air_date || anime.value.air_date,
    airWeekday: pickInfo(['放送星期']) || getWeekdayName(anime.value.air_weekday),
    episodes: pickInfo(['话数']) || (bgm.total_episodes > 0 ? String(bgm.total_episodes) : '') || (anime.value.episode_count || ''),
    country: pickInfo(['国家', '地区', '製作國家', '制作国家']) || (bgm.tags?.includes('日本') ? '日本' : ''),
    director: pickInfo(['导演', '总导演', '监督']),
    original: pickInfo(['原作']),
    studio: pickInfo(['动画制作', '製作', '制作', '动画公司']),
    website: pickInfo(['官方网站', '官网']),
  }
})
const hasQuickInfo = computed(() => Object.values(quickInfo.value).some(v => v))

// 过滤不重要的 infobox 项（已在上方快速栏或太细节的）
const hideKeys = new Set([
  '中文名', '放送开始', '放送星期', '话数', '导演', '原作', '动画制作', '官方网站',
  '色彩设计', 'CG 导演', 'OP·ED 分镜', 'OP·ED 演出', '制作管理', '文艺制作', '制作进行',
  '音响制作担当', '录音助理', '录音工作室', '音乐监督', '音乐助理', '摄影监督助理',
])
const visibleInfobox = computed(() => {
  if (!bangumi.value?.infobox) return []
  const list = bangumi.value.infobox.filter(kv => !hideKeys.has(kv.key))
  return showAllInfo.value ? list : list.slice(0, 10)
})

const WEEKDAY_NAMES = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

function showCharacterDetail(char) {
  selectedChar.value = char
  showCharModal.value = true
}

function searchByTag(tag) {
  router.push({ path: '/anime-library', query: { tag } })
}
function getWeekdayName(wd) {
  if (wd == null) return ''
  return WEEKDAY_NAMES[wd] || ''
}

async function handleSubscribe() {
  const bgmId = bangumi.value.id
  if (!bgmId) return
  subscribing.value = true
  try {
    const resp = await post(`/bangumi/${bgmId}/subscribe`)
    toast.success('追番成功')
    bangumi.value = { ...bangumi.value, is_subscribed: true }
    if (resp.anime_id) {
      animeId.value = resp.anime_id
      try {
        const animeData = await get(`/anime/${resp.anime_id}`)
        anime.value = animeData
      } catch { /* ignore */ }
    }
  } catch (e) {
    toast.error(e.message || '追番失败')
  } finally { subscribing.value = false }
}

async function handleUnsubscribe() {
  const bgmId = bangumi.value.id
  if (!bgmId) return
  subscribing.value = true
  try {
    await del(`/bangumi/${bgmId}/subscribe`)
    toast.success('已取消追番')
    // 两个源都要更新，因为 isSubscribed 是 bangumi || anime 的 OR
    bangumi.value = { ...bangumi.value, is_subscribed: false }
    if (anime.value) {
      anime.value = { ...anime.value, is_subscribed: false }
    }
  } catch (e) {
    toast.error(e.message || '取消追番失败')
  } finally { subscribing.value = false }
}

async function handleCheckUpdates() {
  if (!animeId.value) return
  checkingUpdates.value = true
  try {
    await post(`/anime/${animeId.value}/check-updates`)
    toast.success('已触发更新检查，几秒后查看剧集状态')
  } catch (e) {
    toast.error(e.message || '检查更新失败')
  } finally {
    // 给后端一点时间处理（搜索+下载是异步的）
    setTimeout(() => { checkingUpdates.value = false }, 2000)
  }
}

async function fetchAnimeDetail() {
  loading.value = true
  try {
    if (isBangumiMode) {
      // 番剧库模式：用 bangumi_id 获取详情
      const data = await get(`/bangumi/${routeId}`)
      bangumi.value = data
      // 如果已追番，获取 anime 数据和观看源
      if (data.is_subscribed && data.local_id) {
        animeId.value = data.local_id
        try {
          const animeData = await get(`/anime/${data.local_id}`)
          anime.value = animeData
        } catch { /* ignore */ }
      }
      fetchCharacters(data.id)
    } else {
      // 已追番模式：用 anime_id 获取数据
      const data = await get(`/anime/${routeId}`)
      anime.value = data
      animeId.value = parseInt(routeId)
      if (data.bangumi_id) {
        fetchBangumiInfo(data.bangumi_id)
      }
    }
  } catch {
    toast.error('获取详情失败')
  } finally {
    loading.value = false
  }
}

async function fetchBangumiInfo(bangumiID) {
  try {
    const [detail, chars] = await Promise.all([
      get(`/bangumi/${bangumiID}`).catch(() => null),
      get(`/bangumi/${bangumiID}/characters`).catch(() => [])
    ])
    if (detail) bangumi.value = detail
    characters.value = Array.isArray(chars) ? chars : []
  } catch (e) {
    console.error('获取 Bangumi 详情失败:', e)
  }
}

async function fetchCharacters(bgmId) {
  try {
    const data = await get(`/bangumi/${bgmId}/characters`)
    characters.value = Array.isArray(data) ? data : []
  } catch {
    characters.value = []
  }
}

onMounted(fetchAnimeDetail)
</script>
