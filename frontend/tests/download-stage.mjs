import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { resolveDownloadStage, hasProgress, STAGE_KEYS } from '../src/utils/downloadStage.js'

// 下载阶段轨道：把抽象流程画成逐个点亮的圆点（动森机场信息牌那次借鉴）。
// 这里盯住两件事：阶段必须由后端真实字段推导（不能靠猜），异常态必须能覆盖。

const read = (path) => readFile(new URL(path, import.meta.url), 'utf8')
const t = (key, params) => (params ? `${key}:${JSON.stringify(params)}` : key)

const base = { source: 'bt', status: 'downloading', progress: 0, downloaded_bytes: null }

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

// 4. 流媒体不硬套 BT 阶段
assert.equal(resolveDownloadStage({ ...base, source: 'stream' }, t), null, '流媒体任务应返回 null 走普通进度条')
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
assert.match(list, /resolveDownloadStage\(task, t, t\)/, '下载行必须用纯函数推导阶段')
assert.match(list, /<AcStageTrack/)
assert.match(list, /<AcFlipText :text="stageOf\(task\)\.label"/)
assert.match(list, /<AcCountUp :value="task\.progress \|\| 0"/, '百分比也要滚动')

console.log('download stage tests passed')
