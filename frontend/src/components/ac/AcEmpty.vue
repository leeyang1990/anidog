<template>
  <div class="ac-empty flex flex-col items-center justify-center text-center py-12 px-4">
    <div class="ac-empty-art mb-4 text-ac-grass-light" aria-hidden="true">
      <slot name="art">
        <!-- 默认插画：一片大叶子 -->
        <svg width="92" height="92" viewBox="0 0 24 24" fill="currentColor" class="opacity-80">
          <path d="M12 2 C 5 5, 4 14, 12 22 C 20 14, 19 5, 12 2 Z" />
          <path d="M12 5 L 12 21" stroke="rgba(85,139,47,0.35)" stroke-width="0.6" fill="none" />
        </svg>
      </slot>
    </div>
    <h3 v-if="title" class="text-base font-bold text-foreground mb-1">{{ title }}</h3>
    <p v-if="description" class="text-sm text-muted-foreground max-w-xs">{{ description }}</p>
    <div v-if="$slots.actions" class="mt-4 flex items-center justify-center gap-2"><slot name="actions" /></div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, default: '什么也没有~' },
  description: { type: String, default: '' },
})
</script>
