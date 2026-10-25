<script setup>
import { computed } from 'vue'
import { downloadOutcome, subtitleFacts } from '../../utils/downloadOutcome'
const props = defineProps({ task: { type: Object, required: true }, t: { type: Function, required: true } })
const diagnostic = computed(() => ['failed', 'paused'].includes(props.task.status)
  ? props.task.last_error || props.task.error_message || props.task.quality_note || '' : '')
const outcome = computed(() => downloadOutcome(props.task))
const evidence = computed(() => subtitleFacts(props.task))
</script>

<template>
  <div v-if="task.inspection_active" class="text-xs text-ac-sky-dark mt-0.5" role="status">{{ t('pages.downloads.validatingMedia') }}</div>
  <div v-if="task.validation_pending && task.status === 'downloading'" class="text-xs text-ac-sky-dark mt-0.5" role="status">{{ t('pages.downloads.validatingMedia') }}</div>
  <div v-if="task.status === 'failed'" class="text-xs font-bold mt-0.5" :class="outcome === 'error' ? 'text-ac-heart-dark' : 'text-ac-sun-dark'">{{ t(`pages.downloads.outcome_${outcome}`) }}</div>
  <div v-if="diagnostic" class="text-xs truncate mt-0.5" :class="outcome === 'error' ? 'text-ac-heart-dark' : 'text-muted-foreground'" :title="diagnostic" role="status">{{ diagnostic }}</div>
  <div v-if="task.subtitle_evidence" class="text-xs text-muted-foreground mt-0.5">
    <span v-if="evidence.method === 'subtitle_tracks'">{{ t('pages.downloads.nativeSubtitleTracks') }}：{{ evidence.tracks.join('、') }}</span>
    <span v-else>{{ t('pages.downloads.subtitleProof', { language: evidence.language ? t(`pages.downloads.language_${evidence.language}`) : t('pages.downloads.language_unknown'), captions: evidence.captions, attempted: evidence.attempted }) }}</span>
    <span v-if="evidence.method !== 'subtitle_tracks' && evidence.tracks.length"> · {{ t('pages.downloads.nativeSubtitleTracks') }}：{{ evidence.tracks.join('、') }}</span>
    <span v-if="task.evidence_reused_from"> · {{ t('pages.downloads.proofReused', { id: task.evidence_reused_from }) }}</span>
  </div>
  <div v-if="task.purpose === 'season_upgrade'" class="text-xs text-ac-sky-dark mt-0.5" :title="task.upgrade_reason">
    {{ task.upgrade_reason || t('pages.downloads.seasonUpgrade') }}
    <span v-if="task.backup_path" class="block text-muted-foreground" :title="task.backup_path">{{ t('pages.downloads.upgradeBackup') }}</span>
  </div>
  <div v-else-if="task.scope === 'season' && task.episode_start && task.episode_end" class="text-xs text-muted-foreground mt-0.5">
    {{ t('pages.downloads.seasonPack', { start: String(task.episode_start).padStart(2, '0'), end: String(task.episode_end).padStart(2, '0') }) }}
  </div>
  <div v-else-if="task.episode_number" class="text-xs text-muted-foreground mt-0.5">
    {{ t('pages.downloads.episode', { episode: String(task.episode_number).padStart(2, '0') }) }}
  </div>
  <div v-if="task.status === 'failed' && task.recovery_note" class="text-xs text-ac-sky-dark truncate mt-0.5" :title="task.recovery_note">
    {{ t('pages.downloads.recoveryStatus') }}：{{ task.recovery_note }}
  </div>
</template>
