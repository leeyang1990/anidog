<template>
  <div class="ac-tabs">
    <div ref="barEl" class="ui-tabbar relative flex items-center gap-1.5 border-b-2 border-ac-sand pb-0 -mb-px overflow-x-auto" :class="centered ? 'justify-center' : ''">
      <span class="ac-tab-marker" :style="markerStyle" aria-hidden="true" />
      <button
        v-for="t in tabs"
        :key="t.key"
        type="button"
        :ref="(el) => setTabRef(t.key, el)"
        class="ac-tab-btn relative px-4 py-2 text-sm font-bold whitespace-nowrap transition-all duration-150 rounded-t-2xl border-2 border-b-0"
        :class="t.key === modelValue
          ? 'bg-card text-ac-grass-dark border-ac-sand -mb-0.5 z-10 shadow-sm'
          : 'bg-transparent text-muted-foreground border-transparent hover:text-foreground hover:bg-ac-sand/40'"
        :disabled="t.disabled"
        :aria-pressed="t.key === modelValue"
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
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
const props = defineProps({
  modelValue: { default: undefined },
  tabs: { type: Array, default: () => [] }, // [{key,label,icon?,badge?,disabled?}]
  centered: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'change'])

// 选中的标签下划线在标签之间滑动，而不是整块硬切
const barEl = ref(null)
const tabEls = new Map()
const marker = ref({ left: 0, width: 0, visible: false })

function setTabRef(key, el) {
  if (el) tabEls.set(key, el)
  else tabEls.delete(key)
}

function syncMarker() {
  const el = tabEls.get(props.modelValue)
  if (!el || !barEl.value) {
    marker.value = { ...marker.value, visible: false }
    return
  }
  marker.value = { left: el.offsetLeft, width: el.offsetWidth, visible: true }
}

const markerStyle = computed(() => ({
  transform: `translateX(${marker.value.left}px)`,
  width: `${marker.value.width}px`,
  opacity: marker.value.visible ? 1 : 0,
}))

watch([() => props.modelValue, () => props.tabs.length, () => props.centered], async () => {
  await nextTick()
  syncMarker()
})

let tabResizeObserver = null
onMounted(async () => {
  await nextTick()
  syncMarker()
  if (typeof ResizeObserver !== 'undefined' && barEl.value) {
    tabResizeObserver = new ResizeObserver(() => syncMarker())
    tabResizeObserver.observe(barEl.value)
  }
})
onBeforeUnmount(() => tabResizeObserver?.disconnect())

function select(t) {
  if (t.disabled) return
  emit('update:modelValue', t.key)
  emit('change', t.key)
}
</script>
