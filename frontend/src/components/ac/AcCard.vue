<template>
  <div
    :class="[
      'ac-card relative bg-card text-card-foreground border-2 border-ac-sand transition-all duration-200 ease-ac',
      paddingCls,
      roundedCls,
      hoverable ? 'hover:-translate-y-0.5 hover:shadow-lg' : '',
      shadowCls,
    ]"
  >
    <div v-if="$slots.header || title" class="flex items-center justify-between gap-3 mb-4">
      <div class="flex items-center gap-3 min-w-0">
        <slot name="icon" />
        <div class="min-w-0">
          <h3 v-if="title" class="text-base font-bold tracking-tight text-foreground truncate">{{ title }}</h3>
          <p v-if="subtitle" class="text-xs text-muted-foreground mt-0.5 truncate">{{ subtitle }}</p>
        </div>
      </div>
      <div class="shrink-0 flex items-center gap-2"><slot name="extra" /></div>
    </div>
    <slot />
    <div v-if="$slots.footer" class="mt-4 pt-3 border-t-2 border-dashed border-ac-sand"><slot name="footer" /></div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  title: { type: String, default: '' },
  subtitle: { type: String, default: '' },
  hoverable: { type: Boolean, default: false },
  padding: { type: String, default: 'md' }, // none | sm | md | lg
  rounded: { type: String, default: '2xl' }, // lg | xl | 2xl | 3xl
  shadow: { type: String, default: 'md' }, // none | sm | md | lg
})

const paddingCls = computed(() => ({
  none: 'p-0',
  sm: 'p-3',
  md: 'p-5',
  lg: 'p-6 md:p-7',
}[props.padding] || 'p-5'))

const roundedCls = computed(() => ({
  lg: 'rounded-2xl',
  xl: 'rounded-3xl',
  '2xl': 'rounded-3xl',
  '3xl': 'rounded-[32px]',
}[props.rounded] || 'rounded-3xl'))

const shadowCls = computed(() => ({
  none: 'shadow-none',
  sm: 'shadow-sm',
  md: 'shadow-md',
  lg: 'shadow-lg',
}[props.shadow] || 'shadow-md'))
</script>
