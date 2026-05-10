<template>
  <div>
    <PageHeader title="放送日历" subtitle="查看当季番剧放送时间表">
      <template #actions>
        <button
          class="inline-flex items-center gap-1.5 h-9 px-3 rounded-md border border-input bg-background text-sm font-medium hover:bg-accent transition-colors"
          :disabled="refreshing"
          @click="refreshCalendar"
        >
          <n-icon size="16"><RefreshOutline /></n-icon>
          {{ refreshing ? '刷新中...' : '刷新' }}
        </button>
      </template>
    </PageHeader>

    <div v-if="loading" class="flex justify-center py-16">
      <n-spin size="large" />
    </div>

    <template v-else>
      <!-- 星期选择器 -->
      <nav class="flex gap-2 mb-6 overflow-x-auto pb-1">
        <button v-for="day in calendarData" :key="day.weekday"
          class="flex flex-col items-center px-5 py-3 rounded-lg text-sm font-medium whitespace-nowrap transition-all shrink-0 min-w-[72px]"
          :class="activeDay === day.weekday
            ? 'bg-primary text-primary-foreground shadow-md'
            : 'bg-card border text-muted-foreground hover:bg-accent hover:text-foreground'"
          @click="activeDay = day.weekday">
          <span>{{ day.weekdayName }}</span>
          <span v-if="day.isToday" class="text-[10px] opacity-80 mt-0.5">今天</span>
          <span class="text-xs mt-1 opacity-70">{{ day.items.length }}部</span>
        </button>
      </nav>

      <!-- 当天番剧列表 -->
      <div v-if="currentDay" class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8 gap-3">
        <AnimeCard
          v-for="item in currentDay.items"
          :key="item.id"
          :item="item"
          @click="goToDetail(item)"
          @subscribe="subscribeBangumi(item)"
        />
      </div>
      <div v-else class="py-16 text-center text-muted-foreground">暂无放送日历数据</div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { get, post } from '@/utils/api'
import { useMessage, NIcon, NSpin } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import { useResponsive } from '@/composables/useResponsive'
import PageHeader from '@/components/Common/PageHeader.vue'
import AnimeCard from '../Anime/AnimeCard.vue'

const router = useRouter()
const message = useMessage()

const loading = ref(false)
const refreshing = ref(false)
const calendarData = ref([])

const WEEKDAY_NAMES = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const today = new Date().getDay()
const activeDay = ref(today)

const currentDay = computed(() => {
  return calendarData.value.find(d => d.weekday === activeDay.value)
})

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
          id: item.id,
          name: item.name_cn || item.name,
          image: item.image,
          rating_score: item.rating_score,
          air_date: item.air_date,
          is_subscribed: item.is_subscribed,
          local_id: item.local_id
        }))
      }
    }

    calendarData.value = Object.values(grouped).sort((a, b) => {
      const diffA = (a.weekday - today + 7) % 7
      const diffB = (b.weekday - today + 7) % 7
      return diffA - diffB
    })
  } catch (e) {
    message.error('获取放送日历失败')
  } finally {
    loading.value = false
  }
}

async function refreshCalendar() {
  refreshing.value = true
  try {
    await post('/calendar/refresh')
    message.success('日历已刷新')
    await fetchCalendar()
  } catch (e) {
    message.error('刷新失败')
  } finally {
    refreshing.value = false
  }
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
  if (item.local_id) {
    router.push(`/anime/${item.local_id}`)
  } else {
    router.push(`/anime-library/${item.id}`)
  }
}

onMounted(fetchCalendar)
</script>
