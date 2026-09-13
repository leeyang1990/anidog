<template>
  <Teleport to="body">
    <transition name="ac-radial">
      <div
        v-if="open"
        class="fixed inset-0 z-[1200]"
        role="menu"
        :aria-label="label"
        @contextmenu.prevent
      >
        <div class="ac-radial__scrim absolute inset-0" @click="close" />
        <div class="ac-radial__ring" :style="{ left: `${x}px`, top: `${y}px` }">
          <span class="ac-radial__hub" aria-hidden="true" />
          <button
            v-for="(item, index) in items"
            :key="item.key"
            ref="itemEls"
            type="button"
            role="menuitem"
            class="ac-radial__item"
            :class="item.class"
            :disabled="item.disabled"
            :style="slotStyle(index)"
            :title="item.label"
            @click.stop="choose(item)"
          >
            <component :is="item.icon" class="size-4" aria-hidden="true" />
            <span class="ac-radial__label">{{ item.label }}</span>
          </button>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup>
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

// 环形快捷菜单：把高频动作摊在指针周围，不用精准去点某个小图标。
// 动森的工具环 / 表情环就是这么用的——摇杆一推就到，眼睛不用离开目标。
//
// 这里用在下载行：右键（或长按）在指针位置展开一圈操作。
const props = defineProps({
  open: { type: Boolean, default: false },
  x: { type: Number, default: 0 },
  y: { type: Number, default: 0 },
  items: { type: Array, default: () => [] }, // [{ key, label, icon, class, disabled }]
  label: { type: String, default: '' },
  radius: { type: Number, default: 78 },
})

const emit = defineEmits(['select', 'update:open'])
const itemEls = ref([])

// 以指针为圆心均分，从正上方开始顺时针排
function slotStyle(index) {
  const total = Math.max(props.items.length, 1)
  const angle = (index / total) * Math.PI * 2 - Math.PI / 2
  return {
    transform: `translate(calc(-50% + ${Math.cos(angle) * props.radius}px), calc(-50% + ${Math.sin(angle) * props.radius}px))`,
    '--ac-radial-delay': `${index * 28}ms`,
  }
}

function choose(item) {
  if (item.disabled) return
  emit('select', item.key)
  close()
}

function close() {
  emit('update:open', false)
}

function onKeydown(e) {
  if (e.key === 'Escape') close()
}

watch(() => props.open, async (open) => {
  if (open) {
    window.addEventListener('keydown', onKeydown)
    await nextTick()
    itemEls.value?.[0]?.focus?.()
  } else {
    window.removeEventListener('keydown', onKeydown)
  }
})

onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

defineExpose({ close })
</script>
