<template>
  <div class="ac-tabs">
    <div class="flex items-center gap-1.5 border-b-2 border-ac-sand pb-0 -mb-px overflow-x-auto" :class="centered ? 'justify-center' : ''">
      <button
        v-for="t in tabs"
        :key="t.key"
        type="button"
        class="ac-tab-btn relative px-4 py-2 text-sm font-bold whitespace-nowrap transition-all duration-150 rounded-t-2xl border-2 border-b-0"
        :class="t.key === modelValue
          ? 'bg-card text-ac-grass-dark border-ac-sand -mb-0.5 z-10 shadow-sm'
          : 'bg-transparent text-muted-foreground border-transparent hover:text-foreground hover:bg-ac-sand/40'"
        :disabled="t.disabled"
        @click="select(t)"
      >
        <component v-if="t.icon" :is="t.icon" class="inline-block w-4 h-4 mr-1 align-text-bottom" />
        {{ t.label }}
        <span v-if="t.badge !== undefined && t.badge !== null && t.badge !== ''" class="ml-1.5 inline-flex items-center justify-center min-w-[18px] h-4 px-1 rounded-full text-[10px] font-bold bg-ac-grass text-white">{{ t.badge }}</span>
      </button>
    </div>
    <div class="ac-tabs-pane pt-4">
      <slot />
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  modelValue: { default: undefined },
  tabs: { type: Array, default: () => [] }, // [{key,label,icon?,badge?,disabled?}]
  centered: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'change'])

function select(t) {
  if (t.disabled) return
  emit('update:modelValue', t.key)
  emit('change', t.key)
}
</script>
