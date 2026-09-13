import { onBeforeUnmount, ref, watch } from 'vue'

// 数字滚动：数值变化时补一段过渡，而不是瞬间跳变。
//
// 动森里连"数值变化"都是被演出的（资源里有一整套 UI_CountUp / UI_CountDown /
// UI_Count_*_Finish / _Repeat 音效，分"进行中"和"结束"两段）。我们至少把视觉补上。
//
// 关键取舍：
//  - 从"当前显示值"继续滚，而不是每次都从 0 重来（下载速度每秒刷新，从 0 重来会一直闪）
//  - 目标值没变就不重启动画
//  - 减少动态偏好下直接落位，不做过渡

export function easeOutCubic(t) {
  return 1 - (1 - t) ** 3
}

export function interpolate(from, to, progress) {
  return from + (to - from) * progress
}

/** 该不该播过渡：非有限数、相等、或用户要求减少动态时都不播。 */
export function shouldAnimate(from, to, reduceMotion = false) {
  if (reduceMotion) return false
  if (!Number.isFinite(to)) return false
  if (!Number.isFinite(from)) return true
  return from !== to
}

export function prefersReducedMotion() {
  if (typeof document !== 'undefined' && document.documentElement.dataset.reduceMotion === 'true') return true
  if (typeof window === 'undefined' || !window.matchMedia) return false
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * @param {() => number} read 取目标值
 * @param {{ duration?: number, animateOnMount?: boolean }} options
 *   animateOnMount: 首次渲染从 0 滚上来（用于"数据到达后才有数字"的场景，
 *   例如日历每天的部数、看板统计——直接以终值挂载是看不到滚动的）
 * @returns {{ display: import('vue').Ref<number>, finish: () => void }}
 */
export function useCountUp(read, options = {}) {
  const duration = options.duration ?? 600
  const target = () => Number(read())
  const display = ref(options.animateOnMount ? 0 : target())
  let frame = 0

  const stop = () => {
    if (frame) cancelAnimationFrame(frame)
    frame = 0
  }

  const finish = () => {
    stop()
    display.value = target()
  }

  const play = () => {
    const from = display.value
    const to = target()
    if (!shouldAnimate(from, to, prefersReducedMotion())) {
      // 减少动态或值未变：直接落位，不启动 rAF
      if (Number.isFinite(to)) display.value = to
      return
    }
    stop()
    const started = performance.now()
    const step = (now) => {
      const progress = Math.min(1, (now - started) / duration)
      display.value = interpolate(from, to, easeOutCubic(progress))
      if (progress < 1) frame = requestAnimationFrame(step)
      else frame = 0
    }
    frame = requestAnimationFrame(step)
  }

  const stopWatch = watch(target, play, { immediate: true })
  onBeforeUnmount(() => {
    stopWatch()
    stop()
  })

  return { display, finish }
}
