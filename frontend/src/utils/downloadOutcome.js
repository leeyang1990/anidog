export function downloadOutcome(task) {
  if (task.result_class) return task.result_class
  if (task.status !== 'failed') return task.status
  if (task.failure_code === 'subtitle_unknown' || task.failure_kind === 'review') return 'review'
  if (task.failure_code === 'duplicate_media') return 'reused'
  if (task.failure_kind === 'rejected' && task.failure_code !== 'player_unavailable') return 'rejected'
  return 'error'
}

export function subtitleFacts(task) {
  try {
    const evidence = JSON.parse(task.subtitle_evidence || '{}')
    return { language: evidence.language || '', method: evidence.method || '', tracks: Array.isArray(evidence.track_languages) ? evidence.track_languages : [], attempted: Number(evidence.attempted || 0), captions: (evidence.samples || []).filter(x => x.caption).length, reason: evidence.reason || '' }
  } catch { return { language: '', method: '', tracks: [], attempted: 0, captions: 0, reason: '' } }
}
