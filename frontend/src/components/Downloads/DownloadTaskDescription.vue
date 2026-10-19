<script setup>
import { computed } from 'vue'
const props = defineProps({ task: { type: Object, required: true }, t: { type: Function, required: true } })
const diagnostic = computed(() => ['failed', 'paused'].includes(props.task.status)
  ? props.task.last_error || props.task.error_message || props.task.quality_note || '' : '')
</script>

<template>
  <div v-if="diagnostic" class="text-xs text-ac-heart-dark truncate mt-0.5" :title="diagnostic" role="status">{{ diagnostic }}</div>
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
