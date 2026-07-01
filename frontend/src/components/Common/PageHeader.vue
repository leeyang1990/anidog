<template>
  <AcPageHeader :title="title" :subtitle="subtitle">
    <template v-if="$slots.actions" #actions>
      <slot name="actions" />
    </template>
  </AcPageHeader>
</template>

<script setup>
import { AcPageHeader } from '../ac'

defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' }
})
</script>
