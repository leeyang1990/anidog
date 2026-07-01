<template>
  <textarea
    :value="modelValue"
    :placeholder="placeholder"
    :rows="rows"
    :disabled="disabled"
    :readonly="readonly"
    class="ac-textarea w-full bg-card text-foreground border-2 border-ac-sand-dark rounded-2xl px-4 py-3 outline-none transition-all duration-150 placeholder:text-muted-foreground/70 disabled:bg-ac-sand/40 disabled:cursor-not-allowed focus:border-ac-grass focus:ring-4 focus:ring-ac-grass/20 resize-y text-sm leading-relaxed"
    v-bind="$attrs"
    @input="$emit('update:modelValue', $event.target.value)"
  />
</template>

<script setup>
defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: '' },
  rows: { type: [Number, String], default: 4 },
  disabled: { type: Boolean, default: false },
  readonly: { type: Boolean, default: false },
})
defineEmits(['update:modelValue'])
</script>
