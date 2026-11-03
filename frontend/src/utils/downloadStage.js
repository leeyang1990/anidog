// 下载任务的阶段推导（纯函数，便于单测）。
//
// 只依据后端真实字段，不猜阶段：
//   pending / queued                  -> 排队
//   downloading + metadata_probe_started_at 有值 -> 取元数据（后端正在为磁力探测元数据）
//   downloading 但一个字节都还没下到     -> 取元数据
//   downloading 且有进度                -> 下载（fill = 进度）
//   completed                         -> 完成
//   异常态叠加：stalled_since（后端停滞检测）、seeking_alternative（已去抢别的源）
//
// download_type 决定传输阶段；source 只描述候选来源，不参与进度推导。
// 未知类型返回 null，不能根据来源猜成 BT。

export const STAGE_KEYS = ['stageQueued', 'stageMetadata', 'stageDownloading', 'stageCompleted']
export const STREAM_STAGE_KEYS = ['stageQueued', 'stageDownloading', 'stageCompleted']

/** 判定是否有进度可言：有字节或进度大于 0。 */
export function hasProgress(task) {
  return Number(task?.downloaded_bytes || 0) > 0 || Number(task?.progress || 0) > 0
}

/**
 * @param {object} task 下载任务（后端 Download JSON）
 * @param {(key: string) => string} t i18n 函数
 * @param {(key: string) => string} [tStatus] 状态文案函数，默认复用 t
 * @returns {{stages: Array, index: number, state: string, fill: number, label: string, labelClass: string}|null}
 */
export function resolveDownloadStage(task, t, tStatus) {
  const statusLabel = tStatus || t
  if (!task) return null
  const keys = task.download_type === 'torrent' ? STAGE_KEYS
    : task.download_type === 'stream' ? STREAM_STAGE_KEYS : null
  if (!keys) return null

  const stages = keys.map((key) => ({ key, label: t(`pages.downloads.${key}`) }))
  const downloadIndex = keys.indexOf('stageDownloading')
  const progress = Number(task.progress || 0)
  const reachedDownload = hasProgress(task)

  if (task.status === 'completed') {
    return {
      stages, index: stages.length - 1, state: 'done', fill: 100,
      label: t('pages.downloads.stageCompleted'), labelClass: 'text-ac-leaf-dark font-bold',
    }
  }

  if (task.status === 'failed') {
    return {
      stages, index: reachedDownload ? downloadIndex : 0, state: 'failed', fill: progress,
      label: statusLabel('status.failed'), labelClass: 'text-ac-heart-dark font-bold',
    }
  }

  if (task.status === 'paused') {
    return {
      stages, index: reachedDownload ? downloadIndex : 0, state: 'paused', fill: progress,
      label: statusLabel('status.paused'), labelClass: 'text-muted-foreground',
    }
  }

  if (task.status === 'pending' || task.status === 'queued') {
    return {
      stages, index: 0, state: 'active', fill: 0,
      label: t('pages.downloads.stageQueued'), labelClass: 'text-muted-foreground',
    }
  }

  if (task.status === 'superseded') {
    return {
      stages, index: reachedDownload ? downloadIndex : 0, state: 'paused', fill: progress,
      label: statusLabel('status.superseded'), labelClass: 'text-muted-foreground',
    }
  }

  if (task.status !== 'downloading') return null
  const inMetadata = task.download_type === 'torrent' && (Boolean(task.metadata_probe_started_at) || !reachedDownload)
  const stalled = Boolean(task.stalled_since)
  const seeking = Boolean(task.seeking_alternative)
  const index = inMetadata ? 1 : downloadIndex

  let label = t('pages.downloads.stageDownloading')
  let labelClass = 'text-ac-grass-dark font-bold'
  if (stalled) {
    label = t('pages.downloads.stageStalled')
    labelClass = 'text-ac-sun-dark font-bold'
  } else if (seeking) {
    label = t('pages.downloads.stageSeekingAlternative')
    labelClass = 'text-ac-sky-dark font-bold'
  } else if (inMetadata) {
    label = t('pages.downloads.stageMetadata')
    labelClass = 'text-muted-foreground'
  }

  return {
    stages, index,
    state: stalled ? 'stalled' : 'active',
    fill: inMetadata ? 0 : progress,
    label, labelClass,
  }
}
