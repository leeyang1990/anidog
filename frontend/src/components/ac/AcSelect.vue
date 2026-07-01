<template>
  <div ref="rootEl" class="ac-select relative" :class="block ? 'w-full' : 'inline-block'">
    <button
      ref="triggerEl"
      type="button"
      :disabled="disabled"
      class="ac-select-trigger w-full flex items-center justify-between gap-2 bg-card text-foreground border-2 border-ac-sand-dark rounded-2xl outline-none transition-all focus:border-ac-grass focus:ring-4 focus:ring-ac-grass/20 disabled:bg-ac-sand/40 disabled:cursor-not-allowed"
      :class="sizeCls"
      @click="toggle"
    >
      <span class="truncate text-left flex-1" :class="!selectedLabel ? 'text-muted-foreground' : ''">
        {{ selectedLabel || placeholder }}
      </span>
      <svg viewBox="0 0 24 24" class="w-4 h-4 transition-transform shrink-0" :class="{ 'rotate-180': open }" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
        <path d="M6 9l6 6 6-6" />
      </svg>
    </button>
    <Teleport to="body">
      <transition name="ac-pop">
        <div
          v-if="open"
          ref="menuEl"
          class="ac-select-menu fixed z-[1100] bg-card text-card-foreground border-2 border-ac-sand rounded-2xl shadow-lg py-1.5 max-h-[280px] overflow-y-auto"
          :style="menuStyle"
        >
          <button
            v-for="opt in options"
            :key="opt.value"
            type="button"
            class="w-full flex items-center justify-between gap-2 px-3 py-2 text-sm text-left transition-colors"
            :class="opt.value === modelValue ? 'bg-ac-grass-light/40 text-ac-grass-dark font-bold' : 'hover:bg-ac-sand/60'"
            :disabled="opt.disabled"
            @click="select(opt)"
          >
            <span class="truncate">{{ opt.label }}</span>
            <svg v-if="opt.value === modelValue" viewBox="0 0 24 24" class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round">
              <path d="M5 13l4 4L19 7" />
            </svg>
          </button>
          <div v-if="!options.length" class="px-3 py-4 text-center text-xs text-muted-foreground">无选项</div>
        </div>
      </transition>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, nextTick } from 'vue'
import { useClickOutside } from '../../composables/useClickOutside'

const props = defineProps({
  modelValue: { default: undefined },
  options: { type: Array, default: () => [] }, // [{label,value,disabled?}]
  placeholder: { type: String, default: '请选择' },
  size: { type: String, default: 'md' },
  disabled: { type: Boolean, default: false },
  block: { type: Boolean, default: true },
})

const emit = defineEmits(['update:modelValue', 'change'])

const rootEl = ref(null)
const triggerEl = ref(null)
const menuEl = ref(null)
const open = ref(false)
const menuStyle = ref({})

useClickOutside(rootEl, () => { open.value = false })

const selectedLabel = computed(() => {
  const opt = props.options.find(o => o.value === props.modelValue)
  return opt?.label || ''
})

const sizeCls = computed(() => ({
  sm: 'h-8 px-3 text-xs',
  md: 'h-10 px-4 text-sm',
  lg: 'h-12 px-5 text-base',
}[props.size] || 'h-10 px-4 text-sm'))

async function toggle() {
  if (props.disabled) return
  open.value = !open.value
  if (open.value) {
    await nextTick()
    updatePosition()
  }
}

function updatePosition() {
  const trig = triggerEl.value
  const menu = menuEl.value
  if (!trig || !menu) return
  const r = trig.getBoundingClientRect()
  menu.style.minWidth = `${r.width}px`
  const mh = menu.offsetHeight
  const vh = window.innerHeight
  let top = r.bottom + 4
  let left = r.left
  if (top + mh > vh - 8) top = r.top - mh - 4
  menuStyle.value = { top: `${top}px`, left: `${left}px`, minWidth: `${r.width}px` }
}

function select(opt) {
  if (opt.disabled) return
  emit('update:modelValue', opt.value)
  emit('change', opt.value, opt)
  open.value = false
}
</script>

<style scoped>
.ac-pop-enter-active,
.ac-pop-leave-active {
  transition: transform 0.18s cubic-bezier(0.34, 1.56, 0.64, 1), opacity 0.16s ease;
}
.ac-pop-enter-from,
.ac-pop-leave-to {
  opacity: 0;
  transform: scale(0.96) translateY(-4px);
}
</style>
