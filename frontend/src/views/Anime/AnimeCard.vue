<template>
  <article
    class="ac-anime-card group relative flex cursor-pointer flex-col overflow-hidden rounded-xl border border-border/70 bg-card shadow-[0_1px_2px_rgb(0_0_0/0.05)] transition-[transform,box-shadow,border-color] duration-200 ease-out hover:-translate-y-1 hover:border-primary/40 hover:shadow-[0_14px_30px_-14px_rgb(0_0_0/0.35)] active:translate-y-0 active:duration-75"
    @click="$emit('click', item)"
  >
    <!-- 相框式海报：内缩一圈 + 独立圆角。外圆角 24px − 内距 6px = 内圆角 18px，两层正好同心 -->
    <div class="p-1.5">
      <div class="relative aspect-[2/3] overflow-hidden rounded-lg bg-muted/40">
        <AcPoster
          :src="item.image || ''"
          :alt="item.name_cn || item.name"
          :sizes="posterSizes"
        />

        <!-- 顶部轻压暗：角标压在任何封面上都不糊 -->
        <div class="pointer-events-none absolute inset-x-0 top-0 h-12 bg-gradient-to-b from-ac-night/35 to-transparent" />

        <!-- 海报状态角标 -->
        <div class="pointer-events-none absolute inset-x-1.5 top-1.5 flex items-start justify-between gap-1">
          <span
            v-if="item.rating_score"
            class="ac-poster-badge"
            :aria-label="`评分 ${formatRating(item.rating_score)}`"
          >
            <Star class="size-2.5 text-ac-sun" aria-hidden="true" />
            {{ formatRating(item.rating_score) }}
          </span>
          <span v-else />

          <AcStamp v-if="item.is_subscribed">
            <span class="ac-poster-badge ac-poster-badge--grass" aria-label="已追番">
              <CheckmarkCircle class="size-2.5" aria-hidden="true" />
            </span>
          </AcStamp>
        </div>

        <!-- 追番按钮（悬浮） -->
        <div
          v-if="!item.is_subscribed"
          class="absolute inset-x-0 bottom-0 bg-gradient-to-t from-ac-night/65 to-transparent p-1.5 opacity-0 transition-opacity duration-200 group-hover:opacity-100 group-focus-within:opacity-100"
        >
          <button
            type="button"
            class="relative h-6 w-full rounded-lg bg-ac-grass/95 text-[10.5px] font-bold text-white shadow-sm transition-colors hover:bg-ac-grass-dark active:translate-y-px"
            @click.stop="onSubscribe(item)"
          >
            + 追番
            <AcBurst ref="burstRef" :spokes="8" />
          </button>
        </div>
      </div>
    </div>

    <!-- 标题区：标题固定两行高度，日期在整行卡片里对齐 -->
    <div class="flex flex-1 flex-col px-2.5 pb-2">
      <h3
        class="line-clamp-2 min-h-[2.1rem] text-[12px] font-bold leading-[1.35] text-foreground"
        :title="item.name_cn || item.name"
      >
        {{ item.name_cn || item.name }}
      </h3>
      <p v-if="item.air_date" class="mt-0.5 text-[10.5px] font-medium text-muted-foreground/85 font-num">
        {{ item.air_date }}
      </p>
    </div>
  </article>
</template>

<script setup>
import { ref } from 'vue'
import { CheckmarkCircle, Star } from '@vicons/ionicons5'
import { AcPoster, AcBurst, AcStamp } from '@/components/ac'

defineProps({ item: { type: Object, required: true } })
const emit = defineEmits(['click', 'subscribe'])

const burstRef = ref(null)

// 追番是有分量的确认动作：先炸一下放疗线，再落状态。
// 这里必须留一个"演出节拍"——订阅成功后按钮会被 v-if 移除，
// 立刻抛事件的话放射动画还没画完就被卸载，等于白做。
const SUBSCRIBE_BEAT = 220

function onSubscribe(item) {
  const played = burstRef.value?.play()
  if (!played) {
    // 减少动态偏好：不演出，直接落状态
    emit('subscribe', item)
    return
  }
  setTimeout(() => emit('subscribe', item), SUBSCRIBE_BEAT)
}

// 卡片网格：<640px 两列、sm 三列、md 三列（此时侧栏展开、内容反而更窄）、lg 四列、
// xl 五列、2xl 六列；1440 视口下约 210px/张，圆角比例不再失衡。
// sizes 必须跟着网格走，浏览器才能挑到刚好够用的那一档（DPR1 取 200w、DPR2 取 400w）。
const posterSizes = '(min-width: 1536px) 215px, (min-width: 1280px) 185px, (min-width: 1024px) 170px, (min-width: 768px) 145px, (min-width: 640px) 190px, 50vw'

function formatRating(value) {
  const rating = Number(value)
  return Number.isFinite(rating) ? rating.toFixed(1) : value
}
</script>
