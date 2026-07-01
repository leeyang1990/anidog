<template>
  <div :class="['ac-skeleton bg-ac-sand/70 rounded-xl animate-pulse', shapeCls]" :style="style" />
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  width: { type: String, default: '100%' },
  height: { type: String, default: '1rem' },
  rounded: { type: String, default: 'xl' }, // sm | md | lg | xl | full
})

const shapeCls = computed(() => ({
  sm: 'rounded-md',
  md: 'rounded-lg',
  lg: 'rounded-2xl',
  xl: 'rounded-3xl',
  full: 'rounded-full',
}[props.rounded] || 'rounded-xl'))

const style = computed(() => ({ width: props.width, height: props.height }))
</script>
