import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { easeOutCubic, interpolate, shouldAnimate } from '../src/composables/useCountUp.js'

// P0 交互打磨的护栏：数字滚动、确认放射反馈、按下手感。
// 这些都是"看起来只是好看"的东西，最容易在后续改动里被悄悄删掉，所以钉死。

const read = (path) => readFile(new URL(path, import.meta.url), 'utf8')

// 1. 数字滚动：缓动与"该不该播"
assert.equal(easeOutCubic(0), 0)
assert.equal(easeOutCubic(1), 1)
assert.ok(Math.abs(easeOutCubic(0.5) - 0.875) < 1e-9, 'easeOutCubic 中段应快于线性')
assert.ok(easeOutCubic(0.25) > 0.25 && easeOutCubic(0.75) > 0.75, '缓动应始终不低于线性')
assert.equal(interpolate(100, 200, 0.5), 150)
assert.equal(interpolate(0, 0, 1), 0)

assert.equal(shouldAnimate(1, 1), false, '数值没变不应重启动画')
assert.equal(shouldAnimate(1, NaN), false, '目标值非法时应直接落位')
assert.equal(shouldAnimate(0, 42), true)
assert.equal(shouldAnimate(0, 42, true), false, '减少动态偏好下不应播过渡')

const countUp = await read('../src/composables/useCountUp.js')
// 减少动态与"值没变"必须在启动 rAF 之前拦掉
assert.match(countUp, /if \(!shouldAnimate\(from, to, prefersReducedMotion\(\)\)\)/)
assert.match(countUp, /document\.documentElement\.dataset\.reduceMotion === 'true'/)
assert.match(countUp, /cancelAnimationFrame/, '必须在卸载时取消 rAF')
// 从"当前显示值"继续滚，而不是每次从 0 重来（下载速度每秒刷新）
assert.match(countUp, /const from = display\.value/)

const countUpComponent = await read('../src/components/ac/AcCountUp.vue')
assert.match(countUpComponent, /props\.format\(shown\)/, '应支持自定义格式化（速度/体积）')
assert.match(countUpComponent, /\{\{ text \}\}/)
assert.match(countUpComponent, /useCountUp/)

// 2. 确认放射反馈
const burst = await read('../src/components/ac/AcBurst.vue')
assert.match(burst, /prefersReducedMotion\(\)\) return false/, '减少动态下不应播放射反馈')
assert.match(burst, /defineExpose\(\{ play \}\)/, 'AcBurst 必须暴露 play 供父组件在确认时调用')
assert.match(burst, /class="ac-burst"/)

