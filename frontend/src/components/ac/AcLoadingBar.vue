<template>
  <div
    v-show="state.active"
    class="ac-loading-bar fixed top-0 left-0 z-[3000] h-1 transition-all duration-300 ease-out"
    :class="state.error ? 'bg-ac-heart' : 'bg-ac-grass'"
    :style="{ width: state.progress + '%' }"
  />
</template>

<script setup>
import { _useLoadingBarState } from '../../composables/useLoadingBar'
const state = _useLoadingBarState()
</script>

<style scoped>
.ac-loading-bar {
  box-shadow: 0 0 8px rgba(124, 179, 66, 0.5);
}
</style>
