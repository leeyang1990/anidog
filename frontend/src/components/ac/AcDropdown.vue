<template>
  <div ref="rootEl" class="ac-dropdown relative inline-block">
    <div ref="triggerEl" @click="toggle">
      <slot name="trigger" :open="open" />
    </div>
    <Teleport to="body">
      <transition name="ac-pop">
        <div
          v-if="open"
          ref="menuEl"
          class="ac-dropdown-menu fixed z-[1100] min-w-[160px] bg-card text-card-foreground border-2 border-ac-sand rounded-2xl shadow-lg py-1.5 origin-top"
          :style="menuStyle"
        >
          <template v-for="(item, i) in options" :key="item.key ?? i">
            <div v-if="item.type === 'divider'" class="my-1 border-t-2 border-dashed border-ac-sand" />
            <button
              v-else
              type="button"
              class="w-full flex items-center gap-2 px-3 py-2 text-sm text-foreground hover:bg-ac-grass-light/30 transition-colors text-left"
              :class="item.danger ? 'text-ac-heart-dark hover:bg-ac-heart/15' : ''"
              :disabled="item.disabled"
              @click="onSelect(item)"
            >
              <component v-if="item.icon" :is="item.icon" class="w-4 h-4" />
              <span>{{ item.label }}</span>
            </button>
          </template>
          <div v-if="$slots.default" class="py-1">
            <slot :close="close" />
          </div>
        </div>
      </transition>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, nextTick } from 'vue'
import { useClickOutside } from '../../composables/useClickOutside'

const props = defineProps({
  options: { type: Array, default: () => [] },
  placement: { type: String, default: 'bottom-end' }, // bottom-start | bottom-end | top-start | top-end
})

const emit = defineEmits(['select'])

const rootEl = ref(null)
const triggerEl = ref(null)
const menuEl = ref(null)
const open = ref(false)
const menuStyle = ref({})

useClickOutside(rootEl, () => { open.value = false })

async function toggle() {
  open.value = !open.value
  if (open.value) {
    await nextTick()
    updatePosition()
  }
}

function close() { open.value = false }

function updatePosition() {
  const trig = triggerEl.value
  const menu = menuEl.value
  if (!trig || !menu) return
  const r = trig.getBoundingClientRect()
  const mw = menu.offsetWidth
  const mh = menu.offsetHeight
  let top, left
  switch (props.placement) {
    case 'bottom-start':
      top = r.bottom + 6; left = r.left; break
    case 'bottom-end':
      top = r.bottom + 6; left = r.right - mw; break
    case 'top-start':
      top = r.top - mh - 6; left = r.left; break
    case 'top-end':
      top = r.top - mh - 6; left = r.right - mw; break
    default:
      top = r.bottom + 6; left = r.left
  }
  // 视口边界保护
  const vw = window.innerWidth
  const vh = window.innerHeight
  if (left + mw > vw - 8) left = vw - mw - 8
  if (left < 8) left = 8
  if (top + mh > vh - 8) top = r.top - mh - 6
  if (top < 8) top = 8
  menuStyle.value = { top: `${top}px`, left: `${left}px` }
}

function onSelect(item) {
  if (item.disabled) return
  emit('select', item.key, item)
  close()
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
  transform: scale(0.92) translateY(-4px);
}
</style>
