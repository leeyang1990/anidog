<template>
  <div class="ac-page-header flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-6">
    <div class="min-w-0">
      <h1 class="text-2xl font-bold text-foreground tracking-tight truncate">
        <slot name="title">{{ title }}</slot>
      </h1>
      <p v-if="subtitle || $slots.subtitle" class="text-sm text-muted-foreground mt-1">
        <slot name="subtitle">{{ subtitle }}</slot>
      </p>
    </div>
    <div class="flex items-center gap-2 shrink-0 flex-wrap">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, default: '' },
  subtitle: { type: String, default: '' },
})
</script>
