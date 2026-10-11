export function calendarDay(queryDay, today) {
  if (typeof queryDay !== 'string' || !/^[0-6]$/.test(queryDay)) return today
  return Number(queryDay)
}

export function calendarDate(date = new Date()) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

export function calendarDays(data, today) {
  const days = Array.from({ length: 7 }, (_, weekday) => ({ weekday, isToday: weekday === today, items: [] }))
  for (const day of Array.isArray(data) ? data : []) {
    const wd = day.weekday_id === 7 ? 0 : day.weekday_id
    if (!Number.isInteger(wd) || wd < 0 || wd > 6) continue
    days[wd].items.push(...(Array.isArray(day.items) ? day.items : []).map(item => ({
      id: item.id, name: item.name_cn || item.name, image: item.image,
      rating_score: item.rating_score, air_date: item.air_date,
      is_subscribed: item.is_subscribed, local_id: item.local_id
    })))
  }
  return days.sort((a, b) => (a.weekday - today + 7) % 7 - (b.weekday - today + 7) % 7)
}
