<template>
  <span class="ac-flip-text"><span :key="text" class="ac-flip-text__face">{{ text }}</span></span>
</template>

<script setup>
// 翻牌文字：内容变化时像机场信息屏那样翻一下。
// 只做"换字"这一件事——用 :key 触发一次入场动画，不需要额外的状态机。
defineProps({
  text: { type: String, default: '' },
})
</script>
