import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'
import vue from '@vitejs/plugin-vue'
import { createSSRApp, h } from 'vue'
import { createI18n } from 'vue-i18n'
import { renderToString } from 'vue/server-renderer'
import { resolveDownloadStage, hasProgress, STAGE_KEYS, STREAM_STAGE_KEYS } from '../src/utils/downloadStage.js'

// 下载阶段轨道：把抽象流程画成逐个点亮的圆点（动森机场信息牌那次借鉴）。
// 这里盯住两件事：阶段必须由后端真实字段推导（不能靠猜），异常态必须能覆盖。

const read = (path) => readFile(new URL(path, import.meta.url), 'utf8')
const t = (key, params) => (params ? `${key}:${JSON.stringify(params)}` : key)

const base = { source: 'bt', download_type: 'torrent', status: 'downloading', progress: 0, downloaded_bytes: null }

// 来源只用于标签，不能决定传输阶段。Mikan/manual/未来新来源也必须服从实际类型。
for (const source of ['bt', 'rss', 'mikan', 'manual', 'stream', 'future-source', undefined]) {
  assert.deepEqual(
    resolveDownloadStage({ ...base, source, progress: 42 }, t),
    resolveDownloadStage({ ...base, progress: 42 }, t),
    `同为 torrent，${source} 来源不应改变阶段`,
  )
}

// 1. 正常路径
const pending = resolveDownloadStage({ ...base, status: 'pending' }, t)
assert.equal(pending.index, 0, '排队应在第 1 段')
assert.equal(pending.state, 'active')
assert.equal(pending.fill, 0)

const probing = resolveDownloadStage({ ...base, metadata_probe_started_at: '2026-01-01T00:00:00Z' }, t)
assert.equal(probing.index, 1, '磁力探测元数据时应停在第 2 段')
assert.equal(probing.label, 'pages.downloads.stageMetadata')
assert.equal(probing.fill, 0, '元数据阶段没有进度可言')

const noBytes = resolveDownloadStage({ ...base, progress: 0, downloaded_bytes: 0 }, t)
assert.equal(noBytes.index, 1, '一个字节都没下到时按取元数据处理')

const downloading = resolveDownloadStage({ ...base, progress: 62.3, downloaded_bytes: 8123456789 }, t)
assert.equal(downloading.index, 2)
assert.equal(downloading.fill, 62.3, '下载阶段的填充等于真实进度')
assert.equal(downloading.label, 'pages.downloads.stageDownloading')

const done = resolveDownloadStage({ ...base, status: 'completed', progress: 100 }, t)
assert.equal(done.index, 3)
assert.equal(done.state, 'done')
assert.equal(done.fill, 100)

// 2. 异常态必须压过正常阶段（后端有停滞检测与换源标记，不能白算）
const stalled = resolveDownloadStage({ ...base, progress: 12.7, downloaded_bytes: 987654321, stalled_since: '2026-01-01T00:00:00Z' }, t)
assert.equal(stalled.state, 'stalled', '停滞应改变轨道状态')
assert.equal(stalled.index, 2, '停滞发生在下载段')
assert.equal(stalled.label, 'pages.downloads.stageStalled')

const seeking = resolveDownloadStage({ ...base, progress: 31.5, downloaded_bytes: 1, seeking_alternative: true }, t)
assert.equal(seeking.label, 'pages.downloads.stageSeekingAlternative')
assert.equal(seeking.state, 'active', '换源仍在进行，不等于停滞')
// 停滞优先于换源（更严重的信号先展示）
const both = resolveDownloadStage({ ...base, progress: 31.5, downloaded_bytes: 1, stalled_since: 'x', seeking_alternative: true }, t)
assert.equal(both.label, 'pages.downloads.stageStalled')

// 3. 失败 / 暂停 / 替代要停在"实际到达过"的位置，不能一律退回起点
const failedLate = resolveDownloadStage({ ...base, status: 'failed', progress: 28.4, downloaded_bytes: 3210000000 }, t)
assert.equal(failedLate.index, 2)
assert.equal(failedLate.state, 'failed')
const failedEarly = resolveDownloadStage({ ...base, status: 'failed', progress: 0, downloaded_bytes: null }, t)
assert.equal(failedEarly.index, 0, '还没下到东西就失败，应停在起点')

const paused = resolveDownloadStage({ ...base, status: 'paused', progress: 45.1, downloaded_bytes: 45 }, t)
assert.equal(paused.state, 'paused')
assert.equal(paused.index, 2)
assert.equal(paused.fill, 45.1)
const superseded = resolveDownloadStage({ ...base, status: 'superseded', progress: 10 }, t)
assert.equal(superseded.state, 'paused')

