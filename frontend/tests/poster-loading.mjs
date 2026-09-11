import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { toResizedImage } from '../src/utils/image.js'

const read = (path) => readFile(new URL(path, import.meta.url), 'utf8')

// 日历一屏能塞 30+ 张海报，尺寸取错会同时拖慢网络与解码，所以这里把约束钉死。

// 1. Bangumi 的 resize 接口只认白名单宽度，传别的值直接 400（r/300 实测不可用）。
const BANGUMI_WIDTHS = [100, 200, 400, 600, 800]
const poster = await read('../src/components/ac/AcPoster.vue')

const declared = /default:\s*\(\)\s*=>\s*\[([^\]]+)\]/.exec(poster)
assert.ok(declared, 'AcPoster 必须声明候选宽度')
for (const width of declared[1].split(',').map((v) => Number(v.trim()))) {
  assert.ok(BANGUMI_WIDTHS.includes(width), `候选宽度 ${width} 不在 Bangumi 白名单内，会返回 400`)
}
for (const width of BANGUMI_WIDTHS) {
  assert.ok(
    toResizedImage('https://lain.bgm.tv/pic/cover/l/ce/e2/456080_C4q4C.jpg', width)
      .includes(`/r/${width}/`),
    `toResizedImage 应能生成 r/${width}/`,
  )
}
// 非 Bangumi 图源不能被塞进 /r/<w>/（那会把地址改坏），只能退回单张原图
const foreign = 'https://example.com/covers/1.jpg'
assert.equal(toResizedImage(foreign, 200), foreign)
assert.equal(toResizedImage(foreign, 600), foreign)

// 2. 占位与淡入：骨架在解码完成前顶住版面，图片加载完成后才显示
assert.match(poster, /class="ac-poster absolute inset-0 overflow-hidden"/)
assert.match(poster, /ac-poster-shimmer absolute inset-0 bg-ac-sand\/50/)
assert.match(poster, /:data-loaded="status === 'loaded' \? 'true' : 'false'"/)
assert.match(poster, /:class="zoom \? 'group-hover:scale-105' : ''"/)
assert.match(poster, /decoding="async"/)
assert.match(poster, /:loading="eager \? 'eager' : 'lazy'"/)
assert.match(poster, /:srcset="srcset \|\| undefined"/)
assert.match(poster, /:sizes="srcset \? sizes : undefined"/)
// 某一档尺寸挂掉时先退回原图，不能直接开天窗
assert.match(poster, /if \(useSrcset\.value && !retried\.value\)/)
// 换图（翻页 / 切日期）必须重置状态，否则新海报会沿用上一张的已加载状态
assert.match(poster, /watch\(\(\) => props\.src/)
// 复用性：图源为空、加载失败都要有兜底，不能留白块
assert.match(poster, /const showFallback = computed/)
assert.match(poster, /<slot name="fallback">/)

// 3. 样式：扫光走 transform（只合成，不重排），并且尊重"减少动态"偏好
const css = await read('../src/assets/tailwind.css')
assert.match(css, /\.ac-poster-shimmer::after/)
assert.match(css, /animation: ac-poster-shimmer 1\.4s ease-in-out infinite/)
assert.match(css, /@keyframes ac-poster-shimmer/)
assert.match(css, /transform: translateX\(-100%\)/)
assert.match(css, /@keyframes ac-poster-shimmer \{\s*100% \{\s*transform: translateX\(100%\);/)
assert.match(css, /\.ac-poster-img\[data-loaded='true'\]/)
assert.match(css, /@media \(prefers-reduced-motion: reduce\) \{[\s\S]*?\.ac-poster-shimmer::after \{\s*animation: none;/)
// materials.css 负责 data-reduce-motion（桌面端原生"减少动态"）的全局兜底
const materials = await read('../src/assets/materials.css')
assert.match(materials, /data-reduce-motion="true"[\s\S]*?animation: none !important/)

// 4. 日历 / 番剧库 / 追番列表都必须走 AcPoster，不能再各自手写 <img> 下大图
const animeCard = await read('../src/views/Anime/AnimeCard.vue')
const naiveCard = await read('../src/components/Anime/NaiveAnimeCard.vue')
for (const [name, source] of [['AnimeCard', animeCard], ['NaiveAnimeCard', naiveCard]]) {
  assert.match(source, /import \{ AcPoster \} from '@\/components\/ac'/, `${name} 应使用 AcPoster`)
  assert.match(source, /<AcPoster/, `${name} 应渲染 AcPoster`)
  assert.match(source, /:sizes="posterSizes"/, `${name} 应传入与实际格子匹配的 sizes`)
  assert.doesNotMatch(source, /toResizedImage\(/, `${name} 不应再自行拼图片地址`)
  assert.doesNotMatch(source, /<img\s/, `${name} 不应再手写海报 <img>`)
}
// 日历与番剧库共用同一套 3/4/5/6/8 列网格，取图宽度要跟着列数收敛
const calendar = await read('../src/views/Calendar/index.vue')
assert.match(calendar, /xl:grid-cols-8/)
assert.match(animeCard, /33vw/)

console.log('poster loading tests passed')
