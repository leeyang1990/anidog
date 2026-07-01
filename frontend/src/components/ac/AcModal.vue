<template>
  <Teleport to="body">
    <transition name="ac-modal">
      <div
        v-if="show"
        class="fixed inset-0 z-[1000] flex items-center justify-center p-4 sm:p-6"
        role="dialog"
        aria-modal="true"
      >
        <div class="absolute inset-0 bg-ac-night/40 backdrop-blur-sm" @click="onMaskClick" />
        <div
          ref="modalEl"
          class="relative bg-card text-card-foreground border-2 border-ac-sand rounded-[32px] shadow-2xl w-full max-h-[90vh] flex flex-col overflow-hidden"
          :style="{ maxWidth: maxWidth }"
        >
          <button
            type="button"
            class="absolute top-3 right-3 z-10 size-9 rounded-full bg-ac-heart text-white flex items-center justify-center font-bold text-lg shadow-md hover:bg-ac-heart-dark transition-colors"
            @click="close"
            aria-label="关闭"
          >×</button>
          <div v-if="title || $slots.header" class="px-6 pt-6 pb-3 border-b-2 border-dashed border-ac-sand">
            <slot name="header">
              <h2 class="text-lg font-bold text-foreground pr-12">{{ title }}</h2>
              <p v-if="description" class="text-sm text-muted-foreground mt-1">{{ description }}</p>
            </slot>
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
import { ref, watch, nextTick, onBeforeUnmount } from 'vue'
import { useFocusTrap } from '../../composables/useFocusTrap'

const props = defineProps({
  show: { type: Boolean, default: false },
  title: { type: String, default: '' },
  description: { type: String, default: '' },
  maxWidth: { type: String, default: '560px' },
  maskClosable: { type: Boolean, default: true },
  escClosable: { type: Boolean, default: true },
})

const emit = defineEmits(['update:show', 'close'])

const modalEl = ref(null)
let prevOverflow = ''

useFocusTrap(modalEl, {
  get active() { return props.show },
  onEscape: () => { if (props.escClosable) close() },
})

watch(() => props.show, async (v) => {
  if (v) {
    prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    await nextTick()
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
.ac-modal-enter-active,
.ac-modal-leave-active {
  transition: opacity 0.22s ease;
}
.ac-modal-enter-active > div:last-child,
.ac-modal-leave-active > div:last-child {
  transition: transform 0.32s cubic-bezier(0.34, 1.56, 0.64, 1), opacity 0.22s ease;
}
.ac-modal-enter-from,
.ac-modal-leave-to {
  opacity: 0;
}
.ac-modal-enter-from > div:last-child,
.ac-modal-leave-to > div:last-child {
  transform: scale(0.94) translateY(8px);
  opacity: 0;
}
</style>
