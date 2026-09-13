<template>
  <div class="ac-scene-loader" role="status" :aria-live="'polite'" :aria-label="label">
    <div class="ac-scene-loader__track" aria-hidden="true">
      <span class="ac-scene-loader__pulse" />
    </div>
    <div v-if="items.length" class="ac-scene-loader__chips" aria-hidden="true">
      <span
        v-for="(item, index) in items"
        :key="item"
        class="ac-scene-loader__chip"
        :style="{ '--ac-scan-index': index }"
      >{{ item }}</span>
    </div>
    <p class="ac-scene-loader__label">{{ label }}</p>
  </div>
</template>

<script setup>
// 场景化等待：把"正在并发探测多个站点"这件事画出来，而不是丢一个转圈。
//
// 动森的 loading 是有内容的（小飞机穿云、浮动小岛），但那套讲的是海岛世界观。
// 我们的场景是"同时问好几个资源站"，所以画成扫描线 + 依次点亮的站点标签。
//
// 诚实说明：后端是一次性聚合返回，没有逐站进度，所以这里的点亮是"进行中"的
// 循环提示，不是真实进度——因此标签用的是中性措辞，不显示"已完成几个"。
defineProps({
  label: { type: String, default: '' },
  items: { type: Array, default: () => [] },
})
</script>
