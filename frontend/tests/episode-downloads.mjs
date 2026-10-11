import assert from 'node:assert/strict'
import { groupEpisodeDownloads } from '../src/utils/episodeDownloads.js'

const pack = { id: 324, scope: 'season', episode_number: 1, episode_start: 1, episode_end: 24, status: 'completed' }
const episodes = Array.from({ length: 24 }, (_, i) => ({ episode_number: i + 1, status: 'completed', download_id: pack.id }))
const grouped = groupEpisodeDownloads([pack], episodes)
assert.equal(Object.values(grouped).filter(rows => rows[0]?.id === pack.id).length, 24)

const partial = episodes.map(ep => ep.episode_number === 24 ? { ...ep, status: 'missing', download_id: undefined } : ep)
assert.deepEqual(groupEpisodeDownloads([pack], partial)[24], [], '未归档的集不能继承合集完成状态')
assert.deepEqual(groupEpisodeDownloads([{ ...pack, media_missing: true }], episodes)[1], [])
assert.deepEqual(groupEpisodeDownloads([{ ...pack, status: 'superseded' }], episodes)[1], [])
for (const status of ['downloading', 'pending', 'paused', 'failed']) {
  const active = { ...pack, status, episode_start: 2, episode_end: 12 }
  const result = groupEpisodeDownloads([active], episodes)
  assert.deepEqual(result[1], [])
  assert.deepEqual(result[12], [active])
  assert.deepEqual(result[13], [])
}
const single = { id: 325, scope: 'episode', episode_number: 2, status: 'failed' }
assert.deepEqual(groupEpisodeDownloads([pack, single], episodes)[2], [pack, single])
assert.deepEqual(groupEpisodeDownloads([pack, single], episodes)[3], [pack])
console.log('episode download mapping tests passed')
