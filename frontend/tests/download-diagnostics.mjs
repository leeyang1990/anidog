import assert from 'node:assert/strict'
import { createServer } from 'vite'
import vue from '@vitejs/plugin-vue'
import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'

const server = await createServer({ configFile: false, plugins: [vue()], server: { middlewareMode: true }, appType: 'custom' })
try {
  const { default: Description } = await server.ssrLoadModule('/src/components/Downloads/DownloadTaskDescription.vue')
  const t = (key, args) => key + (args ? JSON.stringify(args) : '')
  const render = task => renderToString(createSSRApp({ render: () => h(Description, { task, t }) }))
  for (const metadata of [{ scope: 'season', episode_start: 1, episode_end: 12 }, { purpose: 'season_upgrade', upgrade_reason: 'upgrade', backup_path: '/backup' }]) {
    for (const status of ['paused', 'failed']) {
      const html = await render({ ...metadata, status, last_error: 'actual failure', quality_note: 'old note' })
      assert.match(html, /actual failure/)
      assert.doesNotMatch(html, /old note/)
      assert.match(html, metadata.scope ? /seasonPack/ : /upgradeBackup/)
    }
  }
  assert.match(await render({ status: 'paused', quality_note: 'capacity reason' }), /capacity reason/)
  assert.match(await render({ status: 'failed', error_message: 'legacy reason', recovery_note: 'retry pending' }), /legacy reason.*retry pending/s)
  assert.doesNotMatch(await render({ status: 'completed', last_error: 'stale failure' }), /stale failure/)
  const escaped = await render({ status: 'paused', last_error: '<script>alert(1)</script>' })
  assert.match(escaped, /&lt;script&gt;/)
  assert.doesNotMatch(escaped, /<script>/)
  console.log('download diagnostic rendering tests passed')
} finally {
  await server.close()
}
