import { ref } from 'vue'

// 路由转场的三种模式（抄自动森"按有没有图形出发点来选转场"那套逻辑）：
//   expand —— 从点击位置扩满屏幕：详情页这种"从某张卡片长出来"的场景
//   wave   —— 从底部升起的弧形遮罩：进入一条工作流（RSS、规则、番剧库）
//   fade   —— 最朴素的渐入：设置、通知这类由硬件/工具栏触发的页面，越安静越好
//
// 点击位置记在模块级：卡片点击 → 路由变化之间没有别的通道，
// 而转场 CSS 只认 document 上的自定义属性。

export const ROUTE_TRANSITIONS = ['expand', 'wave', 'fade']

const origin = ref({ x: 0.5, y: 0.5 }) // 默认屏幕中心
let bound = false

function record(event) {
  if (typeof window === 'undefined') return
  const point = event.touches?.[0] || event.changedTouches?.[0] || event
  const x = Number.isFinite(point.clientX) ? point.clientX : window.innerWidth / 2
  const y = Number.isFinite(point.clientY) ? point.clientY : window.innerHeight / 2
  origin.value = { x, y }
  const root = document.documentElement
  root.style.setProperty('--ac-route-x', `${Math.round(x)}px`)
  root.style.setProperty('--ac-route-y', `${Math.round(y)}px`)
}

/** 在应用根部调用一次：记录最近一次指针位置，供 expand 转场当圆心。 */
export function installRouteTransitionOrigin() {
  if (bound || typeof window === 'undefined') return () => {}
  bound = true
  window.addEventListener('pointerdown', record, { capture: true, passive: true })
  window.addEventListener('touchstart', record, { capture: true, passive: true })
  return () => {
    bound = false
    window.removeEventListener('pointerdown', record, { capture: true })
    window.removeEventListener('touchstart', record, { capture: true })
  }
}

/** 取路由的转场名；meta.transition 优先，未声明则 fade。 */
export function transitionForRoute(route) {
  const name = route?.meta?.transition
  return ROUTE_TRANSITIONS.includes(name) ? name : 'fade'
}

export function routeTransitionName(route) {
  return `ac-route-${transitionForRoute(route)}`
}

export function currentOrigin() {
  return origin.value
}
