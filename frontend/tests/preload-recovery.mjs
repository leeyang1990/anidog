import assert from 'node:assert/strict'

import {
  installPreloadRecovery,
  RELOAD_GUARD_KEY,
  RELOAD_GUARD_MS,
} from '../src/utils/preloadRecovery.js'

function createWindow(lastReloadAt = null) {
  const listeners = new Map()
  const values = new Map()
  if (lastReloadAt !== null) values.set(RELOAD_GUARD_KEY, String(lastReloadAt))

  const windowRef = {
    reloads: 0,
    addEventListener(type, listener) { listeners.set(type, listener) },
    dispatch(type, event) { listeners.get(type)?.(event) },
    sessionStorage: {
      getItem(key) { return values.get(key) ?? null },
      setItem(key, value) { values.set(key, value) },
    },
    location: {
      reload: () => { windowRef.reloads++ },
    },
  }
  return windowRef
}

const now = 100_000
const freshWindow = createWindow()
let prevented = false
installPreloadRecovery(freshWindow, () => now)
freshWindow.dispatch('vite:preloadError', { preventDefault: () => { prevented = true } })
assert.equal(prevented, true)
assert.equal(freshWindow.reloads, 1)
assert.equal(freshWindow.sessionStorage.getItem(RELOAD_GUARD_KEY), String(now))

const guardedWindow = createWindow(now - RELOAD_GUARD_MS + 1)
prevented = false
installPreloadRecovery(guardedWindow, () => now)
guardedWindow.dispatch('vite:preloadError', { preventDefault: () => { prevented = true } })
assert.equal(prevented, false, 'reload loop guard should let Vite surface the repeated error')
assert.equal(guardedWindow.reloads, 0)

const expiredWindow = createWindow(now - RELOAD_GUARD_MS)
installPreloadRecovery(expiredWindow, () => now)
expiredWindow.dispatch('vite:preloadError', { preventDefault() {} })
assert.equal(expiredWindow.reloads, 1)

console.log('preload recovery tests passed')
