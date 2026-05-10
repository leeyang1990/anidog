<template>
  <div class="flex items-center justify-between mb-6 gap-4">
    <div class="min-w-0 flex-1">
      <h1 class="text-2xl font-semibold tracking-tight">{{ title }}</h1>
      <p v-if="subtitle" class="text-sm text-muted-foreground mt-1">{{ subtitle }}</p>
    </div>
    <div v-if="$slots.actions" class="flex gap-2 shrink-0">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' }
})
</script>
