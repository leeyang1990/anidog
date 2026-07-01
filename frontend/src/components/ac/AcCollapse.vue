<template>
  <div class="ac-collapse divide-y-2 divide-dashed divide-ac-sand">
    <div v-for="item in items" :key="item.key" class="py-1">
      <button
        type="button"
        class="w-full flex items-center justify-between gap-3 py-3 text-left text-sm font-bold text-foreground hover:text-ac-grass-dark transition-colors"
        @click="toggle(item.key)"
      >
        <span class="flex items-center gap-2">
          <component v-if="item.icon" :is="item.icon" class="w-4 h-4" />
          {{ item.label }}
        </span>
        <svg viewBox="0 0 24 24" class="w-4 h-4 transition-transform" :class="{ 'rotate-180': isOpen(item.key) }" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
          <path d="M6 9l6 6 6-6" />
        </svg>
      </button>
      <div v-show="isOpen(item.key)" class="pb-3 text-sm text-muted-foreground">
        <slot :name="item.key">{{ item.content }}</slot>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  items: { type: Array, default: () => [] }, // [{key,label,icon?,content?}]
  modelValue: { type: Array, default: () => [] }, // 当前展开的 key 列表
  accordion: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue'])

const internal = ref([...props.modelValue])

const opened = computed(() => props.modelValue?.length ? props.modelValue : internal.value)

function isOpen(key) { return opened.value.includes(key) }

function toggle(key) {
  let next
  if (isOpen(key)) {
    next = opened.value.filter(k => k !== key)
  } else {
    next = props.accordion ? [key] : [...opened.value, key]
  }
  internal.value = next
  emit('update:modelValue', next)
}
</script>
