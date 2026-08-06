<template>
  <article
    class="bg-card rounded-2xl border-2 border-ac-sand overflow-hidden hover:shadow-lg hover:-translate-y-0.5 hover:border-ac-grass transition-all duration-200 cursor-pointer group"
    @click="$emit('click', item)"
  >
    <div class="relative aspect-[2/3] overflow-hidden bg-ac-sand/40">
      <img v-if="item.image" :src="toResizedImage(item.image, 600)" :alt="item.name_cn || item.name"
        class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
        loading="lazy"
        @error="($event.target).style.display='none'" />
      <div v-else class="w-full h-full flex items-center justify-center text-ac-wood-dark">
        <FilmOutline class="size-6" />
      </div>
      <div class="pointer-events-none absolute inset-x-0 top-0 h-16 bg-gradient-to-b from-ac-night/35 to-transparent" />

      <!-- 海报状态角标：统一使用高对比胶囊，不受海报底色影响 -->
      <div class="absolute top-1.5 inset-x-1.5 xl:top-2 xl:inset-x-2 flex items-start justify-between gap-1 xl:gap-1.5 pointer-events-none">
        <span
          v-if="item.rating_score"
          class="inline-flex h-5 xl:h-6 shrink-0 items-center gap-0.5 xl:gap-1 rounded-full border border-white/45 bg-ac-night/75 px-1.5 xl:px-2 text-[10px] xl:text-[11px] font-extrabold leading-none text-white shadow-md backdrop-blur-sm font-num"
          :aria-label="`评分 ${formatRating(item.rating_score)}`"
        >
          <Star class="size-3 xl:size-3.5 text-ac-sun" aria-hidden="true" />
          {{ formatRating(item.rating_score) }}
        </span>
        <span v-else />

        <span
          v-if="item.is_subscribed"
          class="inline-flex size-5 xl:size-auto xl:h-6 shrink-0 items-center justify-center xl:gap-1 rounded-full border border-white/45 bg-ac-grass-dark/90 xl:px-2 xl:text-[11px] font-bold leading-none text-white shadow-md backdrop-blur-sm"
          aria-label="已追番"
        >
          <CheckmarkCircle class="size-3 xl:size-3.5" aria-hidden="true" />
          <span class="hidden xl:inline">已追</span>
        </span>
      </div>
      <!-- 追番按钮（悬浮） -->
      <div v-if="!item.is_subscribed" class="absolute bottom-0 inset-x-0 bg-gradient-to-t from-ac-night/70 to-transparent p-1.5 opacity-0 group-hover:opacity-100 transition-opacity">
        <button
          type="button"
          class="w-full h-7 rounded-xl bg-ac-grass text-white text-[11px] font-bold hover:bg-ac-grass-dark transition-colors"
          @click.stop="$emit('subscribe', item)"
        >
          + 追番
        </button>
      </div>
    </div>
    <div class="px-2.5 py-2.5 min-h-[3.75rem]">
      <h3 class="text-xs font-bold line-clamp-2 leading-snug text-foreground" :title="item.name_cn || item.name">
        {{ item.name_cn || item.name }}
      </h3>
      <p v-if="item.air_date" class="text-[10px] text-muted-foreground mt-1 font-num">{{ item.air_date }}</p>
    </div>
  </article>
</template>

<script setup>
import { CheckmarkCircle, FilmOutline, Star } from '@vicons/ionicons5'
import { toResizedImage } from '@/utils/image'

defineProps({ item: { type: Object, required: true } })
defineEmits(['click', 'subscribe'])

function formatRating(value) {
  const rating = Number(value)
  return Number.isFinite(rating) ? rating.toFixed(1) : value
}
</script>
