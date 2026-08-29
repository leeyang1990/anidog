import assert from 'node:assert/strict'

const storage = new Map()
globalThis.localStorage = {
  getItem: key => storage.get(key) ?? null,
  setItem: (key, value) => storage.set(key, String(value)),
}
Object.defineProperty(globalThis, 'navigator', {
  configurable: true,
  value: { language: 'ja-JP', languages: ['ja-JP'] },
})
globalThis.document = {
  documentElement: { lang: '' },
  createElement: () => ({}),
}

const { i18n, normalizeLocale, setLocale, localeOptions } = await import('../src/i18n/index.js')

assert.equal(normalizeLocale('zh-Hant-HK'), 'zh-TW')
assert.equal(normalizeLocale('en-GB'), 'en-US')
assert.equal(normalizeLocale('ja'), 'ja-JP')
assert.equal(normalizeLocale('fr-FR'), 'zh-CN')
assert.equal(i18n.global.locale.value, 'ja-JP')
assert.equal(localeOptions.length, 4)

function flattenKeys(value, prefix = '') {
  return Object.entries(value).flatMap(([key, child]) => {
    const path = prefix ? `${prefix}.${key}` : key
    return child && typeof child === 'object' && !Array.isArray(child) ? flattenKeys(child, path) : [path]
  })
}

const referenceKeys = flattenKeys(i18n.global.getLocaleMessage('zh-CN')).sort()
for (const localeName of localeOptions.map(option => option.value)) {
  assert.deepEqual(flattenKeys(i18n.global.getLocaleMessage(localeName)).sort(), referenceKeys, `${localeName} translation keys differ`)
}

setLocale('en-US')
assert.equal(i18n.global.locale.value, 'en-US')
assert.equal(storage.get('anidog.locale'), 'en-US')
assert.equal(document.documentElement.lang, 'en-US')
assert.equal(i18n.global.t('nav.dashboard'), 'Home')
assert.equal(i18n.global.t('pages.downloads.add'), 'Add')
assert.equal(i18n.global.t('pages.search.followDownload'), 'Follow & download')
assert.equal(i18n.global.t('pages.notifications.addChannel'), 'Add channel')
assert.equal(i18n.global.t('pages.rules.import'), 'Import')
assert.equal(i18n.global.t('pages.rss.add'), 'Add feed')

setLocale('zh-HK')
assert.equal(i18n.global.locale.value, 'zh-TW')
assert.equal(i18n.global.t('settings.tabs.appearance'), '外觀主題')

console.log('i18n tests passed')
