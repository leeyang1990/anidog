<template>
  <div class="ac-empty flex flex-col items-center justify-center text-center py-12 px-4">
    <div class="ac-empty-art mb-4 text-ac-grass-light" aria-hidden="true">
      <slot name="art">
        <!-- 动森主题：叶片 -->
        <svg v-if="skin !== 'classic'" width="92" height="92" viewBox="0 0 24 24" fill="currentColor" class="opacity-80">
          <path d="M12 2 C 5 5, 4 14, 12 22 C 20 14, 19 5, 12 2 Z" />
          <path d="M12 5 L 12 21" stroke="rgba(85,139,47,0.35)" stroke-width="0.6" fill="none" />
        </svg>
        <!-- 常规主题：中性收件箱 -->
        <svg v-else width="72" height="72" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="opacity-70">
          <path d="M4 5.5h16l1 10.5a2 2 0 0 1-2 2.2H5A2 2 0 0 1 3 16L4 5.5Z" />
          <path d="M3.5 14h4l1.5 2h6l1.5-2h4" />
        </svg>
      </slot>
    </div>
    <h3 v-if="resolvedTitle" class="text-base font-bold text-foreground mb-1">{{ resolvedTitle }}</h3>
    <p v-if="description" class="text-sm text-muted-foreground max-w-xs">{{ description }}</p>
    <div v-if="$slots.actions" class="mt-4 flex items-center justify-center gap-2"><slot name="actions" /></div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSkin } from '@/composables/useSkin'

const props = defineProps({
  title: { type: String, default: '' },
  description: { type: String, default: '' },
})

const { t } = useI18n({ useScope: 'global' })
const { skin } = useSkin()
const resolvedTitle = computed(() => props.title || t('common.empty'))
</script>
