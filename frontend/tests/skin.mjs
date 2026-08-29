import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { nextTick } from 'vue'

const storage = new Map([['skin', 'classic']])
const attributes = new Map()
globalThis.localStorage = {
  getItem: key => storage.get(key) ?? null,
  setItem: (key, value) => storage.set(key, String(value)),
}
globalThis.window = {}
globalThis.document = {
  documentElement: {
    setAttribute: (key, value) => attributes.set(key, String(value)),
  },
}

const { useSkin, SKINS } = await import('../src/composables/useSkin.js')
const { skin, setSkin } = useSkin()

assert.equal(skin.value, 'classic')
assert.equal(attributes.get('data-skin'), 'classic')
assert.ok(SKINS.every(item => item.labelKey && item.descriptionKey))

setSkin('ac-grove')
await nextTick()
assert.equal(attributes.get('data-skin'), 'ac-grove')
assert.equal(storage.get('skin'), 'ac-grove')

setSkin('invalid')
await nextTick()
assert.equal(skin.value, 'ac-grove')

const spinnerSource = await readFile(new URL('../src/components/ac/AcSpinner.vue', import.meta.url), 'utf8')
assert.match(spinnerSource, /ac-spinner-ring/)
assert.match(spinnerSource, /v-if="skin !== 'classic'"/)
assert.match(spinnerSource, /<svg v-else/)
assert.match(spinnerSource, /const \{ skin \} = useSkin\(\)/)

const dashboardSource = await readFile(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
assert.match(dashboardSource, /classic: \{ primary: '#6366F1'/)
assert.match(dashboardSource, /watch\(\[skin, locale\], rebuildCharts\)/)

const toastSource = await readFile(new URL('../src/components/ac/AcToastContainer.vue', import.meta.url), 'utf8')
assert.doesNotMatch(toastSource, /color="#7CB342"/)

const buttonSource = await readFile(new URL('../src/components/ac/AcButton.vue', import.meta.url), 'utf8')
assert.doesNotMatch(buttonSource, /return '#7CB342'/)
assert.match(buttonSource, /return 'currentColor'/)

const emptySource = await readFile(new URL('../src/components/ac/AcEmpty.vue', import.meta.url), 'utf8')
assert.match(emptySource, /v-if="skin !== 'classic'"/)
assert.match(emptySource, /常规主题：中性收件箱/)

console.log('skin tests passed')
