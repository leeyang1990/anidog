import { resolveAppearance } from './appearancePolicy.js'

// One lifecycle-managed controller for web and every desktop platform.
export function installDesktopAppearance() {
  const root = document.documentElement
  const bridge = window.go?.main?.DesktopBridge
  const media = {
    reduceTransparency: window.matchMedia('(prefers-reduced-transparency: reduce)'),
    reduceMotion: window.matchMedia('(prefers-reduced-motion: reduce)'),
    highContrast: window.matchMedia('(prefers-contrast: more)'),
  }
  const supportsBlur = window.CSS?.supports('backdrop-filter', 'blur(1px)') ||
    window.CSS?.supports('-webkit-backdrop-filter', 'blur(1px)') || false
  let native
  let disposed = false
  let updating = false
  let pending = false
  const render = () => {
    if (disposed) return
    const state = resolveAppearance({ native, supportsBlur,
      ...Object.fromEntries(Object.entries(media).map(([key, value]) => [key, value.matches])),
    })
    root.dataset.platform = state.platform
    root.dataset.windowMaterial = state.windowMaterial
    root.dataset.uiMaterial = state.uiMaterial
    root.dataset.reduceMotion = String(state.reduceMotion)
  }
  const update = async () => {
    if (disposed || !bridge?.ApplyAppearance) return
    if (updating) { pending = true; return }
    updating = true
    try {
      native = await bridge.ApplyAppearance(root.classList.contains('dark'))
    } catch (error) {
      native = undefined
      console.warn('Native appearance unavailable; using web materials', error)
    } finally {
      render()
      updating = false
      if (pending && !disposed) { pending = false; void update() }
    }
  }
  render()
  void update()
  const observer = new MutationObserver(update)
  observer.observe(root, { attributes: true, attributeFilter: ['class'] })
  for (const query of Object.values(media)) query.addEventListener('change', render)
  window.addEventListener('anidog:native-appearance-change', update)
  window.addEventListener('focus', update)
  return () => {
    disposed = true
    observer.disconnect()
    for (const query of Object.values(media)) query.removeEventListener('change', render)
    window.removeEventListener('anidog:native-appearance-change', update)
    window.removeEventListener('focus', update)
    for (const key of ['platform', 'windowMaterial', 'uiMaterial', 'reduceMotion']) delete root.dataset[key]
  }
}
