<template>
  <div
    class="group relative bg-card text-card-foreground rounded-lg border overflow-hidden cursor-pointer hover:shadow-md transition-shadow"
    @click="$emit('click')"
  >
    <!-- 删除按钮 (hover 显示) -->
    <button
      class="absolute top-2 right-2 z-10 h-8 w-8 rounded-full bg-background/80 backdrop-blur-sm border flex items-center justify-center opacity-0 group-hover:opacity-100 hover:bg-red-500 hover:text-white hover:border-red-500 transition-all shadow-sm"
      title="从追番列表移除"
      @click.stop="handleDelete"
    >
      <n-icon size="14"><TrashOutline /></n-icon>
    </button>

    <!-- Cover -->
    <div class="relative aspect-[2/3] overflow-hidden bg-muted">
      <img
        :src="anime.cover_url || anime.cover_image || ''"
        :alt="anime.title"
        class="h-full w-full object-cover group-hover:scale-105 transition-transform duration-300"
        @error="($event.target).style.display='none'"
      />

      <!-- Rating -->
      <span
        v-if="anime.bangumi_rating || anime.rating"
        class="absolute top-2 left-2 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-bold bg-amber-500 text-white shadow"
      >{{ anime.bangumi_rating || anime.rating }}</span>

      <!-- Bottom gradient + title -->
      <div class="absolute inset-x-0 bottom-0 h-1/2 bg-gradient-to-t from-black/80 to-transparent pointer-events-none" />
      <div class="absolute inset-x-0 bottom-0 p-3">
        <h3 class="text-sm font-medium text-white line-clamp-2">{{ anime.title }}</h3>
        <span class="text-xs text-white/70 mt-1 inline-block">{{ episodeText }}</span>
      </div>
    </div>

    <!-- Info bar -->
    <div class="px-3 py-2 text-xs text-muted-foreground flex items-center justify-between">
      <span>{{ statusText }}</span>
      <span v-if="anime.air_weekday != null">{{ WEEKDAY_NAMES[anime.air_weekday] || '' }}</span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { NIcon, useDialog } from 'naive-ui'
import { TrashOutline } from '@vicons/ionicons5'

const props = defineProps({
  anime: { type: Object, required: true }
})

const emit = defineEmits(['click', 'delete'])
const dialog = useDialog()

const WEEKDAY_NAMES = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

const statusText = computed(() => {
  const map = { ongoing: '连载中', completed: '已完结', upcoming: '即将开播', dropped: '已弃番' }
  return map[props.anime.status] || ''
})

const episodeText = computed(() => {
  const { current_episode, episode_count } = props.anime
  const total = episode_count || 0
  if (current_episode && current_episode > 0) {
    return `${current_episode}/${total}`
  }
  return total > 0 ? `${total}集` : ''
})

function handleDelete() {
  dialog.warning({
    title: '移除追番',
    content: `确定要将《${props.anime.title}》从追番列表中移除吗？`,
    positiveText: '移除',
    negativeText: '取消',
    onPositiveClick: () => emit('delete', props.anime)
  })
}
</script>
