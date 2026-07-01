<template>
  <AcModal :show="show" @update:show="$emit('update:show', $event)" max-width="800px">
    <div v-if="loading" class="py-16 flex justify-center"><AcSpinner :size="48" /></div>

    <div v-else-if="detail" class="grid grid-cols-1 md:grid-cols-[300px_1fr] gap-6">
      <!-- 左侧：大图 -->
      <div>
        <div class="w-full rounded-2xl overflow-hidden bg-ac-sand/40 border-2 border-ac-sand shadow-md flex items-center justify-center">
          <img v-if="coverImage" :src="coverImage" :alt="detail.name"
            class="w-full h-auto object-contain"
            style="max-height: 520px"
            @error="$event.target.style.display='none'" />
          <div v-else class="w-full aspect-[3/4] flex items-center justify-center">
            <PersonCircleOutline class="size-20 text-ac-wood-dark" />
          </div>
        </div>
        <div class="flex flex-wrap gap-1.5 mt-3 justify-center">
          <AcTag v-if="character?.relation" variant="grass">{{ character.relation }}</AcTag>
          <AcTag v-if="detail.gender" variant="wood">{{ genderLabel(detail.gender) }}</AcTag>
        </div>
      </div>

      <!-- 右侧：详情 -->
      <div class="min-w-0 space-y-4">
        <div>
          <h2 class="text-2xl font-bold tracking-tight text-foreground">{{ detail.name_cn || detail.name }}</h2>
          <p v-if="detail.name_cn && detail.name !== detail.name_cn"
            class="text-sm text-muted-foreground mt-0.5 font-num">{{ detail.name }}</p>
        </div>

        <div v-if="character?.actor" class="flex items-center gap-3 bg-ac-sand/40 rounded-2xl px-4 py-2 border-2 border-ac-sand">
          <span class="text-xs text-muted-foreground font-bold">🎙️ 声优</span>
          <span class="text-sm font-bold">{{ character.actor }}</span>
        </div>

        <div v-if="detail.summary" class="space-y-1.5">
          <div class="text-xs text-muted-foreground font-bold">📝 简介</div>
          <p class="text-sm leading-relaxed whitespace-pre-line text-foreground/90">{{ detail.summary }}</p>
        </div>

        <div v-if="infoboxItems.length" class="space-y-1.5">
          <div class="text-xs text-muted-foreground font-bold">📋 资料</div>
          <dl class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 text-sm">
            <template v-for="(kv, i) in infoboxItems" :key="i">
              <dt class="text-muted-foreground truncate font-bold">{{ kv.key }}</dt>
              <dd class="text-foreground break-words">{{ formatValue(kv.value) }}</dd>
            </template>
          </dl>
        </div>

        <div v-if="detail.id" class="pt-2 border-t-2 border-dashed border-ac-sand">
          <a :href="`https://bgm.tv/character/${detail.id}`" target="_blank"
            class="text-xs text-ac-grass-dark hover:underline inline-flex items-center gap-1 font-bold">
            <OpenOutline class="size-3" />
            在 Bangumi 查看完整资料
          </a>
        </div>
      </div>
    </div>

    <div v-else class="py-16 text-center text-sm text-muted-foreground">
      <p>角色详情获取失败 😿</p>
    </div>
  </AcModal>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { PersonCircleOutline, OpenOutline } from '@vicons/ionicons5'
import { toHighResImage } from '@/utils/image'
import { get } from '@/utils/api'
import { AcModal, AcSpinner, AcTag } from '@/components/ac'

const props = defineProps({
  show: { type: Boolean, default: false },
  character: { type: Object, default: () => ({}) }
})
defineEmits(['update:show'])

const detail = ref(null)
const loading = ref(false)

const coverImage = computed(() => {
  if (!detail.value) return ''
  return toHighResImage(
    detail.value.images?.large ||
    detail.value.images?.medium ||
    detail.value.images?.small ||
    props.character?.image ||
    ''
  )
})

const PRIORITY_KEYS = ['性别', '生日', '血型', '身高', '体重', '三围', '年龄', '星座', '所属']
const HIDE_KEYS = new Set(['简体中文名', '第二名', '昵称', '别名'])
const infoboxItems = computed(() => {
  if (!detail.value?.infobox) return []
  const list = detail.value.infobox.filter(kv => !HIDE_KEYS.has(kv.key))
  list.sort((a, b) => {
    const ia = PRIORITY_KEYS.indexOf(a.key)
    const ib = PRIORITY_KEYS.indexOf(b.key)
    if (ia === -1 && ib === -1) return 0
    if (ia === -1) return 1
    if (ib === -1) return -1
    return ia - ib
  })
  return list.slice(0, 10)
})

function formatValue(v) {
  if (Array.isArray(v)) return v.map(x => x.v || x).join(', ')
  if (typeof v === 'object' && v !== null) return v.v || JSON.stringify(v)
  return String(v ?? '')
}

function genderLabel(g) {
  if (g === 'male' || g === '男') return '♂ 男'
  if (g === 'female' || g === '女') return '♀ 女'
  return g
}

async function fetchDetail(id) {
  loading.value = true
  detail.value = null
  try {
    const data = await get(`/bangumi/characters/${id}`)
    detail.value = data
  } catch (e) { detail.value = null }
  finally { loading.value = false }
}

watch(
  () => [props.show, props.character?.id],
  ([show, id]) => {
    if (show && id) fetchDetail(id)
    if (!show) detail.value = null
  },
  { immediate: true }
)
</script>
