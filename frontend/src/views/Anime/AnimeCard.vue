<template>
  <div
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
      <!-- 评分 -->
      <span v-if="item.rating_score" class="absolute top-1.5 left-1.5 rounded-full px-2 py-0.5 text-[10px] font-bold bg-ac-sun text-white shadow font-num">
        ⭐ {{ item.rating_score.toFixed(1) }}
      </span>
      <!-- 已追番标记 -->
      <span v-if="item.is_subscribed" class="absolute top-1.5 right-1.5 rounded-full px-2 py-0.5 text-[10px] font-bold bg-ac-grass text-white shadow">
        🌿 已追
      </span>
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
    <div class="px-2 py-2">
      <h3 class="text-xs font-bold line-clamp-1 leading-snug text-foreground">{{ item.name_cn || item.name }}</h3>
      <p v-if="item.air_date" class="text-[10px] text-muted-foreground mt-0.5 font-num">{{ item.air_date }}</p>
    </div>
  </div>
</template>

<script setup>
import { FilmOutline } from '@vicons/ionicons5'
import { toResizedImage } from '@/utils/image'

defineProps({ item: { type: Object, required: true } })
defineEmits(['click', 'subscribe'])
</script>
