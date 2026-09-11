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

// Theme fonts must be available offline and must not be overridden by a platform.
const html = await readFile(new URL('../index.html', import.meta.url), 'utf8')
assert.doesNotMatch(html, /fonts\.googleapis|fonts\.gstatic/)
const fontCSS = await readFile(new URL('../src/assets/fonts.css', import.meta.url), 'utf8')
assert.match(fontCSS, /@fontsource\/zcool-kuaile\/400\.css/)
for (const weight of [400, 600, 700, 800]) {
  assert.ok(fontCSS.includes(`@fontsource/nunito/latin-${weight}.css`))
}
for (const family of ['zcool-kuaile', 'nunito']) {
  const bundledLicense = await readFile(new URL(`../public/font-licenses/${family}.txt`, import.meta.url), 'utf8')
  const sourceLicense = await readFile(new URL(`../node_modules/@fontsource/${family}/LICENSE`, import.meta.url), 'utf8')
  assert.equal(bundledLicense, sourceLicense)
}
console.log('offline font imports and licenses passed')
