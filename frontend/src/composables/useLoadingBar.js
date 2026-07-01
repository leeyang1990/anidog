// composables/useLoadingBar.js
// 顶部进度条单例
import { reactive } from 'vue'

const state = reactive({
  active: false,
  progress: 0, // 0..100
  error: false,
})

let timer = null

function start() {
  state.error = false
  state.active = true
  state.progress = 8
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    if (state.progress < 88) state.progress += (90 - state.progress) * 0.08
  }, 200)
}

function finish() {
  if (timer) { clearInterval(timer); timer = null }
  state.progress = 100
  setTimeout(() => {
    state.active = false
    state.progress = 0
  }, 320)
}

function error() {
  state.error = true
  if (timer) { clearInterval(timer); timer = null }
  state.progress = 100
  setTimeout(() => {
    state.active = false
    state.progress = 0
    state.error = false
  }, 600)
}

export function useLoadingBar() {
  return { start, finish, error }
}

export function _useLoadingBarState() {
  return state
}
