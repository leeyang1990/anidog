<template>
  <div
    class="ui-panel group relative flex flex-col overflow-hidden rounded-xl border border-border/60 bg-card cursor-pointer transition-[transform,box-shadow,border-color] duration-200 ease-out hover:-translate-y-1 hover:border-ac-grass/40 hover:shadow-lg"
    @click="$emit('click')"
  >
    <!-- 删除按钮 (hover 显示) -->
    <button
      type="button"
      class="absolute top-2 right-2 z-10 flex size-7 items-center justify-center rounded-full bg-card/85 text-muted-foreground opacity-0 shadow-sm backdrop-blur-md transition-all hover:bg-ac-heart hover:text-white group-hover:opacity-100"
      title="从追番列表移除"
      @click.stop="handleDelete"
    >
      <TrashOutline class="size-3.5" />
    </button>

    <!-- Cover -->
    <div class="relative aspect-[2/3] overflow-hidden bg-muted/40">
      <AcPoster
        :src="anime.cover_url || anime.cover_image || ''"
        :alt="anime.title"
        :sizes="posterSizes"
      />

      <!-- Rating -->
      <span
        v-if="anime.bangumi_rating || anime.rating"
        class="ac-poster-badge absolute left-2 top-2 font-num"
      >
        <Star class="size-2.5 text-ac-sun" aria-hidden="true" />
        {{ anime.bangumi_rating || anime.rating }}
      </span>

      <!-- Bottom gradient + title -->
      <div class="absolute inset-x-0 bottom-0 h-1/2 bg-gradient-to-t from-ac-night/85 to-transparent pointer-events-none" />
      <div class="absolute inset-x-0 bottom-0 p-3">
        <h3 class="text-sm font-bold text-white line-clamp-2">{{ anime.title }}</h3>
        <span class="text-xs text-white/80 mt-1 inline-block font-num">{{ episodeText }}</span>
      </div>
    </div>

    <!-- Info bar -->
    <div class="flex items-center justify-between px-3 py-2 text-xs font-bold text-muted-foreground">
      <span>{{ statusText }}</span>
      <span v-if="anime.air_weekday != null">{{ WEEKDAY_NAMES[anime.air_weekday] || '' }}</span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Star, TrashOutline } from '@vicons/ionicons5'
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
