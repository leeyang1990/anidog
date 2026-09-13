<template>
  <Teleport to="body">
    <transition name="ac-settle">
      <div v-if="show" class="ac-settle" role="status" :aria-label="title">
        <div class="ac-settle__card">
          <p class="ac-settle__title">{{ title }}</p>
          <ul class="ac-settle__list">
            <li
              v-for="(item, index) in items"
              :key="item + index"
              class="ac-settle__row"
              :style="{ '--ac-settle-index': index }"
            >
              <CheckmarkCircle class="size-3.5 shrink-0 text-ac-leaf-dark" aria-hidden="true" />
              <span class="truncate">{{ item }}</span>
            </li>
          </ul>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup>
import { watch, onBeforeUnmount } from 'vue'
import { CheckmarkCircle } from '@vicons/ionicons5'

// 操作的"结算"：一批动作做完后，把结果一条条推出来再收走。
// 动森换装结束时，搭配好的部件会从人物周围依次飞出——做完的事要有交代，
// 而不是只留一句"操作成功"。
//
// 只在用户主动触发的批量操作上播；后台自动完成不要弹，会打扰。
const props = defineProps({
  show: { type: Boolean, default: false },
  title: { type: String, default: '' },
  items: { type: Array, default: () => [] },
  duration: { type: Number, default: 2600 },
})

const emit = defineEmits(['update:show'])
let timer = 0

watch(() => props.show, (show) => {
  clearTimeout(timer)
  if (!show) return
  // 条目多的时候按条数延长一点，保证每条都被看一眼
  const total = props.duration + Math.min(props.items.length, 8) * 120
  timer = setTimeout(() => emit('update:show', false), total)
})

onBeforeUnmount(() => clearTimeout(timer))
</script>
