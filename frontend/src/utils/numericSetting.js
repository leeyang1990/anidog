// Zero is a valid saved value, not a missing setting.
export function nonNegativeSetting(value, fallback = 0) {
  if (value === undefined || value === null || String(value).trim() === '') return fallback
  const number = Number(value)
  return Number.isFinite(number) ? Math.max(0, number) : fallback
}
