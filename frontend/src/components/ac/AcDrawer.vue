<template>
  <Teleport to="body">
    <transition name="ac-drawer">
      <div v-if="show" class="fixed inset-0 z-[1000] flex" :class="placement === 'right' ? 'justify-end' : 'justify-start'">
        <div class="absolute inset-0 bg-ac-night/40 backdrop-blur-sm" @click="onMaskClick" />
        <div
          ref="drawerEl"
          class="relative bg-card text-card-foreground border-2 border-ac-sand h-full flex flex-col overflow-hidden"
          :class="placement === 'right' ? 'rounded-l-[32px] border-r-0' : 'rounded-r-[32px] border-l-0'"
          :style="{ width: width, maxWidth: '92vw' }"
        >
          <div v-if="title || $slots.header" class="px-6 py-4 border-b-2 border-dashed border-ac-sand flex items-center justify-between gap-3">
            <slot name="header">
              <h2 class="text-base font-bold text-foreground truncate">{{ title }}</h2>
            </slot>
            <button
              type="button"
              class="shrink-0 size-9 rounded-full bg-ac-heart text-white flex items-center justify-center font-bold text-lg shadow-md hover:bg-ac-heart-dark transition-colors"
              @click="close"
              aria-label="关闭"
            >×</button>
          </div>
          <div class="flex-1 overflow-y-auto px-6 py-5">
            <slot />
          </div>
          <div v-if="$slots.footer" class="px-6 py-4 border-t-2 border-dashed border-ac-sand bg-ac-cream/30">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup>
import { ref, watch, onBeforeUnmount } from 'vue'
import { useFocusTrap } from '../../composables/useFocusTrap'

const props = defineProps({
  show: { type: Boolean, default: false },
  title: { type: String, default: '' },
  width: { type: String, default: '440px' },
  placement: { type: String, default: 'right' }, // right | left
  maskClosable: { type: Boolean, default: true },
})

const emit = defineEmits(['update:show', 'close'])

const drawerEl = ref(null)
let prevOverflow = ''

useFocusTrap(drawerEl, {
  get active() { return props.show },
  onEscape: () => close(),
})

watch(() => props.show, (v) => {
  if (v) {
    prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
  } else {
    document.body.style.overflow = prevOverflow
  }
})

onBeforeUnmount(() => {
  if (props.show) document.body.style.overflow = prevOverflow
})

function close() {
  emit('update:show', false)
  emit('close')
}

function onMaskClick() {
  if (props.maskClosable) close()
}
</script>

<style scoped>
.ac-drawer-enter-active,
.ac-drawer-leave-active {
  transition: opacity 0.22s ease;
}
.ac-drawer-enter-active > div:last-child,
.ac-drawer-leave-active > div:last-child {
  transition: transform 0.32s cubic-bezier(0.32, 0.72, 0, 1);
}
.ac-drawer-enter-from,
.ac-drawer-leave-to {
  opacity: 0;
}
.ac-drawer-enter-from > div:last-child,
.ac-drawer-leave-to > div:last-child {
  transform: translateX(100%);
}
</style>
