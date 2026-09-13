<template>
  <Teleport to="body">
    <div
      v-if="phase !== 'idle'"
      class="ac-iris"
      :data-phase="phase"
      aria-hidden="true"
    />
  </Teleport>
</template>

<script setup>
import { ref } from 'vue'

// iris 转场：给"重操作"一点仪式感。
// 动森离岛时用一个带圆形镂空的遮罩收缩再展开，这里用同构的做法：
// 深色圆从点击处扩满全屏 → 真正干活 → 再从中心收回，露出新内容。
//
// 用法：await iris.run(() => doSomethingHeavy())
const phase = ref('idle')

async function run(task) {
  const reduced = typeof document !== 'undefined' &&
    (document.documentElement.dataset.reduceMotion === 'true' ||
      window.matchMedia?.('(prefers-reduced-motion: reduce)').matches)
  if (reduced) return task()

  phase.value = 'closing'
  await wait(320)
  try {
    return await task()
  } finally {
    phase.value = 'opening'
    await wait(340)
    phase.value = 'idle'
  }
}

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

defineExpose({ run })
</script>
