<template>
  <Teleport to="body">
    <transition name="ac-modal">
      <div
        v-if="show"
        class="fixed inset-0 z-[1000] flex items-center justify-center p-4 sm:p-6"
        role="dialog"
        aria-modal="true"
      >
        <div class="ui-mask absolute inset-0 bg-ac-night/40 backdrop-blur-sm" @click="onMaskClick" />
        <div
          ref="modalEl"
          class="ui-overlay relative bg-card text-card-foreground border-2 border-ac-sand rounded-[32px] shadow-2xl w-full max-h-[90vh] flex flex-col overflow-hidden"
          :style="{ maxWidth: maxWidth }"
        >
          <button
            type="button"
            class="absolute top-3 right-3 z-10 size-9 rounded-full bg-ac-heart text-white flex items-center justify-center font-bold text-lg shadow-md hover:bg-ac-heart-dark transition-colors"
            @click="close"
            :aria-label="t('common.close')"
          >×</button>
          <div v-if="title || $slots.header" class="ui-divider-bottom px-6 pt-6 pb-3 border-b-2 border-dashed border-ac-sand">
            <slot name="header">
              <h2 class="text-lg font-bold text-foreground pr-12">{{ title }}</h2>
              <p v-if="description" class="text-sm text-muted-foreground mt-1">{{ description }}</p>
            </slot>
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
import { ref, watch, nextTick, onBeforeUnmount } from 'vue'
import { useFocusTrap } from '../../composables/useFocusTrap'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  show: { type: Boolean, default: false },
  title: { type: String, default: '' },
  description: { type: String, default: '' },
  maxWidth: { type: String, default: '560px' },
  maskClosable: { type: Boolean, default: true },
  escClosable: { type: Boolean, default: true },
})

const emit = defineEmits(['update:show', 'close'])
const { t } = useI18n({ useScope: 'global' })

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
/* 气泡式入场：小 → 胀大 → 回弹 → 轻微横向晃动后安定。
 * 动森的弹窗静止时还会持续形变，但网页里整块弹窗持续缩放着会让文字发虚、还要一直合成，
 * 所以只做一次有始有终的演出，不保留常驻形变。 */
.ac-modal-enter-active,
.ac-modal-leave-active {
  transition: opacity 0.22s ease;
}
.ac-modal-enter-active > div:last-child {
  animation: ac-bubble-in 420ms cubic-bezier(0.34, 1.56, 0.64, 1);
}
.ac-modal-leave-active > div:last-child {
  transition: transform 0.24s cubic-bezier(0.4, 0, 1, 1), opacity 0.2s ease;
}
.ac-modal-enter-from,
.ac-modal-leave-to {
  opacity: 0;
}
.ac-modal-enter-from > div:last-child {
  transform: scale(0.82) translateY(14px);
  opacity: 0;
}
.ac-modal-leave-to > div:last-child {
  transform: scale(0.96) translateY(6px);
  opacity: 0;
}

/* 气泡炸开：从面板中心向外扩一圈光晕（放在蒙层上，不会被面板的 overflow 裁掉） */
.ac-modal-enter-active > .ui-mask::after {
  content: '';
  position: absolute;
  left: 50%;
  top: 50%;
  width: 140px;
  height: 140px;
  margin: -70px 0 0 -70px;
  border-radius: 9999px;
  background: radial-gradient(circle, hsl(var(--card) / 0.75) 0%, transparent 70%);
  animation: ac-bubble-burst 460ms cubic-bezier(0.22, 1, 0.36, 1) forwards;
}

@media (prefers-reduced-motion: reduce) {
  .ac-modal-enter-active > div:last-child,
  .ac-modal-enter-active > .ui-mask::after {
    animation: none;
  }
}
</style>
