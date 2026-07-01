// composables/useFocusTrap.js
// 把 Tab/Shift+Tab 限制在容器内 + esc 调用 onEscape
import { onMounted, onBeforeUnmount, nextTick } from 'vue'

const FOCUSABLE = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

export function useFocusTrap(elRef, { active = true, onEscape } = {}) {
  let prevActive = null

  function getFocusable() {
    const el = typeof elRef === 'function' ? elRef() : elRef.value
    if (!el) return []
    return Array.from(el.querySelectorAll(FOCUSABLE)).filter(n => !n.hasAttribute('aria-hidden'))
  }

  function onKeydown(e) {
    if (!active) return
    if (e.key === 'Escape' && typeof onEscape === 'function') {
      onEscape(e)
      return
    }
    if (e.key !== 'Tab') return
    const list = getFocusable()
    if (!list.length) return
    const first = list[0]
    const last = list[list.length - 1]
    if (e.shiftKey) {
      if (document.activeElement === first || !getRoot()?.contains(document.activeElement)) {
        e.preventDefault()
        last.focus()
      }
    } else {
      if (document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }
  }

  function getRoot() {
    return typeof elRef === 'function' ? elRef() : elRef.value
  }

  onMounted(async () => {
    prevActive = document.activeElement
    await nextTick()
    const list = getFocusable()
    if (list.length) list[0].focus()
    document.addEventListener('keydown', onKeydown)
  })

  onBeforeUnmount(() => {
    document.removeEventListener('keydown', onKeydown)
    if (prevActive && typeof prevActive.focus === 'function') {
      try { prevActive.focus() } catch (_) {}
    }
  })
}
