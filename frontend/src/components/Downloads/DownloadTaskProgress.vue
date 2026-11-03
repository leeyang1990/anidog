<template>
  <div v-if="task.status === 'completed'" class="download-task-progress" data-presentation="completed">
    <AcProgress :value="100" :height="4" variant="leaf" />
    <div class="mt-1 text-[11px] font-num text-ac-leaf-dark font-bold">{{ t('status.completed') }} · 100%</div>
  </div>
  <div v-else-if="stage" class="download-task-progress" data-presentation="stages">
    <AcStageTrack :stages="stage.stages" :index="stage.index" :state="stage.state" :progress="stage.fill" class="mb-1" />
    <div class="flex items-center justify-between gap-1 text-[11px] font-num">
      <AcFlipText :text="stage.label" :class="stage.labelClass" />
      <span class="text-muted-foreground"><AcCountUp :value="progress" :duration="450" />%</span>
    </div>
    <div v-if="task.eta && task.status === 'downloading'" class="text-[11px] text-muted-foreground font-num">{{ task.eta }}</div>
  </div>
  <div v-else-if="task.status === 'downloading' || task.status === 'paused'" class="download-task-progress" data-presentation="progress">
    <div class="flex items-center justify-between text-xs mb-0.5 font-num">
      <span class="text-muted-foreground"><AcCountUp :value="progress" :duration="450" />%</span>
      <span v-if="task.eta && task.status === 'downloading'" class="text-muted-foreground">{{ task.eta }}</span>
    </div>
    <AcProgress :value="progress" :height="4" :variant="task.status === 'paused' ? 'wood' : 'grass'" />
  </div>
  <div v-else class="text-xs text-muted-foreground">—</div>
</template>

<script setup>
import { computed } from 'vue'
import { resolveDownloadStage } from '../../utils/downloadStage'
import { downloadOutcome } from '../../utils/downloadOutcome'
import AcProgress from '../ac/AcProgress.vue'
import AcStageTrack from '../ac/AcStageTrack.vue'
import AcFlipText from '../ac/AcFlipText.vue'
import AcCountUp from '../ac/AcCountUp.vue'

const props = defineProps({
  task: { type: Object, required: true },
  t: { type: Function, required: true },
})

// 完成态优先于传输细节；字节下载到 100% 不代表已经通过验收入库。
const progress = computed(() => Number(props.task.progress) || 0)
const stage = computed(() => {
  if (props.task.status === 'completed') return null
  if (props.task.status === 'failed' && downloadOutcome(props.task) !== 'error') return null
  return resolveDownloadStage(props.task, props.t)
})
</script>
