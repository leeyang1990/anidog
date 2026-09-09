export function isDesktopRuntime() {
  return import.meta.env.MODE === 'desktop' ||
    window.location.protocol === 'wails:' ||
    Boolean(window.go?.main?.DesktopBridge)
}

function bridge() {
  return window.go?.main?.DesktopBridge || null
}

export async function selectDesktopDirectory(current = '') {
  const fn = bridge()?.SelectDirectory
  if (!fn) return null
  const selected = await fn(current || '')
  return selected || null
}

export async function openDesktopPath(path = '') {
  const fn = bridge()?.OpenPath
  if (!fn) return false
  await fn(path || '')
  return true
}
