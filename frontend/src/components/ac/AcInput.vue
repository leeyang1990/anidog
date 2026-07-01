<template>
  <label class="ac-input-wrap relative inline-flex items-center w-full" :class="[wrapCls]">
    <span v-if="$slots.prefix || prefixIcon" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none flex items-center">
      <slot name="prefix" />
    </span>
    <input
      :type="type"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      :readonly="readonly"
      :autocomplete="autocomplete"
      class="ac-input w-full bg-card text-foreground border-2 border-ac-sand-dark rounded-2xl outline-none transition-all duration-150 placeholder:text-muted-foreground/70 disabled:bg-ac-sand/40 disabled:cursor-not-allowed focus:border-ac-grass focus:ring-4 focus:ring-ac-grass/20"
      :class="[sizeCls, paddingCls]"
      v-bind="$attrs"
      @input="onInput"
      @change="$emit('change', $event.target.value)"
      @keydown.enter="$emit('keyup-enter', $event)"
      @blur="$emit('blur', $event)"
      @focus="$emit('focus', $event)"
    />
    <span v-if="$slots.suffix || clearable && modelValue" class="absolute right-3 top-1/2 -translate-y-1/2 flex items-center gap-1">
      <button
        v-if="clearable && modelValue"
        type="button"
        class="size-5 rounded-full bg-ac-wood/20 text-ac-wood text-xs flex items-center justify-center hover:bg-ac-wood/40 transition-colors"
        @click="clear"
        tabindex="-1"
      >×</button>
      <slot name="suffix" />
    </span>
  </label>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: { type: [String, Number], default: '' },
  type: { type: String, default: 'text' },
  size: { type: String, default: 'md' }, // sm | md | lg
  placeholder: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
  readonly: { type: Boolean, default: false },
  clearable: { type: Boolean, default: false },
  autocomplete: { type: String, default: 'off' },
  prefixIcon: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'change', 'keyup-enter', 'blur', 'focus'])

function onInput(e) {
  emit('update:modelValue', e.target.value)
}

function clear() {
  emit('update:modelValue', '')
}

const wrapCls = computed(() => '')

const sizeCls = computed(() => ({
  sm: 'h-8 text-xs',
  md: 'h-10 text-sm',
  lg: 'h-12 text-base',
}[props.size] || 'h-10 text-sm'))

// padding 跟 prefix/suffix 走
import { useSlots } from 'vue'
const slots = useSlots()
const paddingCls = computed(() => {
  const hasPrefix = !!slots.prefix || props.prefixIcon
  const hasSuffix = !!slots.suffix || (props.clearable && props.modelValue)
  return [
    hasPrefix ? 'pl-10' : 'pl-4',
    hasSuffix ? 'pr-10' : 'pr-4',
  ].join(' ')
})
</script>

<style scoped>
.ac-input::-webkit-search-decoration,
.ac-input::-webkit-search-cancel-button {
  display: none;
}
</style>
