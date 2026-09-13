import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

// P2 这批动效与音效的护栏。
// 音效那部分用打桩的 AudioContext 真跑一遍，确认"默认静音"和"关掉时不发声"不是嘴上说说。

const read = (path) => readFile(new URL(path, import.meta.url), 'utf8')

// ---- 打桩：localStorage + AudioContext ----
const storage = new Map()
globalThis.localStorage = {
  getItem: (k) => storage.get(k) ?? null,
  setItem: (k, v) => storage.set(k, String(v)),
  removeItem: (k) => storage.delete(k),
}

const oscillators = []
class FakeParam {
  setValueAtTime() { return this }
  exponentialRampToValueAtTime() { return this }
}
class FakeNode {
  connect(target) { this.target = target; return target }
}
class FakeOscillator extends FakeNode {
  constructor() {
    super()
    this.frequency = new FakeParam()
    this.type = 'sine'
    oscillators.push(this)
  }
  start() {}
  stop() {}
}
class FakeGain extends FakeNode {
  constructor() {
    super()
    this.gain = new FakeParam()
  }
}
class FakeAudioContext {
  constructor() {
    this.state = 'running'
    this.currentTime = 0
    this.destination = new FakeNode()
  }
  createOscillator() { return new FakeOscillator() }
  createGain() { return new FakeGain() }
  resume() { this.state = 'running' }
}

globalThis.window = { AudioContext: FakeAudioContext, matchMedia: () => ({ matches: false }) }
globalThis.document = {
  documentElement: { dataset: {}, style: { setProperty() {} } },
  createElement: () => ({}),
  createTextNode: () => ({}),
}

const { useSound, SOUND_NAMES, _resetSoundState } = await import('../src/composables/useSound.js')

// 1. 音效：默认必须是静音，且关着的时候一个振荡器都不能建
_resetSoundState()
storage.clear()
const sound = useSound()
assert.equal(sound.enabled.value, false, '音效默认必须关闭')
assert.equal(SOUND_NAMES.length >= 6, true, '至少要有 open/close/confirm/cancel/tab/done/error 这一档')
for (const name of ['open', 'close', 'confirm', 'cancel', 'tab', 'done', 'error']) {
  assert.ok(SOUND_NAMES.includes(name), `缺少音效 ${name}`)
}
assert.equal(sound.play('open'), false, '关闭状态下不应发声')
assert.equal(oscillators.length, 0, '关闭状态下不能创建任何音频节点')

// 2. 打开后按配方发声：done 是三音上行
sound.enabled.value = true
oscillators.length = 0
assert.equal(sound.play('done'), true)
assert.equal(oscillators.length, 3, '完成音应是三音（动森领取奖励那一下）')
oscillators.length = 0
assert.equal(sound.play('confirm'), true)
assert.equal(oscillators.length, 2, '确认音是两音叠置')
oscillators.length = 0
assert.equal(sound.play('nope'), false, '未知音效不应发声')
assert.equal(oscillators.length, 0)

// 3. 设置持久化 + 音量钳位
sound.volume.value = 5
assert.equal(sound.volume.value, 1, '音量上限 1')
sound.volume.value = -3
assert.equal(sound.volume.value, 0, '音量下限 0')
assert.equal(JSON.parse(storage.get('anidog.sound')).enabled, true, '开关要落盘')
sound.enabled.value = false
assert.equal(JSON.parse(storage.get('anidog.sound')).enabled, false)

const soundSource = await read('../src/composables/useSound.js')
assert.match(soundSource, /enabled: raw\.enabled === true/, '解析配置时只有显式 true 才算开启')
assert.match(soundSource, /if \(!settings\.value\.enabled\) return false/, '播放前必须再查一次开关')
assert.match(soundSource, /AudioContext/, '音频上下文只能惰性创建（浏览器要求用户手势之后）')

// 4. 路由转场三档
const routeTransition = await import('../src/composables/useRouteTransition.js')
assert.deepEqual(routeTransition.ROUTE_TRANSITIONS, ['expand', 'wave', 'fade'])
assert.equal(routeTransition.transitionForRoute({ meta: { transition: 'expand' } }), 'expand')
assert.equal(routeTransition.transitionForRoute({ meta: { transition: 'wave' } }), 'wave')
assert.equal(routeTransition.transitionForRoute({ meta: {} }), 'fade', '未声明时用最安静的渐入')
assert.equal(routeTransition.transitionForRoute({ meta: { transition: 'bogus' } }), 'fade')
assert.equal(routeTransition.routeTransitionName({ meta: { transition: 'expand' } }), 'ac-route-expand')

