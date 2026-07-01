<template>
  <label class="ac-radio inline-flex items-center gap-2 cursor-pointer select-none" :class="{ 'opacity-50 cursor-not-allowed': disabled }">
    <span class="relative flex items-center justify-center w-5 h-5 border-2 rounded-full transition-all"
      :class="checked ? 'border-ac-grass-dark' : 'border-ac-sand-dark bg-card'">
      <span v-if="checked" class="w-2.5 h-2.5 rounded-full bg-ac-grass" />
    </span>
    <input type="radio" class="sr-only" :checked="checked" :disabled="disabled" @change="onChange" />
    <span v-if="$slots.default || label" class="text-sm text-foreground"><slot>{{ label }}</slot></span>
  </label>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: { default: undefined },
  value: { default: undefined },
  label: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'change'])

const checked = computed(() => props.modelValue === props.value)

function onChange() {
  if (props.disabled) return
  emit('update:modelValue', props.value)
  emit('change', props.value)
}
</script>
