// Pure presentation policy. Pages and components never need OS checks.
export function resolveAppearance({ native, supportsBlur, reduceTransparency, reduceMotion, highContrast }) {
  const platform = ['darwin', 'windows', 'linux'].includes(native?.platform) ? native.platform : 'web'
  const windowMaterial = platform === 'darwin' && ['liquid', 'vibrancy', 'solid'].includes(native?.mode)
    ? native.mode : 'none'
  const opaque = !supportsBlur || reduceTransparency || highContrast || native?.reduce_transparency || windowMaterial === 'solid'
  return {
    platform,
    windowMaterial,
    uiMaterial: opaque ? 'solid' : 'frosted',
    reduceMotion: Boolean(reduceMotion || native?.reduce_motion),
  }
}
