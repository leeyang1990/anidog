<template>
  <span
    class="inline-flex items-center justify-center shrink-0 align-middle"
    :style="{ width: sizePx, height: sizePx, color: color || undefined }"
    aria-hidden="true"
  >
    <slot />
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  size: { type: [Number, String], default: 18 },
  color: { type: String, default: '' },
})

const sizePx = computed(() => {
  const v = typeof props.size === 'number' ? props.size : parseInt(props.size, 10)
  return Number.isFinite(v) ? `${v}px` : props.size
})
</script>

<style scoped>
span :deep(svg) {
  width: 100%;
  height: 100%;
  fill: currentColor;
  stroke: currentColor;
  stroke-width: 0;
}
</style>
