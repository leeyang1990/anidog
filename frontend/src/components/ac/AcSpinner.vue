<template>
  <span class="inline-flex items-center justify-center" :style="{ width: sizePx, height: sizePx }" role="status" :aria-label="resolvedAriaLabel">
    <svg v-if="skin !== 'classic'" :width="sizePx" :height="sizePx" viewBox="0 0 24 24" class="ac-spinner-leaf animate-spin-leaf">
      <!-- 一片转圈的小叶子 -->
      <path
        d="M12 2 C 6 6, 6 14, 12 22 C 18 14, 18 6, 12 2 Z"
        :fill="resolvedColor"
        opacity="0.85"
      />
      <path
        d="M12 4 L 12 20"
        :stroke="resolvedColor"
        stroke-width="0.8"
        fill="none"
      />
    </svg>
    <svg v-else :width="sizePx" :height="sizePx" viewBox="0 0 24 24" class="ac-spinner-ring animate-spin" fill="none">
      <circle cx="12" cy="12" r="9" :stroke="resolvedColor" stroke-width="3" opacity="0.2" />
      <path d="M12 3a9 9 0 0 1 9 9" :stroke="resolvedColor" stroke-width="3" stroke-linecap="round" />
    </svg>
  </span>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSkin } from '@/composables/useSkin'

const props = defineProps({
  size: { type: [Number, String], default: 24 },
  color: { type: String, default: '' },
  ariaLabel: { type: String, default: '' },
})

const { t } = useI18n({ useScope: 'global' })
const { skin } = useSkin()
const resolvedColor = computed(() => props.color || 'hsl(var(--primary))')
const resolvedAriaLabel = computed(() => props.ariaLabel || t('common.loading'))

const sizePx = computed(() => {
  const v = typeof props.size === 'number' ? props.size : parseInt(props.size, 10)
  return Number.isFinite(v) ? `${v}px` : props.size
})
</script>
