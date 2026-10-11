import assert from 'node:assert/strict'
import { calendarDay, calendarDate, calendarDays } from '../src/utils/calendar.js'
for (const day of [undefined, null, '', '7', '-1', [], 'invalid']) {
  assert.equal(calendarDay(day, 4), 4, 'invalid query must not select Sunday')
}
assert.equal(calendarDay('0', 4), 0)
assert.equal(calendarDay('1', 4), 1)
assert.equal(calendarDate(new Date(2026, 9, 11)), '2026-10-11')
const data = calendarDays([
  { weekday_id: 7, items: [{ id: 1, name: 'Sunday', is_subscribed: true }] },
  { weekday_id: 4, items: [{ id: 2, name_cn: 'Thursday', local_id: 18 }] },
  { weekday_id: null, items: [{ id: 3, name: 'Unknown' }] },
  { weekday_id: 1, items: null },
], 4)
assert.deepEqual(data.map(d => d.weekday), [4, 5, 6, 0, 1, 2, 3])
assert.equal(data[0].items[0].local_id, 18)
assert.equal(data[3].items[0].name, 'Sunday')
assert.equal(data.reduce((sum, d) => sum + d.items.length, 0), 2)
assert.equal(calendarDays(null, 0).length, 7)
console.log('calendar tests passed')
