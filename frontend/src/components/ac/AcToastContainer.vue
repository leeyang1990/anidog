<template>
  <Teleport to="body">
    <div class="ac-toast-container fixed top-4 left-1/2 -translate-x-1/2 z-[2000] flex flex-col items-center gap-2 pointer-events-none w-[min(92vw,420px)]">
      <transition-group name="ac-toast">
        <div
          v-for="t in state.list"
          :key="t.id"
          class="ac-toast pointer-events-auto bg-card text-card-foreground border-2 rounded-2xl shadow-lg px-4 py-3 flex items-center gap-3 w-full"
          :class="variantCls(t.type)"
        >
          <span class="shrink-0 w-6 h-6 flex items-center justify-center rounded-full" :class="iconBgCls(t.type)">
            <svg v-if="t.type === 'success'" viewBox="0 0 24 24" class="w-4 h-4 text-white" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><path d="M5 13l4 4L19 7" /></svg>
            <svg v-else-if="t.type === 'error'" viewBox="0 0 24 24" class="w-4 h-4 text-white" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><path d="M6 6l12 12M18 6l-12 12" /></svg>
            <svg v-else-if="t.type === 'warning'" viewBox="0 0 24 24" class="w-4 h-4 text-white" fill="currentColor"><path d="M12 2 L22 20 H2 Z" /><circle cx="12" cy="16.5" r="1.2" fill="white" /><rect x="11" y="9" width="2" height="5" fill="white" /></svg>
            <svg v-else-if="t.type === 'info'" viewBox="0 0 24 24" class="w-4 h-4 text-white" fill="currentColor"><circle cx="12" cy="12" r="10" /><rect x="11" y="10" width="2" height="7" fill="white" /><circle cx="12" cy="7.5" r="1.2" fill="white" /></svg>
            <AcSpinner v-else-if="t.type === 'loading'" :size="16" color="#7CB342" />
          </span>
          <span class="text-sm flex-1 leading-snug break-words">{{ t.message }}</span>
          <button
            v-if="t.closable"
            type="button"
            class="shrink-0 size-5 rounded-full text-xs flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-ac-sand transition-colors"
            @click="state.list.splice(state.list.findIndex(x => x.id === t.id), 1)"
          >×</button>
        </div>
      </transition-group>
    </div>
  </Teleport>
</template>

<script setup>
import { _useToastState } from '../../composables/useToast'
import AcSpinner from './AcSpinner.vue'

const state = _useToastState()

function variantCls(type) {
  return {
    success: 'border-ac-leaf/60',
    error: 'border-ac-heart/60',
    warning: 'border-ac-sun',
    info: 'border-ac-sky/70',
    loading: 'border-ac-grass/60',
  }[type] || 'border-ac-sand'
}

function iconBgCls(type) {
  return {
    success: 'bg-ac-leaf',
    error: 'bg-ac-heart',
    warning: 'bg-ac-sun',
    info: 'bg-ac-sky-dark',
    loading: 'bg-transparent',
  }[type] || 'bg-ac-sand'
}
</script>

<style scoped>
.ac-toast-enter-active,
.ac-toast-leave-active {
  transition: all 0.28s cubic-bezier(0.34, 1.56, 0.64, 1);
}
.ac-toast-enter-from {
  opacity: 0;
  transform: translateY(-12px) scale(0.96);
}
.ac-toast-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}
</style>
