<template>
  <Teleport to="body">
    <transition name="ac-drawer">
      <div v-if="show" class="fixed inset-0 z-[1000] flex" :class="placement === 'right' ? 'justify-end' : 'justify-start'">
        <div class="ui-mask absolute inset-0 bg-ac-night/40 backdrop-blur-sm" @click="onMaskClick" />
        <div
          ref="drawerEl"
          class="ui-overlay relative bg-card text-card-foreground border-2 border-ac-sand h-full flex flex-col overflow-hidden"
          :class="placement === 'right' ? 'rounded-l-[32px] border-r-0' : 'rounded-r-[32px] border-l-0'"
          :style="{ width: width, maxWidth: '92vw' }"
        >
          <div v-if="title || $slots.header" class="ui-divider-bottom px-6 py-4 border-b-2 border-dashed border-ac-sand flex items-center justify-between gap-3">
            <slot name="header">
              <h2 class="text-base font-bold text-foreground truncate">{{ title }}</h2>
            </slot>
            <button
              type="button"
              class="shrink-0 size-9 rounded-full bg-ac-heart text-white flex items-center justify-center font-bold text-lg shadow-md hover:bg-ac-heart-dark transition-colors"
              @click="close"
              :aria-label="t('common.close')"
            >×</button>
          </div>
          <div class="flex-1 overflow-y-auto px-6 py-5">
            <slot />
          </div>
          <div v-if="$slots.footer" class="ui-divider px-6 py-4 border-t-2 border-dashed border-ac-sand bg-ac-cream/30">
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
import { useI18n } from 'vue-i18n'
import { useSound } from '@/composables/useSound'

const props = defineProps({
  show: { type: Boolean, default: false },
  title: { type: String, default: '' },
  width: { type: String, default: '440px' },
  placement: { type: String, default: 'right' }, // right | left
  maskClosable: { type: Boolean, default: true },
})

const emit = defineEmits(['update:show', 'close'])
const { t } = useI18n({ useScope: 'global' })
const { play } = useSound()

const drawerEl = ref(null)
let prevOverflow = ''

useFocusTrap(drawerEl, {
  get active() { return props.show },
  onEscape: () => close(),
})

watch(() => props.show, (v) => {
  play(v ? 'open' : 'close')
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
/* 抽屉沿用滑入，收尾带一点回弹；并复用弹窗那圈"气泡炸开"的光晕，保持全家桶一致。 */
.ac-drawer-enter-active,
.ac-drawer-leave-active {
  transition: opacity 0.22s ease;
}
.ac-drawer-enter-active > div:last-child {
  animation: ac-drawer-in 380ms cubic-bezier(0.22, 1, 0.36, 1);
}
.ac-drawer-leave-active > div:last-child {
  transition: transform 0.26s cubic-bezier(0.4, 0, 1, 1);
}
.ac-drawer-enter-from,
.ac-drawer-leave-to {
  opacity: 0;
}
.ac-drawer-enter-from > div:last-child,
.ac-drawer-leave-to > div:last-child {
  transform: translateX(100%);
}
.ac-drawer-enter-active > .ui-mask::after {
  content: '';
  position: absolute;
  left: 50%;
  top: 50%;
  width: 120px;
  height: 120px;
  margin: -60px 0 0 -60px;
  border-radius: 9999px;
  background: radial-gradient(circle, hsl(var(--card) / 0.6) 0%, transparent 70%);
  animation: ac-bubble-burst 420ms cubic-bezier(0.22, 1, 0.36, 1) forwards;
}

@media (prefers-reduced-motion: reduce) {
  .ac-drawer-enter-active > div:last-child,
  .ac-drawer-enter-active > .ui-mask::after {
    animation: none;
  }
}
</style>
