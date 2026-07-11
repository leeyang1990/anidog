<template>
  <div>
    <AcPageHeader title="📅 放送日历" subtitle="查看当季番剧放送时间表">
      <template #actions>
        <AcButton variant="outline" :loading="refreshing" @click="refreshCalendar">
          <template #icon><RefreshOutline class="size-4" /></template>
          {{ refreshing ? '刷新中...' : '刷新' }}
        </AcButton>
      </template>
    </AcPageHeader>

    <div v-if="loading" class="flex justify-center py-16"><AcSpinner :size="48" /></div>

    <template v-else>
      <!-- 星期选择器 -->
      <nav class="flex gap-2 mb-6 overflow-x-auto pb-2">
        <button v-for="day in calendarData" :key="day.weekday"
          type="button"
          class="flex flex-col items-center px-5 py-3 rounded-2xl text-sm font-bold whitespace-nowrap transition-all shrink-0 min-w-[80px] border-2"
          :class="activeDay === day.weekday
            ? 'bg-ac-grass text-white border-ac-grass-dark shadow-sm'
            : 'bg-card border-ac-sand text-muted-foreground hover:border-ac-grass'"
          @click="selectDay(day.weekday)">
          <span>{{ day.weekdayName }}</span>
          <span v-if="day.isToday" class="text-[10px] mt-0.5 px-1.5 rounded-full bg-ac-sun text-ac-night">今天</span>
          <span class="text-xs mt-1 opacity-70 font-num">{{ day.items.length }}部</span>
        </button>
      </nav>

      <!-- 当天番剧列表 -->
      <div v-if="currentDay && currentDay.items.length" class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8 gap-3">
        <AnimeCard
          v-for="item in currentDay.items"
          :key="item.id"
          :item="item"
          @click="goToDetail(item)"
          @subscribe="subscribeBangumi(item)"
        />
      </div>
      <AcEmpty v-else title="今天没有番剧放送" description="试试其他星期吧 🌸" class="py-12" />
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { get, post } from '@/utils/api'
import { useToast } from '@/composables/useToast'
import { RefreshOutline } from '@vicons/ionicons5'
import { AcPageHeader, AcButton, AcSpinner, AcEmpty } from '@/components/ac'
import AnimeCard from '../Anime/AnimeCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

const loading = ref(false)
const refreshing = ref(false)
const calendarData = ref([])

const WEEKDAY_NAMES = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const today = new Date().getDay()
const routeDay = Number(route.query.day)
const activeDay = ref(Number.isInteger(routeDay) && routeDay >= 0 && routeDay <= 6 ? routeDay : today)

const currentDay = computed(() => calendarData.value.find(d => d.weekday === activeDay.value))

function selectDay(day) {
  activeDay.value = day
  router.replace({ query: { ...route.query, day: String(day) } })
}

async function fetchCalendar() {
  loading.value = true
  try {
    const data = await get('/calendar')
    const days = Array.isArray(data) ? data : []
    const grouped = {}
    for (let i = 0; i < 7; i++) {
      grouped[i] = { weekday: i, weekdayName: WEEKDAY_NAMES[i], isToday: i === today, items: [] }
    }
    for (const day of days) {
      const wd = day.weekday_id === 7 ? 0 : day.weekday_id
      if (wd >= 0 && wd <= 6 && grouped[wd]) {
        grouped[wd].items = (day.items || []).map(item => ({
          id: item.id, name: item.name_cn || item.name, image: item.image,
          rating_score: item.rating_score, air_date: item.air_date,
          is_subscribed: item.is_subscribed, local_id: item.local_id
        }))
      }
    }
    calendarData.value = Object.values(grouped).sort((a, b) => {
      const diffA = (a.weekday - today + 7) % 7
      const diffB = (b.weekday - today + 7) % 7
      return diffA - diffB
    })
  } catch { toast.error('获取放送日历失败') }
  finally { loading.value = false }
}

async function refreshCalendar() {
  refreshing.value = true
  try {
    await post('/calendar/refresh')
    toast.success('日历已刷新')
    await fetchCalendar()
  } catch { toast.error('刷新失败') }
  finally { refreshing.value = false }
}

async function subscribeBangumi(item) {
  try {
    await post(`/bangumi/${item.id}/subscribe`)
    toast.success('追番成功')
    item.is_subscribed = true
  } catch (e) { toast.error(e.message || '追番失败') }
}

function goToDetail(item) {
  if (item.local_id) router.push(`/anime/${item.local_id}`)
  else router.push(`/anime-library/${item.id}`)
}

onMounted(fetchCalendar)
</script>
