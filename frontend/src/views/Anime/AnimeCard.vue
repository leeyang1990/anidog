<template>
  <div
    class="bg-card rounded-lg border overflow-hidden hover:shadow-md transition-all cursor-pointer group"
    @click="$emit('click', item)"
  >
    <div class="relative aspect-[2/3] overflow-hidden bg-muted">
      <img v-if="item.image" :src="toResizedImage(item.image, 600)" :alt="item.name_cn || item.name"
        class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
        loading="lazy"
        @error="($event.target).style.display='none'" />
      <div v-else class="w-full h-full flex items-center justify-center text-muted-foreground">
        <n-icon size="20"><FilmOutline /></n-icon>
      </div>
      <!-- 评分 -->
      <span v-if="item.rating_score" class="absolute top-1.5 left-1.5 rounded px-1.5 py-0.5 text-[10px] font-bold bg-amber-500 text-white shadow">
        {{ item.rating_score.toFixed(1) }}
      </span>
      <!-- 已追番标记 -->
      <span v-if="item.is_subscribed" class="absolute top-1.5 right-1.5 rounded px-1.5 py-0.5 text-[10px] font-medium bg-primary text-primary-foreground shadow">
        已追
      </span>
      <!-- 追番按钮（悬浮） -->
      <div v-if="!item.is_subscribed" class="absolute bottom-0 inset-x-0 bg-gradient-to-t from-black/70 to-transparent p-1.5 opacity-0 group-hover:opacity-100 transition-opacity">
        <button
          class="w-full h-6 rounded bg-primary/90 text-primary-foreground text-[11px] font-medium hover:bg-primary transition-colors"
          @click.stop="$emit('subscribe', item)"
        >
          + 追番
        </button>
      </div>
    </div>
    <div class="px-2 py-1.5">
      <h3 class="text-xs font-medium line-clamp-1 leading-snug">{{ item.name_cn || item.name }}</h3>
      <p v-if="item.air_date" class="text-[10px] text-muted-foreground mt-0.5">{{ item.air_date }}</p>
    </div>
  </div>
</template>

<script setup>
import { NIcon } from 'naive-ui'
import { FilmOutline } from '@vicons/ionicons5'
import { toResizedImage } from '@/utils/image'

defineProps({
  item: { type: Object, required: true }
})

defineEmits(['click', 'subscribe'])
</script>
