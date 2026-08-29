<template>
  <div>
    <AcPageHeader :title="t('pages.calendar.title')" :subtitle="t('pages.calendar.subtitle')">
      <template #actions>
        <AcButton variant="outline" :loading="refreshing" @click="refreshCalendar">
          <template #icon><RefreshOutline class="size-4" /></template>
          {{ refreshing ? t('pages.calendar.refreshing') : t('pages.calendar.refresh') }}
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
          <span>{{ weekdayName(day.weekday) }}</span>
          <span v-if="day.isToday" class="text-[10px] mt-0.5 px-1.5 rounded-full bg-ac-sun text-ac-night">{{ t('pages.calendar.today') }}</span>
          <span class="text-xs mt-1 opacity-70 font-num">{{ t('pages.calendar.itemCount', { count: day.items.length }) }}</span>
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
      <AcEmpty v-else :title="t('pages.calendar.empty')" :description="t('pages.calendar.emptyDesc')" class="py-12" />
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
import { useI18n } from 'vue-i18n'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { t } = useI18n({ useScope: 'global' })

const loading = ref(false)
const refreshing = ref(false)
const calendarData = ref([])

const today = new Date().getDay()
const routeDay = Number(route.query.day)
const activeDay = ref(Number.isInteger(routeDay) && routeDay >= 0 && routeDay <= 6 ? routeDay : today)

const currentDay = computed(() => calendarData.value.find(d => d.weekday === activeDay.value))

function weekdayName(day) {
  return t(`pages.calendar.weekdays.${day}`)
}

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
      grouped[i] = { weekday: i, isToday: i === today, items: [] }
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
  } catch { toast.error(t('pages.calendar.fetchFailed')) }
  finally { loading.value = false }
}

async function refreshCalendar() {
  refreshing.value = true
  try {
    await post('/calendar/refresh')
    toast.success(t('pages.calendar.refreshed'))
    await fetchCalendar()
  } catch { toast.error(t('pages.calendar.refreshFailed')) }
  finally { refreshing.value = false }
}

async function subscribeBangumi(item) {
  try {
    await post(`/bangumi/${item.id}/subscribe`)
    toast.success(t('pages.calendar.subscribed'))
    item.is_subscribed = true
  } catch (e) { toast.error(e.message || t('pages.calendar.subscribeFailed')) }
}

function goToDetail(item) {
  if (item.local_id) router.push(`/anime/${item.local_id}`)
  else router.push(`/anime-library/${item.id}`)
}

onMounted(fetchCalendar)
</script>