const css = await read('../src/assets/tailwind.css')
assert.match(css, /\.ac-burst \{/)
assert.match(css, /@keyframes ac-burst-spark/)
assert.match(css, /rotate\(var\(--ac-burst-angle\)\)/, '放疗线按角度分布，不能靠 8 个硬编码 transform')
// 放疗线是纯装饰，不能拦截点击
assert.match(css, /\.ac-burst \{[\s\S]*?pointer-events: none;/)

const button = await read('../src/components/ac/AcButton.vue')
assert.match(button, /burst: \{ type: Boolean, default: false \}/)
assert.match(button, /burstRef\.value\?\.play\(\)/)
// 按下手感：位移 + 阴影收缩
assert.match(button, /active:translate-y-\[3px\] active:shadow-none/)
assert.match(button, /\.ac-btn:active:not\(:disabled\) \{\s*box-shadow: none !important;/)

// 3. 接线：数字滚动要真的用在高频变化的数字上
const dashboard = await read('../src/views/Dashboard.vue')
assert.match(dashboard, /<AcCountUp :value="stat\.value" from-zero \/>/, 'Dashboard 统计数字应从 0 滚上来')
const downloads = await read('../src/views/Downloads/DownloadList.vue')
assert.match(downloads, /<AcCountUp :value="globalDownloadSpeed" :format="formatSpeed" \/>/)
assert.match(downloads, /<AcCountUp :value="globalUploadSpeed" :format="formatSpeed" \/>/)
assert.match(downloads, /<AcButton variant="primary" size="sm" burst/, '添加下载应带确认反馈')
const calendar = await read('../src/views/Calendar/index.vue')
assert.match(calendar, /<AcCountUp :value="day\.items\.length"[^>]*from-zero/, '日历星期条的数量应从 0 滚上来')
assert.match(calendar, /scope="global"/, 'i18n-t 必须显式走 global 作用域')

// 4. 追番按钮自己带放射反馈（它是原生 button，不走 AcButton）
const animeCard = await read('../src/views/Anime/AnimeCard.vue')
assert.match(animeCard, /<AcBurst ref="burstRef" :spokes="8" \/>/)
assert.match(animeCard, /function onSubscribe\(item\) \{\s*const played = burstRef\.value\?\.play\(\)/)
// 演出要先于状态翻转：订阅成功后按钮会被移除，不能立刻抛事件
assert.match(animeCard, /setTimeout\(\(\) => emit\('subscribe', item\), SUBSCRIBE_BEAT\)/)
assert.match(animeCard, /const SUBSCRIBE_BEAT = 220/)

// 5. 气泡式弹窗入场
const modal = await read('../src/components/ac/AcModal.vue')
assert.match(modal, /animation: ac-bubble-in 420ms cubic-bezier\(0\.34, 1\.56, 0\.64, 1\)/, '弹窗入场要弹性')
assert.match(modal, /\.ac-modal-enter-active > \.ui-mask::after/, '炸开光晕放在蒙层上，否则会被面板的 overflow 裁掉')
assert.match(modal, /@media \(prefers-reduced-motion: reduce\)/, '必须有减少动态兜底')
// 常驻的"呼吸形变"是刻意不做的：整块弹窗持续缩放会让文字发虚且一直合成
assert.doesNotMatch(modal, /ac-bubble-breathe/)
assert.doesNotMatch(await read('../src/assets/tailwind.css'), /ac-bubble-breathe/)

const drawer = await read('../src/components/ac/AcDrawer.vue')
assert.match(drawer, /animation: ac-drawer-in 380ms/)
assert.match(drawer, /ac-drawer-enter-active > \.ui-mask::after/)

const motionCSS = await read('../src/assets/tailwind.css')
assert.match(motionCSS, /@keyframes ac-bubble-in/)
assert.match(motionCSS, /transform: scale\(1\.018\) translateY\(-3px\)/, '要有过冲帧，否则只是普通放大')
assert.match(motionCSS, /@keyframes ac-bubble-burst/)
assert.match(motionCSS, /@keyframes ac-drawer-in/)

// 6. 场景化等待（多源并发探测）
const loader = await read('../src/components/ac/AcSceneLoader.vue')
assert.match(loader, /role="status"/, '等待态要让读屏软件能感知')
assert.match(loader, /:aria-label="label"/)
assert.match(loader, /'--ac-scan-index': index/)
// 后端是一次性聚合返回，没有逐站进度，所以这里不能装成确定性进度条
assert.doesNotMatch(loader, /role="progressbar"|aria-valuenow|%/)
// 只看模板：注释里解释"不显示已完成几个"是允许的，界面里不能真出现
const loaderTemplate = loader.slice(loader.indexOf('<template>'), loader.indexOf('</template>'))
assert.doesNotMatch(loaderTemplate, /已完成|completed/)

const scanCSS = await read('../src/assets/tailwind.css')
assert.match(scanCSS, /@keyframes ac-scan-sweep/)
assert.match(scanCSS, /@keyframes ac-scan-light/)
assert.match(scanCSS, /\.ac-scene-loader__chip \{[\s\S]*?animation-delay: calc\(var\(--ac-scan-index, 0\) \* 200ms\)/)
assert.match(scanCSS, /@media \(prefers-reduced-motion: reduce\) \{\s*\.ac-scene-loader__pulse,[\s\S]*?animation: none;/)

const search = await read('../src/views/Search/index.vue')
assert.match(search, /<AcSceneLoader/)
assert.match(search, /selectedIndexerLabels/, '要把"正在问哪几个站"显示出来')
assert.doesNotMatch(search, /AcSpinner :size="48"/, '搜索等待不该再用干转圈')

console.log('interaction polish tests passed')
