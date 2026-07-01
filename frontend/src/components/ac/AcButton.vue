<template>
  <component
    :is="tag"
    :type="tag === 'button' ? type : undefined"
    :disabled="loading || disabled"
    class="ac-btn relative inline-flex items-center justify-center gap-1.5 select-none whitespace-nowrap font-semibold transition-all duration-150 ease-ac focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60 active:translate-y-[3px] active:shadow-none"
    :class="[sizeCls, variantCls, blockCls, roundedCls]"
    @click="onClick"
  >
    <span v-if="loading" class="inline-flex items-center justify-center"><AcSpinner :size="iconSize" :color="loaderColor" /></span>
    <slot v-else name="icon" />
    <span v-if="$slots.default" :class="{ 'opacity-70': loading }"><slot /></span>
  </component>
</template>

<script setup>
import { computed } from 'vue'
import AcSpinner from './AcSpinner.vue'

const props = defineProps({
  variant: { type: String, default: 'primary' }, // primary | secondary | ghost | danger | sun | sky
  size: { type: String, default: 'md' },         // sm | md | lg | xl
  type: { type: String, default: 'button' },
  tag: { type: String, default: 'button' },
  block: { type: Boolean, default: false },
  round: { type: Boolean, default: false },
  loading: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['click'])

function onClick(e) {
  if (props.loading || props.disabled) return
  emit('click', e)
}

const sizeCls = computed(() => ({
  sm: 'h-8 px-3 text-xs gap-1 [&_svg]:w-3.5 [&_svg]:h-3.5',
  md: 'h-10 px-4 text-sm',
  lg: 'h-12 px-6 text-base',
  xl: 'h-14 px-8 text-lg',
}[props.size] || 'h-10 px-4 text-sm'))

const iconSize = computed(() => ({ sm: 14, md: 16, lg: 18, xl: 20 }[props.size] || 16))

const blockCls = computed(() => props.block ? 'w-full' : '')
const roundedCls = computed(() => props.round ? 'rounded-full' : 'rounded-2xl')

const variantCls = computed(() => {
  const map = {
    primary: 'bg-ac-grass text-white shadow-ac-button hover:bg-ac-grass-dark hover:-translate-y-px',
    secondary: 'bg-ac-sand text-ac-earth shadow-ac-button-secondary hover:bg-ac-sand-dark hover:-translate-y-px',
    ghost: 'bg-transparent text-ac-earth hover:bg-ac-sand/60 active:bg-ac-sand active:translate-y-0',
    danger: 'bg-ac-heart text-white shadow-ac-button-heart hover:bg-ac-heart-dark hover:-translate-y-px',
    sun: 'bg-ac-sun text-white shadow-ac-button-sun hover:bg-ac-sun-dark hover:-translate-y-px',
    sky: 'bg-ac-sky text-ac-earth hover:bg-ac-sky-dark hover:text-white hover:-translate-y-px',
    outline: 'bg-card text-foreground border-2 border-ac-sand-dark hover:bg-ac-sand/60 hover:-translate-y-px',
  }
  return map[props.variant] || map.primary
})

const loaderColor = computed(() => {
  if (['primary', 'danger', 'sun'].includes(props.variant)) return '#FFFDF7'
  return '#7CB342'
})
</script>

<style scoped>
.ac-btn {
  -webkit-tap-highlight-color: transparent;
}
.ac-btn:active:not(:disabled) {
  box-shadow: none !important;
}
</style>