// 4. 真实下载类型决定阶段，不使用来源名推断。未知类型不能冒充 BT。
for (const source of ['bt', 'rss', 'mikan', 'stream', undefined]) {
  const stream = resolveDownloadStage({ ...base, source, download_type: 'stream' }, t)
  assert.deepEqual(stream.stages.map(s => s.key), STREAM_STAGE_KEYS)
  assert.equal(stream.index, 1)
  assert.equal(stream.label, 'pages.downloads.stageDownloading')
  assert.equal(resolveDownloadStage({ ...base, source, download_type: undefined }, t), null)
  assert.equal(resolveDownloadStage({ ...base, source, download_type: 'future' }, t), null)
}
for (const [status, index, state] of [['pending', 0, 'active'], ['queued', 0, 'active'], ['paused', 1, 'paused'], ['failed', 1, 'failed'], ['superseded', 1, 'paused'], ['completed', 2, 'done']]) {
  const stream = resolveDownloadStage({ ...base, download_type: 'stream', status, progress: 42 }, t)
  assert.equal(stream.index, index)
  assert.equal(stream.state, state)
}
assert.equal(resolveDownloadStage({ ...base, status: 'unexpected' }, t), null)
assert.equal(resolveDownloadStage(null, t), null)

// 5. hasProgress 的边界
assert.equal(hasProgress({ progress: 0, downloaded_bytes: 0 }), false)
assert.equal(hasProgress({ progress: 0.1 }), true)
assert.equal(hasProgress({ downloaded_bytes: 1 }), true)
assert.deepEqual(STAGE_KEYS, ['stageQueued', 'stageMetadata', 'stageDownloading', 'stageCompleted'])

// 6. 组件与接线
const track = await read('../src/components/ac/AcStageTrack.vue')
assert.match(track, /class="ac-stage-track"/)
assert.match(track, /data-filled/)
assert.match(track, /:aria-label="ariaLabel"/, '轨道要有无障碍文本（只有视觉是不够的）')
assert.match(track, /function dotStatus\(i\)/)

const flip = await read('../src/components/ac/AcFlipText.vue')
assert.match(flip, /:key="text"/, '翻牌靠 :key 触发一次入场动画')

const css = await read('../src/assets/tailwind.css')
assert.match(css, /\.ac-stage-track__fill/)
assert.match(css, /@keyframes ac-stage-halo/)
assert.match(css, /@keyframes ac-flip-in/)
// 呼吸光环用缩放而不是动画 box-shadow（后者每帧重绘）
assert.match(css, /\.ac-stage-track__dot\[data-status='active'\]::after/)
assert.match(css, /@keyframes ac-stage-halo \{\s*0% \{\s*transform: scale\(0\.6\)/)

const list = await read('../src/views/Downloads/DownloadList.vue')
assert.match(list, /<DownloadTaskProgress :task="task" :t="t"/)
assert.doesNotMatch(list, /stageOf\(|<AcStageTrack|<AcProgress/, '列表不能保留另一套进度显示分支')

// 7. 渲染实际 Vue 组件，验证终态、类型、来源和拒收结果的接线。
const server = await createServer({
  configFile: false, plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('../src', import.meta.url)) } },
  server: { middlewareMode: true }, appType: 'custom',
})
try {
  const { default: Progress } = await server.ssrLoadModule('/src/components/Downloads/DownloadTaskProgress.vue')
  const translate = key => key === 'status.completed' ? '已完成' : key
  const render = task => renderToString(createSSRApp({ render: () => h(Progress, { task, t: translate }) })
    .use(createI18n({ legacy: false, messages: {} })))
  const completed = await render({ ...base, status: 'completed', progress: 100 })
  assert.match(completed, /已完成 · 100%/)
  assert.match(completed, /width:100%/)
  assert.doesNotMatch(completed, /ac-stage-track/)
  for (const source of ['bt', 'rss', 'mikan', 'stream', 'manual', 'future', undefined]) {
    for (const download_type of ['torrent', 'stream', 'future', undefined]) {
      assert.equal(await render({ source, download_type, status: 'completed', progress: 0, metadata_probe_started_at: 'stale', stalled_since: 'stale' }), completed)
    }
    const active = await render({ ...base, source, progress: 42 })
    assert.equal((active.match(/class="ac-stage-track__dot"/g) || []).length, 4)
    assert.match(active, /stageDownloading/)
  }
  const streaming = await render({ ...base, download_type: 'stream', source: 'bt', progress: 42, eta: '02:34' })
  assert.equal((streaming.match(/class="ac-stage-track__dot"/g) || []).length, 3)
  assert.doesNotMatch(streaming, /stageMetadata/)
  assert.match(streaming, /02:34/, '改为阶段展示后仍保留流媒体预计剩余时间')
  const unverified = await render({ ...base, progress: 100 })
  assert.match(unverified, /stageDownloading/)
  assert.doesNotMatch(unverified, /已完成/)
  for (const result_class of ['review', 'rejected', 'reused']) {
    assert.doesNotMatch(await render({ ...base, status: 'failed', progress: 100, result_class }), /ac-stage-track|已完成/)
  }
  assert.match(await render({ ...base, status: 'failed', result_class: 'error' }), /data-state="failed"/)
  const unknown = await render({ ...base, download_type: undefined, progress: 42 })
  assert.match(unknown, /data-presentation="progress"/)
  assert.doesNotMatch(unknown, /ac-stage-track|stageMetadata/)
} finally {
  await server.close()
}

console.log('download stage tests passed')
