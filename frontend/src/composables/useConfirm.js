// composables/useConfirm.js
// 单例 confirm：调用返回 Promise<boolean>
import { reactive } from 'vue'

const state = reactive({
  show: false,
  payload: null,
  resolve: null,
})

export function useConfirm() {
  function confirm({
    title = '确认',
    content = '',
    confirmText = '确定',
    cancelText = '取消',
    variant = 'primary', // primary | danger
    icon,
  } = {}) {
    return new Promise((resolve) => {
      state.payload = { title, content, confirmText, cancelText, variant, icon }
      state.show = true
      state.resolve = resolve
    })
  }
  return { confirm }
}

export function _useConfirmState() {
  return state
}

// 兼容老代码：useDialog().warning({...}) 风格
export function useDialog() {
  const { confirm } = useConfirm()
  const wrap = (variant) => (opts = {}) => confirm({
    title: opts.title,
    content: opts.content,
    confirmText: opts.positiveText || '确定',
    cancelText: opts.negativeText || '取消',
    variant,
  }).then(ok => {
    if (ok && typeof opts.onPositiveClick === 'function') opts.onPositiveClick()
    if (!ok && typeof opts.onNegativeClick === 'function') opts.onNegativeClick()
    return ok
  })
  return {
    warning: wrap('danger'),
    error: wrap('danger'),
    info: wrap('primary'),
    success: wrap('primary'),
  }
}
