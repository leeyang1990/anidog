<template>{{ text }}</template>

<script setup>
import { computed, toRef } from 'vue'
import { useCountUp } from '@/composables/useCountUp'

// 数字滚动文本。默认按整数展示，传 format 可以接我们自己的 formatSpeed / formatSize。
const props = defineProps({
  value: { type: Number, required: true },
  duration: { type: Number, default: 600 },
  format: { type: Function, default: null },
  // 数据到达后才有数字的场景（日历每天部数、看板统计）打开它，从 0 滚上来
  fromZero: { type: Boolean, default: false },
})

const source = toRef(props, 'value')
const { display } = useCountUp(() => source.value, {
  duration: props.duration,
  animateOnMount: props.fromZero,
})

const text = computed(() => {
  const value = source.value
  const shown = Number.isFinite(display.value) ? display.value : value
  if (props.format) return props.format(shown)
  if (Number.isFinite(value) && !Number.isInteger(value)) return shown.toFixed(1)
  return String(Math.round(shown))
})
</script>
