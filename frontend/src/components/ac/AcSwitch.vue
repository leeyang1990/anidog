<template>
  <button
    type="button"
    role="switch"
    :aria-checked="modelValue"
    :disabled="disabled"
    class="ac-switch relative inline-flex items-center transition-all duration-200 rounded-full border-2 shrink-0"
    :class="[
      sizeCls,
      modelValue ? 'bg-ac-grass border-ac-grass-dark' : 'bg-ac-sand border-ac-sand-dark',
      disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
    ]"
    @click="toggle"
  >
    <span
      class="ac-thumb absolute top-1/2 -translate-y-1/2 bg-white rounded-full shadow transition-all duration-200"
      :class="[thumbSizeCls, modelValue ? translateOnCls : 'left-0.5']"
    />
  </button>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  size: { type: String, default: 'md' }, // sm | md
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'change'])

function toggle() {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
  emit('change', !props.modelValue)
}

const sizeCls = computed(() => ({
  sm: 'w-9 h-5',
  md: 'w-12 h-6',
}[props.size] || 'w-12 h-6'))

const thumbSizeCls = computed(() => ({
  sm: 'w-3.5 h-3.5',
  md: 'w-[18px] h-[18px]',
}[props.size] || 'w-[18px] h-[18px]'))

const translateOnCls = computed(() => ({
  sm: 'left-[18px]',
  md: 'left-[26px]',
}[props.size] || 'left-[26px]'))
</script>
