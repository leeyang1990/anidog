const RELOAD_GUARD_KEY = 'anidog:preload-reload-at'
const RELOAD_GUARD_MS = 15_000

// Vite 在版本发布后找不到旧的懒加载 chunk 时会派发 vite:preloadError。
// 自动刷新一次，让仍然打开着旧页面的浏览器加载新版本入口文件。
export function installPreloadRecovery(windowRef = globalThis.window, now = () => Date.now()) {
  if (!windowRef?.addEventListener) return

  let reloadRequested = false
  windowRef.addEventListener('vite:preloadError', (event) => {
    if (reloadRequested) return

    let lastReloadAt = 0
    try {
      lastReloadAt = Number(windowRef.sessionStorage?.getItem(RELOAD_GUARD_KEY)) || 0
    } catch {
      // sessionStorage 不可用时仍允许本次恢复。
    }

    const currentTime = now()
    if (currentTime - lastReloadAt < RELOAD_GUARD_MS) return

    event.preventDefault()
    reloadRequested = true
    try {
      windowRef.sessionStorage?.setItem(RELOAD_GUARD_KEY, String(currentTime))
    } catch {
      // 无存储能力时使用当前页面生命周期内的一次性保护。
    }
    windowRef.location.reload()
  })
}

export { RELOAD_GUARD_KEY, RELOAD_GUARD_MS }
