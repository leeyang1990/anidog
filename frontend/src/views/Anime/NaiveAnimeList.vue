<template>
  <div>
    <AcPageHeader :title="t('pages.animeList.title')" :subtitle="t('pages.animeList.subtitle')">
      <template #actions>
        <div v-if="!isMobile" class="flex gap-1 p-1 rounded-2xl bg-ac-sand">
          <button type="button"
            class="px-3 py-1.5 rounded-xl text-xs font-bold transition-colors flex items-center gap-1"
            :class="viewMode === 'grid' ? 'bg-card text-ac-grass-dark shadow-sm' : 'text-muted-foreground hover:text-foreground'"
            @click="viewMode = 'grid'">
            <GridOutline class="size-3.5" /> {{ t('pages.animeList.grid') }}
          </button>
          <button type="button"
            class="px-3 py-1.5 rounded-xl text-xs font-bold transition-colors flex items-center gap-1"
            :class="viewMode === 'list' ? 'bg-card text-ac-grass-dark shadow-sm' : 'text-muted-foreground hover:text-foreground'"
            @click="viewMode = 'list'">
            <ListOutline class="size-3.5" /> {{ t('pages.animeList.list') }}
          </button>
        </div>
      </template>
    </AcPageHeader>

    <AcCard padding="md" rounded="2xl" class="mb-6">
      <div class="flex items-center gap-2 flex-wrap">
        <div class="flex-1 min-w-[200px]">
          <AcInput v-model="searchQuery" :placeholder="isMobile ? t('pages.animeList.searchShort') : t('pages.animeList.search')" size="md" clearable @keyup-enter="fetchAnimeList">
            <template #prefix><SearchOutline class="size-4" /></template>
          </AcInput>
        </div>
        <div class="w-32">
          <AcSelect v-model="statusFilter" :options="statusOptions" :placeholder="t('pages.animeList.status')" />
        </div>
        <div class="w-40">
          <AcSelect v-model="sortBy" :options="sortOptions" :placeholder="t('pages.animeList.sort')" />
        </div>
        <AcButton variant="primary" :loading="loading" @click="fetchAnimeList">
          <template #icon><SearchOutline class="size-4" /></template>
          <span v-if="!isMobile">{{ t('common.search') }}</span>
        </AcButton>
      </div>
    </AcCard>

    <div v-if="loading" class="flex justify-center py-12"><AcSpinner :size="48" /></div>

    <template v-else>
      <AcEmpty v-if="animeList.length === 0" :title="t('pages.animeList.empty')" :description="t('pages.animeList.emptyDesc')">
        <template #actions>
          <AcButton variant="primary" @click="$router.push('/search')">{{ t('pages.animeList.add') }}</AcButton>
        </template>
      </AcEmpty>

      <template v-else>
        <div :class="gridClasses">
          <NaiveAnimeCard
            v-for="anime in animeList" :key="anime.id"
            :anime="anime" @click="handleAnimeClick(anime)" @delete="handleDelete"
          />
        </div>
      </template>
    </template>

    <div v-if="totalAnime > pagination.pageSize" class="flex justify-center items-center gap-3 mt-6 text-sm">
      <AcButton size="sm" variant="outline" :disabled="pagination.page <= 1" @click="handlePageChange(pagination.page - 1)">{{ t('pages.animeList.previous') }}</AcButton>
      <span class="text-muted-foreground font-num font-bold">{{ t('pages.animeList.page', { page: pagination.page, total: totalPages }) }}</span>
      <AcButton size="sm" variant="outline" :disabled="pagination.page >= totalPages" @click="handlePageChange(pagination.page + 1)">{{ t('pages.animeList.next') }}</AcButton>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { SearchOutline, GridOutline, ListOutline } from '@vicons/ionicons5'
import { get, post, del } from '@/utils/api'
import { useResponsive } from '@/composables/useResponsive'
import { AcPageHeader, AcCard, AcInput, AcSelect, AcButton, AcSpinner, AcEmpty } from '@/components/ac'
import NaiveAnimeCard from '@/components/Anime/NaiveAnimeCard.vue'
import { useI18n } from 'vue-i18n'

const router = useRouter()
const toast = useToast()
const { t } = useI18n({ useScope: 'global' })
const { isMobile, isTablet } = useResponsive()

const loading = ref(false)
const searchQuery = ref('')
const statusFilter = ref('')
const sortBy = ref('updated_at_desc')
const viewMode = ref('grid')
const animeList = ref([])
const totalAnime = ref(0)

const pagination = ref({ page: 1, pageSize: 20 })

const statusOptions = computed(() => [
  { label: t('pages.animeList.all'), value: '' },
  { label: t('pages.animeList.ongoing'), value: 'ongoing' },
  { label: t('pages.animeList.completed'), value: 'completed' },
  { label: t('pages.animeList.upcoming'), value: 'upcoming' },
  { label: t('pages.animeList.dropped'), value: 'dropped' }
])

const sortOptions = computed(() => [
  { label: t('pages.animeList.recent'), value: 'updated_at_desc' },
  { label: t('pages.animeList.az'), value: 'title_asc' },
  { label: t('pages.animeList.za'), value: 'title_desc' },
  { label: t('pages.animeList.rating'), value: 'rating_desc' }
])

const gridClasses = computed(() => {
  if (isMobile.value) return 'grid grid-cols-2 gap-3'
  if (isTablet.value) return 'grid grid-cols-3 gap-4'
  return 'grid gap-4'
})

const totalPages = computed(() => Math.ceil(totalAnime.value / pagination.value.pageSize))

async function fetchAnimeList() {
  loading.value = true
  try {
    const params = {
      page: pagination.value.page,
      per_page: pagination.value.pageSize,
      subscribed: true
    }
    if (statusFilter.value) params.status = statusFilter.value
    const data = await get('/anime', { params })
    animeList.value = data.items || []
    totalAnime.value = data.total || 0
  } catch {
    toast.error(t('pages.animeList.fetchFailed'))
    animeList.value = []
    totalAnime.value = 0
  } finally { loading.value = false }
}

function handlePageChange(page) {
  pagination.value.page = page
  fetchAnimeList()
}

function handleAnimeClick(anime) { router.push(`/anime/${anime.id}`) }

async function handleDelete(anime) {
  try {
    await del(`/anime/${anime.id}`)
    animeList.value = animeList.value.filter(a => a.id !== anime.id)
    totalAnime.value--
    toast.success(t('pages.animeList.deleted', { title: anime.title }))
  } catch { toast.error(t('pages.animeList.deleteFailed')) }
}

watch([statusFilter], () => {
  pagination.value.page = 1
  fetchAnimeList()
})

onMounted(fetchAnimeList)
</script>

<style scoped>
.grid {
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
}
</style>
