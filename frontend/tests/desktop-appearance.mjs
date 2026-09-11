import assert from 'node:assert/strict'
import { readFile, readdir } from 'node:fs/promises'
import { resolveAppearance } from '../src/utils/appearancePolicy.js'
import { installDesktopAppearance } from '../src/utils/desktopAppearance.js'

// Architectural guard: native styling belongs at the boundary, never in pages/components.
async function checkSharedUI(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = new URL(entry.name + (entry.isDirectory() ? '/' : ''), directory)
    if (entry.isDirectory()) await checkSharedUI(path)
    else if (entry.name.endsWith('.vue')) {
      const source = await readFile(path, 'utf8')
      assert.doesNotMatch(source, /data-(?:platform|window-material|mac-material)|ApplyAppearance|resolveAppearance/, path.pathname)
    }
  }
}
await checkSharedUI(new URL('../src/components/', import.meta.url))
await checkSharedUI(new URL('../src/views/', import.meta.url))
const sharedCSS = await readFile(new URL('../src/assets/materials.css', import.meta.url), 'utf8')
assert.doesNotMatch(sharedCSS, /data-platform|data-window-material|darwin|windows|linux/)
const nativeCSS = await readFile(new URL('../src/assets/nativeWindow.css', import.meta.url), 'utf8')
assert.doesNotMatch(nativeCSS, /\.ui-(?:panel|overlay|popover|control|table)|\.ac-/)
assert.doesNotMatch(nativeCSS, /font-family|letter-spacing/, 'Typography belongs to the skin, not the platform')

for (const platform of ['darwin', 'windows', 'linux', 'web']) {
  for (const supportsBlur of [true, false]) {
    const result = resolveAppearance({ native: { platform, mode: platform === 'darwin' ? 'liquid' : 'none' }, supportsBlur })
    assert.equal(result.platform, platform)
    assert.equal(result.uiMaterial, supportsBlur ? 'frosted' : 'solid')
    assert.equal(result.windowMaterial, platform === 'darwin' ? 'liquid' : 'none')
  }
}
assert.equal(resolveAppearance({ native: { platform: 'windows', mode: 'liquid' }, supportsBlur: true }).windowMaterial, 'none')
for (const key of ['reduceTransparency', 'highContrast']) {
  assert.equal(resolveAppearance({ supportsBlur: true, [key]: true }).uiMaterial, 'solid')
}
assert.equal(resolveAppearance({ supportsBlur: true, native: { platform: 'darwin', mode: 'solid' } }).uiMaterial, 'solid')
assert.equal(resolveAppearance({ supportsBlur: true, native: { reduce_transparency: true } }).uiMaterial, 'solid')

let dark = false
let observer
let disconnected
let listeners
let queries
function setup(bridge, supports = true) {
  listeners = new Map()
  queries = new Map()
  disconnected = false
  globalThis.document = { documentElement: { dataset: {}, classList: { contains: () => dark } } }
  globalThis.window = {
    go: bridge ? { main: { DesktopBridge: bridge } } : undefined,
    CSS: { supports: () => supports },
    matchMedia: name => {
      const query = { matches: false, handlers: new Set(),
        addEventListener: (_, fn) => query.handlers.add(fn),
        removeEventListener: (_, fn) => query.handlers.delete(fn) }
      queries.set(name, query)
      return query
    },
    addEventListener: (name, callback) => listeners.set(name, callback),
    removeEventListener: name => listeners.delete(name),
  }
  globalThis.MutationObserver = class {
    constructor(callback) { observer = callback }
    observe(_, options) { assert.deepEqual(options.attributeFilter, ['class']) }
    disconnect() { disconnected = true }
  }
}
const tick = () => new Promise(resolve => setImmediate(resolve))
setup()
let dispose = installDesktopAppearance()
assert.equal(document.documentElement.dataset.uiMaterial, 'frosted')
assert.equal(document.documentElement.dataset.windowMaterial, 'none')
const transparency = queries.get('(prefers-reduced-transparency: reduce)')
transparency.matches = true
for (const handler of transparency.handlers) handler()
assert.equal(document.documentElement.dataset.uiMaterial, 'solid')
dispose()
assert.equal(listeners.size, 0)
assert.ok(disconnected)
assert.ok([...queries.values()].every(query => query.handlers.size === 0))
assert.deepEqual(document.documentElement.dataset, {})

setup(undefined, false)
dispose = installDesktopAppearance()
assert.equal(document.documentElement.dataset.uiMaterial, 'solid')
dispose()

const calls = []
let mode = 'liquid'
setup({ ApplyAppearance: async value => {
  calls.push(value)
  return { platform: 'darwin', mode, reduce_motion: mode === 'solid' }
} })
dispose = installDesktopAppearance()
await tick()
assert.equal(document.documentElement.dataset.windowMaterial, 'liquid')
dark = true
await observer()
assert.deepEqual(calls, [false, true])
mode = 'solid'
await listeners.get('anidog:native-appearance-change')()
assert.equal(document.documentElement.dataset.uiMaterial, 'solid')
assert.equal(document.documentElement.dataset.reduceMotion, 'true')
mode = 'vibrancy'
await listeners.get('focus')()
assert.equal(document.documentElement.dataset.windowMaterial, 'vibrancy')
dispose()

// Rapid toggles coalesce, and an in-flight bridge call cannot mutate an unmounted app.
let finish
let active = 0
let maxActive = 0
const queuedCalls = []
setup({ ApplyAppearance: value => {
  active++
  maxActive = Math.max(maxActive, active)
  queuedCalls.push(value)
  return new Promise(resolve => { finish = () => { active--; resolve({ platform: 'darwin', mode: 'liquid' }) } })
} })
dispose = installDesktopAppearance()
dark = false
await observer()
finish()
await tick()
assert.equal(queuedCalls.at(-1), false)
assert.equal(maxActive, 1)
dispose()
finish()
await tick()
assert.deepEqual(document.documentElement.dataset, {})

setup({ ApplyAppearance: async () => { throw new Error('expected test failure') } })
const warn = console.warn
console.warn = () => {}
dispose = installDesktopAppearance()
await tick()
console.warn = warn
assert.equal(document.documentElement.dataset.windowMaterial, 'none')
assert.equal(document.documentElement.dataset.uiMaterial, 'frosted')
dispose()
console.log('appearance tests passed: platform matrix, CSS fallback, accessibility, concurrency, bridge failure and cleanup')
