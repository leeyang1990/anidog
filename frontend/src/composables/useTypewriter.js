// composables/useTypewriter.js
// 简单打字机：返回随时间增长的字符串
import { ref, watchEffect, onBeforeUnmount } from 'vue'

export function useTypewriter(text, { speed = 60, delay = 200 } = {}) {
  const out = ref('')
  let timer = null
  let timeoutId = null

  watchEffect(() => {
    if (timer) clearInterval(timer)
    if (timeoutId) clearTimeout(timeoutId)
    out.value = ''
    const full = typeof text === 'function' ? text() : (text?.value ?? text)
    if (!full) return
    timeoutId = setTimeout(() => {
      let i = 0
      timer = setInterval(() => {
        out.value = full.slice(0, ++i)
        if (i >= full.length) {
          clearInterval(timer)
          timer = null
        }
      }, speed)
    }, delay)
  })

  onBeforeUnmount(() => {
    if (timer) clearInterval(timer)
    if (timeoutId) clearTimeout(timeoutId)
  })

  return out
}
