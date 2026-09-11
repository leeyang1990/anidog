<template>
  <div
    class="group relative bg-card rounded-2xl border-2 border-ac-sand overflow-hidden cursor-pointer hover:shadow-lg hover:-translate-y-0.5 hover:border-ac-grass transition-all duration-200"
    @click="$emit('click')"
  >
    <!-- 删除按钮 (hover 显示) -->
    <button
      type="button"
      class="absolute top-2 right-2 z-10 size-8 rounded-full bg-card/90 backdrop-blur-sm border-2 border-ac-sand flex items-center justify-center opacity-0 group-hover:opacity-100 hover:bg-ac-heart hover:text-white hover:border-ac-heart-dark transition-all shadow-sm"
      title="从追番列表移除"
      @click.stop="handleDelete"
    >
      <TrashOutline class="size-3.5" />
    </button>

    <!-- Cover -->
    <div class="relative aspect-[2/3] overflow-hidden bg-ac-sand/40">
      <AcPoster
        :src="anime.cover_url || anime.cover_image || ''"
        :alt="anime.title"
        :sizes="posterSizes"
      />

      <!-- Rating -->
      <span
        v-if="anime.bangumi_rating || anime.rating"
        class="absolute top-2 left-2 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-bold bg-ac-sun text-white shadow font-num"
      >⭐ {{ anime.bangumi_rating || anime.rating }}</span>

      <!-- Bottom gradient + title -->
      <div class="absolute inset-x-0 bottom-0 h-1/2 bg-gradient-to-t from-ac-night/85 to-transparent pointer-events-none" />
      <div class="absolute inset-x-0 bottom-0 p-3">
        <h3 class="text-sm font-bold text-white line-clamp-2">{{ anime.title }}</h3>
        <span class="text-xs text-white/80 mt-1 inline-block font-num">{{ episodeText }}</span>
      </div>
    </div>

    <!-- Info bar -->
    <div class="px-3 py-2 text-xs text-muted-foreground flex items-center justify-between font-bold">
      <span>{{ statusText }}</span>
      <span v-if="anime.air_weekday != null">{{ WEEKDAY_NAMES[anime.air_weekday] || '' }}</span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { TrashOutline } from '@vicons/ionicons5'
import { AcPoster } from '@/components/ac'
import { useConfirm } from '@/composables/useConfirm'

const props = defineProps({ anime: { type: Object, required: true } })
const emit = defineEmits(['click', 'delete'])
const { confirm } = useConfirm()

// 追番列表卡片比日历格大一些，取图宽度相应放宽
const posterSizes = '(min-width: 1280px) 220px, (min-width: 768px) 180px, 45vw'

const WEEKDAY_NAMES = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

const statusText = computed(() => {
  const map = { ongoing: '连载中', completed: '已完结', upcoming: '即将开播', dropped: '已弃番' }
  return map[props.anime.status] || ''
})

const episodeText = computed(() => {
  const { current_episode, episode_count } = props.anime
  const total = episode_count || 0
  if (current_episode && current_episode > 0) return `${current_episode}/${total}`
  return total > 0 ? `${total}集` : ''
})

async function handleDelete() {
  const ok = await confirm({
    title: '移除追番',
    content: `确定要将《${props.anime.title}》从追番列表中移除吗？`,
    confirmText: '移除',
    cancelText: '取消',
    variant: 'danger',
  })
  if (ok) emit('delete', props.anime)
}
</script>
