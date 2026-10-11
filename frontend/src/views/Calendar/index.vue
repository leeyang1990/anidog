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
      <div v-if="source !== 'bangumi'" role="status" class="mb-5 rounded-xl border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-900">
        {{ source === 'cache' ? t('pages.calendar.cached', { time: updatedAt }) : t('pages.calendar.local', { start: weekStart, end: weekEnd }) }}
      </div>
      <!-- 星期选择器 -->
      <nav class="flex gap-2 mb-6 overflow-x-auto pb-2">
        <button v-for="day in calendarData" :key="day.weekday"
          type="button"
          class="flex flex-col items-center px-5 py-3 rounded-2xl text-sm font-bold whitespace-nowrap transition-all duration-150 shrink-0 min-w-[80px] border-2 active:translate-y-[2px]"
          :class="activeDay === day.weekday
            ? 'bg-ac-grass text-white border-ac-grass-dark shadow-sm'
            : 'bg-card border-ac-sand text-muted-foreground hover:border-ac-grass'"
          @click="selectDay(day.weekday)">
          <span>{{ weekdayName(day.weekday) }}</span>
          <span v-if="day.isToday" class="text-[10px] mt-0.5 px-1.5 rounded-full bg-ac-sun text-ac-night">{{ t('pages.calendar.today') }}</span>
          <span class="text-xs mt-1 opacity-70 font-num">
            <i18n-t keypath="pages.calendar.itemCount" scope="global">
              <template #count><AcCountUp :value="day.items.length" :duration="420" from-zero /></template>
            </i18n-t>
          </span>
        </button>
      </nav>

      <!-- 当天番剧列表 -->
      <div v-if="currentDay && currentDay.items.length" class="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6">
        <AnimeCard
          v-for="item in currentDay.items"
          :key="item.local_id || item.id"
          :item="item"
          @click="goToDetail(item)"
          @subscribe="subscribeBangumi(item)"
        />
      </div>
      <AcEmpty v-else :title="t(source === 'local' ? 'pages.calendar.localEmpty' : 'pages.calendar.empty', { day: weekdayName(activeDay) })" :description="t('pages.calendar.emptyDesc')" class="py-12" />
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { calendarDay, calendarDate, calendarDays } from '@/utils/calendar'
import { get, post } from '@/utils/api'
import { useToast } from '@/composables/useToast'
import { RefreshOutline } from '@vicons/ionicons5'
import { AcPageHeader, AcButton, AcSpinner, AcEmpty, AcCountUp } from '@/components/ac'
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
const activeDay = ref(calendarDay(route.query.day, today))
const source = ref('bangumi')
const updatedAt = ref('')
const weekStart = ref('')
const weekEnd = ref('')

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
    const data = await get(`/calendar?metadata=1&date=${calendarDate()}`)
    source.value = data.source || 'bangumi'
    updatedAt.value = data.updated_at ? new Date(data.updated_at).toLocaleString() : ''
    weekStart.value = data.week_start || ''
    weekEnd.value = data.week_end || ''
    calendarData.value = calendarDays(Array.isArray(data) ? data : data.days, today)
    return true
  } catch { toast.error(t('pages.calendar.fetchFailed')); return false }
  finally { loading.value = false }
}

async function refreshCalendar() {
  refreshing.value = true
  try {
    await post('/calendar/refresh')
    const fetched = await fetchCalendar()
    if (fetched && source.value === 'bangumi') toast.success(t('pages.calendar.refreshed'))
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
