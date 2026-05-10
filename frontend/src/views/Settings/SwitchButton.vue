<template>
  <button type="button" role="switch" :aria-checked="modelValue"
    class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors"
    :class="modelValue ? 'bg-primary' : 'bg-input'"
    @click="$emit('update:modelValue', !modelValue)">
    <span class="pointer-events-none block h-5 w-5 rounded-full bg-background shadow-lg ring-0 transition-transform"
      :class="modelValue ? 'translate-x-5' : 'translate-x-0'" />
  </button>
</template>

<script setup>
defineProps({ modelValue: { type: Boolean, default: false } })
defineEmits(['update:modelValue'])
</script>
