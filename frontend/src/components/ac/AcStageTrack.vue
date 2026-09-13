<template>
  <div class="ac-stage-track" :data-state="state" role="img" :aria-label="ariaLabel">
    <template v-for="(stage, i) in stages" :key="stage.key">
      <span
        v-if="i > 0"
        class="ac-stage-track__link"
        :data-filled="i <= index ? 'true' : 'false'"
      >
        <span
          v-if="i === index && state === 'active'"
          class="ac-stage-track__fill"
          :style="{ width: `${clampedFill}%` }"
        />
      </span>
      <span class="ac-stage-track__dot" :data-status="dotStatus(i)" />
    </template>
  </div>
</template>

<script setup>
import { computed } from 'vue'

// 阶段轨道：把"找源 → 取元数据 → 下载 → 完成"这种抽象流程画成逐个点亮的圆点。
// 抄自动森机场那块信息牌——好友的飞机靠近时，底部的圆点从对岸逐个亮到"即将到达"。
// 只用真实存在的数据点亮，不猜阶段。
const props = defineProps({
  stages: { type: Array, required: true }, // [{ key, label }]
  index: { type: Number, default: 0 },     // 当前阶段下标
  state: { type: String, default: 'active' }, // active | done | failed | paused | stalled
  progress: { type: Number, default: 0 },  // 当前阶段内的完成度 0-100
})

const clampedFill = computed(() => Math.max(0, Math.min(100, props.progress || 0)))

const ariaLabel = computed(() => {
  const current = props.stages[props.index]?.label ?? ''
  return `${current} (${props.index + 1}/${props.stages.length})`
})

function dotStatus(i) {
  if (props.state === 'failed' && i === props.index) return 'failed'
  if (props.state === 'stalled' && i === props.index) return 'stalled'
  if (i < props.index) return 'done'
  if (i > props.index) return 'todo'
  if (props.state === 'done') return 'done'
  if (props.state === 'paused') return 'paused'
  return 'active'
}
</script>
