<template>
  <label class="ac-checkbox inline-flex items-center gap-2 cursor-pointer select-none" :class="{ 'opacity-50 cursor-not-allowed': disabled }">
    <span class="relative flex items-center justify-center w-5 h-5 border-2 rounded-md transition-all"
      :class="checked ? 'bg-ac-grass border-ac-grass-dark' : 'bg-card border-ac-sand-dark'">
      <svg v-if="checked" viewBox="0 0 24 24" class="w-4 h-4 text-white" fill="none" stroke="currentColor" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M5 13l4 4L19 7" />
      </svg>
    </span>
    <input
      type="checkbox"
      class="sr-only"
      :checked="checked"
      :disabled="disabled"
      @change="onChange"
    />
    <span v-if="$slots.default || label" class="text-sm text-foreground"><slot>{{ label }}</slot></span>
  </label>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: { type: [Boolean, Array], default: false },
  value: { default: undefined },        // 数组模式时此项的值
  label: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'change'])

const checked = computed(() => {
  if (Array.isArray(props.modelValue)) {
    return props.modelValue.includes(props.value)
  }
  return !!props.modelValue
})

function onChange(e) {
  if (props.disabled) return
  if (Array.isArray(props.modelValue)) {
    const next = e.target.checked
      ? [...props.modelValue, props.value]
      : props.modelValue.filter(v => v !== props.value)
    emit('update:modelValue', next)
  } else {
    emit('update:modelValue', e.target.checked)
  }
  emit('change', e.target.checked)
}
</script>
