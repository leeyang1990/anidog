import { computed, ref } from 'vue'

// 界面音效：全部用 Web Audio 现场合成，不引入任何音频文件。
//
// 抄自动森的不是音色，而是"给哪些行为配了音、同语义怎么分档"：
// 打开分大小、确认与取消分开、破坏性操作用更低的音、完成音是上行三音。
//
// 三条硬约束：
//  1. 默认静音。NAS 常驻场景多半在深夜，没人希望打开网页先响一声。
//  2. AudioContext 必须在用户手势之后创建，否则会被浏览器挂起。
//  3. 用户关掉时不能留下任何运行中的节点。

const STORAGE_KEY = 'anidog.sound'

const settings = ref(readSettings())

function readSettings() {
  if (typeof localStorage === 'undefined') return { enabled: false, volume: 0.5 }
  try {
    const raw = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}')
    return {
      enabled: raw.enabled === true,
      volume: typeof raw.volume === 'number' ? Math.min(1, Math.max(0, raw.volume)) : 0.5,
    }
  } catch {
    return { enabled: false, volume: 0.5 }
  }
}

function persist() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings.value))
  } catch {
    /* 隐私模式下写不了就算了，不影响使用 */
  }
}

let context = null

function ensureContext() {
  if (typeof window === 'undefined') return null
  const Ctor = window.AudioContext || window.webkitAudioContext
  if (!Ctor) return null
  if (!context) context = new Ctor()
  if (context.state === 'suspended') void context.resume()
  return context
}

/**
 * 一次"哔"：振荡器 + 指数衰减包络。
 * @param {{type?: OscillatorType, from: number, to?: number, duration: number, gain?: number, delay?: number}} spec
 */
function blip(spec) {
  const ctx = ensureContext()
  if (!ctx) return
  const start = ctx.currentTime + (spec.delay || 0)
  const end = start + spec.duration / 1000
  const osc = ctx.createOscillator()
  const amp = ctx.createGain()
  osc.type = spec.type || 'sine'
  osc.frequency.setValueAtTime(spec.from, start)
  if (spec.to && spec.to !== spec.from) {
    osc.frequency.exponentialRampToValueAtTime(spec.to, end)
  }
  const peak = (spec.gain ?? 0.06) * settings.value.volume
  amp.gain.setValueAtTime(0.0001, start)
  amp.gain.exponentialRampToValueAtTime(Math.max(peak, 0.0002), start + 0.012)
  amp.gain.exponentialRampToValueAtTime(0.0001, end)
  osc.connect(amp).connect(ctx.destination)
  osc.start(start)
  osc.stop(end + 0.02)
}

// 每个音都是"短促、圆润、不刺耳"：正弦为主，音高滑动，包络快起快落。
const RECIPES = {
  // 打开面板：上行，像气泡冒出来
  open: [{ from: 520, to: 780, duration: 110 }],
  // 关闭：下行，比起始音更短更闷
  close: [{ from: 560, to: 360, duration: 90, gain: 0.05 }],
  // 确认：两个音叠一下，有分量
  confirm: [
    { from: 660, duration: 120 },
    { from: 990, duration: 130, delay: 45, gain: 0.045 },
  ],
  // 取消：独立音色，不是确认的反向播放
  cancel: [{ type: 'triangle', from: 320, to: 210, duration: 100, gain: 0.05 }],
  // 切换：极短，只做"到位"的提示
  tab: [{ from: 880, duration: 45, gain: 0.035 }],
  // 完成：上行三音（动森领取奖励那一下）
  done: [
    { from: 660, duration: 110 },
    { from: 880, duration: 110, delay: 90 },
    { from: 1320, duration: 190, delay: 180, gain: 0.05 },
  ],
  // 失败：低沉、短促，不尖叫
  error: [{ type: 'triangle', from: 240, to: 150, duration: 180, gain: 0.06 }],
}

export const SOUND_NAMES = Object.keys(RECIPES)

export function useSound() {
  const enabled = computed({
    get: () => settings.value.enabled,
    set: (value) => {
      settings.value = { ...settings.value, enabled: Boolean(value) }
      persist()
      if (value) play('confirm') // 打开的瞬间给个确认，让用户知道音量是否合适
    },
  })

  const volume = computed({
    get: () => settings.value.volume,
    set: (value) => {
      settings.value = { ...settings.value, volume: Math.min(1, Math.max(0, Number(value) || 0)) }
      persist()
    },
  })

  function play(name) {
    if (!settings.value.enabled) return false
    const recipe = RECIPES[name]
    if (!recipe) return false
    for (const spec of recipe) blip(spec)
    return true
  }

  return { play, enabled, volume, names: SOUND_NAMES }
}

/** 仅供测试重置内部状态。 */
export function _resetSoundState() {
  settings.value = { enabled: false, volume: 0.5 }
  context = null
}
