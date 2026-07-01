// composables/useClickOutside.js
// 监听点击元素外部时调用 handler
import { onMounted, onBeforeUnmount } from 'vue'

export function useClickOutside(elRef, handler) {
  function listener(e) {
    const el = typeof elRef === 'function' ? elRef() : elRef.value
    if (!el) return
    if (el === e.target || el.contains(e.target)) return
    handler(e)
  }
  onMounted(() => {
    document.addEventListener('mousedown', listener, true)
    document.addEventListener('touchstart', listener, true)
  })
  onBeforeUnmount(() => {
    document.removeEventListener('mousedown', listener, true)
    document.removeEventListener('touchstart', listener, true)
  })
}
