<template>
  <div class="ac-progress w-full">
    <div class="flex items-center justify-between mb-1.5" v-if="showLabel">
      <span class="text-xs text-muted-foreground">{{ label }}</span>
      <span class="text-xs font-bold font-num text-foreground">{{ percentText }}</span>
    </div>
    <div class="relative w-full bg-ac-sand rounded-full overflow-hidden border-2 border-ac-sand-dark" :style="{ height: heightPx }">
      <div
        class="absolute inset-y-0 left-0 transition-all duration-500 ease-ac rounded-full"
        :class="colorCls"
        :style="{ width: clamped + '%' }"
      />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  value: { type: [Number, String], default: 0 },
  max: { type: [Number, String], default: 100 },
  height: { type: [Number, String], default: 8 },
  variant: { type: String, default: 'grass' }, // grass | sun | sky | heart
  showLabel: { type: Boolean, default: false },
  label: { type: String, default: '进度' },
})

const clamped = computed(() => {
  const v = Number(props.value) || 0
  const m = Number(props.max) || 100
  return Math.max(0, Math.min(100, (v / m) * 100))
})

const percentText = computed(() => `${Math.round(clamped.value)}%`)

const heightPx = computed(() => {
  const v = typeof props.height === 'number' ? props.height : parseInt(props.height, 10)
  return Number.isFinite(v) ? `${v}px` : props.height
})

const colorCls = computed(() => ({
  grass: 'bg-gradient-to-r from-ac-grass-light to-ac-grass',
  sun: 'bg-gradient-to-r from-ac-sun to-ac-sun-dark',
  sky: 'bg-gradient-to-r from-ac-sky to-ac-sky-dark',
  heart: 'bg-gradient-to-r from-ac-heart to-ac-heart-dark',
}[props.variant] || 'bg-ac-grass'))
</script>