const router = await read('../src/router/index.js')
assert.match(router, /meta: \{ transition: 'expand' \}/, '详情页应声明扩大式转场')
assert.match(router, /meta: \{ transition: 'wave' \}/, '工作流页应声明刷入式转场')

const layout = await read('../src/views/Layout/NaiveLayout.vue')
assert.match(layout, /:name="routeTransitionName\(route\)"/)
assert.match(layout, /installRouteTransitionOrigin\(\)/, 'expand 转场需要记录点击位置')

// 5. 图章 / 环形菜单 / 结算 / iris
const stamp = await read('../src/components/ac/AcStamp.vue')
assert.match(stamp, /class="ac-stamp"/)

const radial = await read('../src/components/ac/AcRadialMenu.vue')
assert.match(radial, /role="menu"/)
assert.match(radial, /role="menuitem"/)
assert.match(radial, /Math\.cos\(angle\) \* props\.radius/, '菜单项应按角度分布成环')
assert.match(radial, /if \(e\.key === 'Escape'\) close\(\)/, '环形菜单必须能用 Esc 关掉')
assert.match(radial, /item\.disabled/, '不可用的动作要能禁用')
assert.match(radial, /:disabled="item\.disabled"/, '禁用要落到按钮本身，而不只是拦点击')
assert.match(radial, /removeEventListener\('keydown'/, '卸载时要解绑键盘监听')

const settle = await read('../src/components/ac/AcSettleBurst.vue')
assert.match(settle, /emit\('update:show', false\)/, '结算层要自动收走')
assert.match(settle, /--ac-settle-index/, '条目要错峰出现')

const iris = await read('../src/components/ac/AcIrisWipe.vue')
assert.match(iris, /document\.documentElement\.dataset\.reduceMotion === 'true'/, 'iris 要尊重减少动态')
assert.match(iris, /finally \{[\s\S]*phase\.value = 'opening'/, '无论任务成功失败都要收回遮罩')

const css = await read('../src/assets/tailwind.css')
for (const keyframe of ['ac-stamp-in', 'ac-iris-close', 'ac-iris-open', 'ac-radial-pop', 'ac-settle-row']) {
  assert.match(css, new RegExp(`@keyframes ${keyframe}`), `缺少关键帧 ${keyframe}`)
}
// 新动效都要能停
assert.match(css, /@media \(prefers-reduced-motion: reduce\) \{[\s\S]*?\.ac-iris/)

// 转场类必须在 transitions.css（Tailwind 会摇掉 @layer 里运行时才拼出的类名：
// Vue 的 <transition name="x"> 生成的 x-enter-active 在源码里搜不到，样式会整段消失）
const transitions = await read('../src/assets/transitions.css')
assert.match(transitions, /@keyframes ac-route-expand/)
assert.match(transitions, /@keyframes ac-route-wave/)
assert.match(transitions, /@keyframes ac-settle-in/)
for (const cls of [
  '.ac-route-expand-enter-active', '.ac-route-wave-enter-active',
  '.ac-route-fade-enter-active', '.ac-settle-enter-active', '.ac-radial-enter-active',
]) {
  assert.ok(transitions.includes(cls), `transitions.css 缺少 ${cls}`)
}
// 扩大式转场必须以点击位置为圆心
assert.match(transitions, /@keyframes ac-route-expand \{\s*0% \{\s*clip-path: circle\(0% at var\(--ac-route-x, 50%\)/)
// 而且不许再放回 Tailwind 的层里
assert.doesNotMatch(css, /@keyframes ac-route-/, '转场关键帧不能放在 tailwind.css（会被摇掉）')
assert.doesNotMatch(css, /\.ac-route-expand-enter-active \{\s*animation:/, '转场类不能放回 @layer components')
const main = await read('../src/main.js')
assert.match(main, /import '\.\/assets\/transitions\.css'/, 'main.js 必须引入 transitions.css')

// 6. 接线：设置页要有音效开关，下载页要有环形菜单与结算
const settings = await read('../src/views/Settings/index.vue')
assert.match(settings, /useSound\(\)/)
assert.match(settings, /soundTitle/)
assert.match(settings, /AcSwitch v-model="soundEnabled"/)

const downloads = await read('../src/views/Downloads/DownloadList.vue')
assert.match(downloads, /@contextmenu\.prevent="openRadial\(\$event, task\)"/, '右键应能唤出环形菜单')
assert.match(downloads, /<AcRadialMenu/)
assert.match(downloads, /<AcSettleBurst/)
assert.match(downloads, /irisRef\.value\.run\(task\)/, '重操作应走 iris')
assert.match(downloads, /showSettle\(/)

console.log('animations and sound tests passed')
