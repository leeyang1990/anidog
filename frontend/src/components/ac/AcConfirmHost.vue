<template>
  <AcModal
    :show="state.show"
    :title="state.payload?.title || '确认'"
    :max-width="'440px'"
    @update:show="onClose"
  >
    <p class="text-sm text-muted-foreground leading-relaxed whitespace-pre-line">{{ state.payload?.content }}</p>
    <template #footer>
      <div class="flex items-center justify-end gap-2">
        <AcButton variant="ghost" @click="resolveAndClose(false)">{{ state.payload?.cancelText || '取消' }}</AcButton>
        <AcButton :variant="state.payload?.variant === 'danger' ? 'danger' : 'primary'" @click="resolveAndClose(true)">{{ state.payload?.confirmText || '确定' }}</AcButton>
      </div>
    </template>
  </AcModal>
</template>

<script setup>
import AcModal from './AcModal.vue'
import AcButton from './AcButton.vue'
import { _useConfirmState } from '../../composables/useConfirm'

const state = _useConfirmState()

function resolveAndClose(ok) {
  if (typeof state.resolve === 'function') state.resolve(ok)
  state.resolve = null
  state.show = false
}

function onClose(v) {
  if (!v) resolveAndClose(false)
}
</script>
