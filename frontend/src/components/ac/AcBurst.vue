<template>
  <span v-if="playing" :key="round" class="ac-burst" aria-hidden="true">
    <i
      v-for="i in spokes"
      :key="i"
      :style="{ '--ac-burst-angle': `${((i - 1) * 360) / spokes}deg` }"
    />
  </span>
</template>

<script setup>
import { onBeforeUnmount, ref } from 'vue'
import { prefersReducedMotion } from '@/composables/useCountUp'

// 确认反馈：选项被确认时向四周"炸"出速度线。
// 动森里选中条目按下后，条目后层的黄色条两侧会出现放射状图形——表达"这一下按实了"的力量感。
// 只给"确认/成功"这类有分量的动作用，普通 hover 不要加。
const props = defineProps({
  spokes: { type: Number, default: 8 },
  duration: { type: Number, default: 260 },
})

const playing = ref(false)
const round = ref(0)
let timer = 0

function play() {
  if (prefersReducedMotion()) return false
  round.value += 1
  playing.value = true
  clearTimeout(timer)
  timer = setTimeout(() => { playing.value = false }, props.duration)
  return true
}

onBeforeUnmount(() => clearTimeout(timer))

defineExpose({ play })
</script>
