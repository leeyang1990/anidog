// useSkin —— 主题皮肤切换：'ac-grove'（动森）/ 'classic'（常规）
//
// 写到 <html data-skin="...">，由 tailwind.css 的属性选择器整体覆盖 HSL token。
// 与"亮/暗"无关：暗色模式仍由 .dark class 管，可与任意皮肤组合。
//
// 状态：localStorage('skin')，初次加载 = ac-grove。
//
// 用法：
//   import { useSkin } from '@/composables/useSkin'
//   const { skin, setSkin, SKINS } = useSkin()

import { ref, watch } from 'vue'

const STORAGE_KEY = 'skin'
const DEFAULT_SKIN = 'ac-grove'

export const SKINS = [
  { value: 'ac-grove', label: '🌿 动森（默认）', description: '米白沙地 + 草绿 + 暖橙，圆润饱满' },
  { value: 'classic', label: '🌆 常规', description: '中性灰 + 蓝紫，商务克制' },
]

// 模块级单例 —— 全 app 共享同一个 ref，任何组件读到的都是同一个值
const skin = ref(readStored())

function readStored() {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v && SKINS.some(s => s.value === v)) return v
  } catch {}
  return DEFAULT_SKIN
}

function applySkin(value) {
  const root = document.documentElement
  if (value === DEFAULT_SKIN) {
    // 默认皮肤可以不设属性（也写一下保险）
    root.setAttribute('data-skin', value)
  } else {
    root.setAttribute('data-skin', value)
  }
}

// 启动即应用
if (typeof window !== 'undefined') {
  applySkin(skin.value)
  watch(skin, (v) => {
    applySkin(v)
    try { localStorage.setItem(STORAGE_KEY, v) } catch {}
  })
}

export function useSkin() {
  function setSkin(v) {
    if (!SKINS.some(s => s.value === v)) return
    skin.value = v
  }
  return { skin, setSkin, SKINS }
}
