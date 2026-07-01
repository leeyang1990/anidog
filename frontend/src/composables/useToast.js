// composables/useToast.js
// 单例 toast：调用 useToast() 总是返回同一个 store；AcToastContainer 渲染列表
import { reactive } from 'vue'

const state = reactive({
  list: [],
  seq: 0,
})

function add(type, message, opts = {}) {
  const id = ++state.seq
  const item = {
    id,
    type,
    message,
    duration: opts.duration ?? (type === 'loading' ? 0 : 3000),
    closable: opts.closable ?? (type !== 'loading'),
  }
  state.list.push(item)
  if (item.duration > 0) {
    setTimeout(() => remove(id), item.duration)
  }
  return {
    id,
    close: () => remove(id),
    update: (msg) => {
      const t = state.list.find(t => t.id === id)
      if (t) t.message = msg
    },
  }
}

function remove(id) {
  const i = state.list.findIndex(t => t.id === id)
  if (i >= 0) state.list.splice(i, 1)
}

const api = {
  success: (msg, o) => add('success', msg, o),
  error:   (msg, o) => add('error',   msg, o),
  warning: (msg, o) => add('warning', msg, o),
  info:    (msg, o) => add('info',    msg, o),
  loading: (msg, o) => add('loading', msg, o),
  remove,
}

export function useToast() {
  return api
}

// 给容器组件用
export function _useToastState() {
  return state
}

// 命名兼容：替代 useMessage()
export function useMessage() {
  return api
}
