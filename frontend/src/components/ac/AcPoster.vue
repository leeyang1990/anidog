<template>
  <div class="ac-poster absolute inset-0 overflow-hidden">
    <!-- 骨架微光：图片解码完成前占住版面，避免大白块 + 大图同时闪现 -->
    <div
      v-if="showSkeleton"
      class="ac-poster-shimmer absolute inset-0 bg-ac-sand/50"
      aria-hidden="true"
    />

    <div
      v-if="showFallback"
      class="absolute inset-0 flex items-center justify-center text-ac-wood-dark"
      aria-hidden="true"
    >
      <slot name="fallback"><FilmOutline class="size-6" /></slot>
    </div>

    <img
      v-if="showImage"
      :src="primarySrc"
      :srcset="srcset || undefined"
      :sizes="srcset ? sizes : undefined"
      :alt="alt"
      :loading="eager ? 'eager' : 'lazy'"
      :fetchpriority="eager ? 'high' : 'auto'"
      decoding="async"
      class="ac-poster-img absolute inset-0 h-full w-full object-cover"
      :class="zoom ? 'group-hover:scale-105' : ''"
      :data-loaded="status === 'loaded' ? 'true' : 'false'"
      @load="status = 'loaded'"
      @error="onError"
    />
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { FilmOutline } from '@vicons/ionicons5'
import { BANGUMI_IMAGE_WIDTHS, toResizedImage } from '@/utils/image'

// 海报统一入口：按容器实际宽度取图 + 加载占位 + 淡入。
//
// Bangumi 的 resize 接口只认白名单宽度（100/200/400/600/800），传 300 之类会直接 400，
// 所以 widths 只能从这几个值里取；非 Bangumi 图源会退化成单张原图。
const props = defineProps({
  src: { type: String, default: '' },
  alt: { type: String, default: '' },
  // 候选宽度：交给浏览器按 sizes + DPR 挑最小够用的那张
  widths: {
    type: Array,
    default: () => [200, 400, 600],
    validator: (value) => value.every((w) => BANGUMI_IMAGE_WIDTHS.includes(w)),
  },
  sizes: { type: String, default: '(min-width: 1024px) 160px, (min-width: 640px) 150px, 33vw' },
  // 首屏图可关掉懒加载
  eager: { type: Boolean, default: false },
  // 卡片 hover 时海报放大
  zoom: { type: Boolean, default: true },
})

const status = ref('loading')
const useSrcset = ref(true)
const retried = ref(false)

const variants = computed(() => {
  if (!props.src) return []
  const urls = props.widths.map((w) => toResizedImage(props.src, w))
  const unique = [...new Set(urls)]
  // 只认得出多档尺寸的图源才用 srcset，否则会和 src 完全重复
  return unique.length > 1 ? unique : []
})

const srcset = computed(() => {
  if (!useSrcset.value || !variants.value.length) return ''
  return variants.value.map((url) => `${url} ${widthOf(url)}w`).join(', ')
})

const primarySrc = computed(() => props.src)

const showImage = computed(() => Boolean(props.src) && status.value !== 'error')
const showSkeleton = computed(() => Boolean(props.src) && status.value === 'loading')
const showFallback = computed(() => !props.src || status.value === 'error')

function widthOf(url) {
  const matched = /\/r\/(\d+)\//.exec(url)
  return matched ? matched[1] : 800
}

function onError() {
  // 某档尺寸挂掉时退回单张原图再试一次，避免整张海报开天窗
  if (useSrcset.value && !retried.value) {
    retried.value = true
    useSrcset.value = false
    return
  }
  status.value = 'error'
}

watch(() => props.src, () => {
  status.value = 'loading'
  useSrcset.value = true
  retried.value = false
})
</script>
