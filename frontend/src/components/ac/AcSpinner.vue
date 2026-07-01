<template>
  <span class="inline-flex items-center justify-center" :style="{ width: sizePx, height: sizePx }" role="status" :aria-label="ariaLabel">
    <svg :width="sizePx" :height="sizePx" viewBox="0 0 24 24" class="animate-spin-leaf">
      <!-- 一片转圈的小叶子 -->
      <path
        d="M12 2 C 6 6, 6 14, 12 22 C 18 14, 18 6, 12 2 Z"
        :fill="color"
        opacity="0.85"
      />
      <path
        d="M12 4 L 12 20"
        stroke="rgba(85,139,47,0.6)"
        stroke-width="0.8"
        fill="none"
      />
    </svg>
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  size: { type: [Number, String], default: 24 },
  color: { type: String, default: '#7CB342' }, // ac-grass
  ariaLabel: { type: String, default: '加载中' },
})

const sizePx = computed(() => {
  const v = typeof props.size === 'number' ? props.size : parseInt(props.size, 10)
  return Number.isFinite(v) ? `${v}px` : props.size
})
</script>
