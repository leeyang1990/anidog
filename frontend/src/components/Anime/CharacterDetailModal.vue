<template>
  <n-modal :show="show" @update:show="$emit('update:show', $event)" preset="card"
    style="width: 760px; max-width: 95vw"
    :bordered="false" :closable="true" :mask-closable="true">
    <div v-if="loading" class="py-16 text-center">
      <n-spin size="large" />
    </div>

    <div v-else-if="detail" class="grid grid-cols-1 md:grid-cols-[300px_1fr] gap-6">
      <!-- 左侧：大图（保持原比例，不裁切） -->
      <div>
        <div class="w-full rounded-lg overflow-hidden bg-muted/40 shadow-md flex items-center justify-center">
          <img v-if="coverImage" :src="coverImage" :alt="detail.name"
            class="w-full h-auto object-contain"
            style="max-height: 520px"
            @error="$event.target.style.display='none'" />
          <div v-else class="w-full aspect-[3/4] flex items-center justify-center">
            <n-icon size="80" class="text-muted-foreground"><PersonCircleOutline /></n-icon>
          </div>
        </div>
        <!-- 基本标签 -->
        <div class="flex flex-wrap gap-1.5 mt-3 justify-center">
          <span v-if="character?.relation"
            class="px-2 py-0.5 rounded-full text-xs font-medium bg-primary/10 text-primary">
            {{ character.relation }}
          </span>
          <span v-if="detail.gender"
            class="px-2 py-0.5 rounded-full text-xs font-medium bg-muted text-foreground">
            {{ genderLabel(detail.gender) }}
          </span>
        </div>
      </div>

      <!-- 右侧：详情 -->
      <div class="min-w-0 space-y-4">
        <!-- 名称 -->
        <div>
          <h2 class="text-2xl font-bold tracking-tight">{{ detail.name_cn || detail.name }}</h2>
          <p v-if="detail.name_cn && detail.name !== detail.name_cn"
            class="text-sm text-muted-foreground mt-0.5">{{ detail.name }}</p>
        </div>

        <!-- CV -->
        <div v-if="character?.actor" class="flex items-center gap-3 bg-muted/40 rounded-md px-3 py-2">
          <span class="text-xs text-muted-foreground">声优</span>
          <span class="text-sm font-medium">{{ character.actor }}</span>
        </div>

        <!-- 简介 -->
        <div v-if="detail.summary" class="space-y-1.5">
          <div class="text-xs text-muted-foreground">简介</div>
          <p class="text-sm leading-relaxed whitespace-pre-line text-foreground/90">
            {{ detail.summary }}
          </p>
        </div>

        <!-- infobox 详细资料 -->
        <div v-if="infoboxItems.length" class="space-y-1.5">
          <div class="text-xs text-muted-foreground">资料</div>
          <dl class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 text-sm">
            <template v-for="(kv, i) in infoboxItems" :key="i">
              <dt class="text-muted-foreground truncate">{{ kv.key }}</dt>
              <dd class="text-foreground break-words">{{ formatValue(kv.value) }}</dd>
            </template>
          </dl>
        </div>

        <!-- Bangumi 链接 -->
        <div v-if="detail.id" class="pt-2 border-t">
          <a :href="`https://bgm.tv/character/${detail.id}`" target="_blank"
            class="text-xs text-primary hover:underline inline-flex items-center gap-1">
            <n-icon size="12"><OpenOutline /></n-icon>
            在 Bangumi 查看完整资料
          </a>
        </div>
      </div>
    </div>

    <div v-else class="py-16 text-center text-sm text-muted-foreground">
      <p>角色详情获取失败</p>
    </div>
  </n-modal>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { NModal, NIcon, NSpin } from 'naive-ui'
import { PersonCircleOutline, OpenOutline } from '@vicons/ionicons5'
import { toHighResImage } from '@/utils/image'
import { get } from '@/utils/api'

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

// 过滤并规范 infobox：去掉图片/重复项，保留"性别/生日/血型/身高"等
const PRIORITY_KEYS = ['性别', '生日', '血型', '身高', '体重', '三围', '年龄', '星座', '所属']
const HIDE_KEYS = new Set(['简体中文名', '第二名', '昵称', '别名'])
const infoboxItems = computed(() => {
  if (!detail.value?.infobox) return []
  const list = detail.value.infobox.filter(kv => !HIDE_KEYS.has(kv.key))
  // 按优先级排序
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
  } catch (e) {
    detail.value = null
  } finally {
    loading.value = false
  }
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
