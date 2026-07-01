<template>
  <AcDrawer
    :show="show"
    :title="`第 ${String(episode).padStart(2,'0')} 集 · ${animeTitle}`"
    :width="'480px'"
    @update:show="$emit('update:show', $event)"
  >
    <div class="space-y-5">
      <!-- 已完成的下载 -->
      <section v-if="completedDownloads.length">
        <h3 class="text-sm font-bold mb-2 flex items-center gap-2">
          <CheckmarkCircleOutline class="size-4 text-ac-leaf-dark" />
          已下载 ({{ completedDownloads.length }})
        </h3>
        <div class="space-y-2">
          <div v-for="d in completedDownloads" :key="d.id"
            class="p-3 rounded-2xl border-2 border-ac-sand bg-card space-y-1 text-xs">
            <div class="flex items-center gap-2">
              <AcTag :variant="sourceVariant(d.source)" size="sm">{{ sourceLabel(d.source) }}</AcTag>
              <span class="font-num text-muted-foreground">{{ formatSize(d.total_bytes || d.downloaded_bytes) }}</span>
              <span class="ml-auto text-muted-foreground font-num">{{ formatTime(d.completed_at || d.updated_at) }}</span>
            </div>
            <div class="truncate" :title="d.name">{{ d.name }}</div>
            <div class="flex gap-2 pt-1">
              <button class="text-ac-grass-dark hover:underline font-bold" @click="retryDownload(d)">重新下载</button>
              <button class="text-ac-heart-dark hover:underline font-bold" @click="deleteDownload(d)">删除</button>
            </div>
          </div>
        </div>
      </section>

      <!-- 进行中的下载 -->
      <section v-if="activeDownloads.length">
        <h3 class="text-sm font-bold mb-2 flex items-center gap-2">
          <CloudDownloadOutline class="size-4 text-ac-sky-dark animate-pulse" />
          下载中 ({{ activeDownloads.length }})
        </h3>
        <div class="space-y-2">
          <div v-for="d in activeDownloads" :key="d.id"
            class="p-3 rounded-2xl border-2 border-ac-sand bg-card text-xs space-y-1.5">
            <div class="flex items-center gap-2">
              <AcTag :variant="sourceVariant(d.source)" size="sm">{{ sourceLabel(d.source) }}</AcTag>
              <span class="truncate flex-1" :title="d.name">{{ d.name }}</span>
            </div>
            <div class="h-1.5 rounded-full bg-ac-sand overflow-hidden">
              <div class="h-full bg-ac-grass transition-all" :style="{ width: (d.progress || 0) + '%' }"></div>
            </div>
            <div class="text-muted-foreground font-num">{{ (d.progress || 0).toFixed(1) }}%</div>
          </div>
        </div>
      </section>

      <!-- 失败的下载 -->
      <section v-if="failedDownloads.length">
        <h3 class="text-sm font-bold mb-2 flex items-center gap-2">
          <CloseCircleOutline class="size-4 text-ac-heart-dark" />
          失败 ({{ failedDownloads.length }})
        </h3>
        <div class="space-y-2">
          <div v-for="d in failedDownloads" :key="d.id"
            class="p-3 rounded-2xl border-2 border-ac-sand bg-card text-xs space-y-1">
            <div class="flex items-center gap-2">
              <AcTag variant="heart" size="sm">{{ sourceLabel(d.source) }} 失败</AcTag>
            </div>
            <div class="truncate" :title="d.name">{{ d.name }}</div>
            <div class="flex gap-2 pt-1">
              <button class="text-ac-grass-dark hover:underline font-bold" @click="retryDownload(d)">重试</button>
              <button class="text-ac-heart-dark hover:underline font-bold" @click="deleteDownload(d)">删除记录</button>
            </div>
          </div>
        </div>
      </section>

      <!-- 诊断信息 -->
      <section v-if="diagnosis && diagnosis.sources">
        <h3 class="text-sm font-bold mb-2 flex items-center gap-2">
          <AlertCircleOutline class="size-4 text-ac-sun-dark" />
          源检查诊断
        </h3>
        <p v-if="!hasAnyDownload" class="text-xs text-muted-foreground mb-2">
          系统已尝试所有已启用的源，详情如下：
        </p>
        <div class="space-y-2">
          <div v-for="(info, src) in diagnosis.sources" :key="src"
            class="p-3 rounded-2xl border-2 border-ac-sand bg-ac-cream/40 text-xs space-y-1">
            <div class="flex items-center gap-2">
              <AcTag :variant="sourceVariant(src)" size="sm">{{ sourceLabel(src) }}</AcTag>
              <span class="text-muted-foreground font-num">{{ info.checked_at }}</span>
            </div>
            <div>
              <span class="text-muted-foreground">返回：</span>
              <span class="font-num">{{ info.result_count }} 条</span>
              <span v-if="info.ranked_out > 0" class="text-muted-foreground">
                （被过滤 {{ info.ranked_out }}）
              </span>
            </div>
            <div class="text-muted-foreground">{{ info.reason }}</div>
            <div v-if="info.best_title" class="text-muted-foreground truncate" :title="info.best_title">
              Best: {{ info.best_title }} (score={{ info.best_score.toFixed(1) }})
            </div>
          </div>
        </div>
        <div v-if="!hasAnyDownload" class="mt-3 flex gap-2 flex-wrap">
          <AcButton size="sm" variant="primary" @click="$emit('manual-search', episode)">🔍 查看所有候选种子</AcButton>
          <router-link to="/settings" class="text-xs text-ac-grass-dark hover:underline inline-flex items-center font-bold">
            调整下载偏好 →
          </router-link>
        </div>
      </section>

      <!-- 空状态 -->
      <div v-if="!hasAnyDownload && !diagnosis" class="text-center py-8 text-sm text-muted-foreground space-y-3">
        <div>等待下次检查... 🌱</div>
        <div class="flex gap-2 justify-center">
          <AcButton size="sm" variant="primary" @click="$emit('manual-search', episode)">🔍 立即手动搜索</AcButton>
          <AcButton size="sm" variant="outline" @click="$emit('refresh')">手动刷新</AcButton>
        </div>
      </div>
    </div>
  </AcDrawer>
</template>

<script setup>
import { computed } from 'vue'
import {
  CheckmarkCircleOutline, CloudDownloadOutline, CloseCircleOutline, AlertCircleOutline,
} from '@vicons/ionicons5'
import { post, del } from '@/utils/api'
import { useToast } from '@/composables/useToast'
import { AcDrawer, AcButton, AcTag } from '@/components/ac'

const props = defineProps({
  show: Boolean,
  animeId: Number,
  animeTitle: { type: String, default: '' },
  episode: { type: Number, default: 0 },
  downloads: { type: Array, default: () => [] },
  diagnosis: { type: Object, default: null },
  epMeta: { type: Object, default: null },
})

const emit = defineEmits(['update:show', 'refresh', 'manual-search'])
const toast = useToast()

const completedDownloads = computed(() => props.downloads.filter(d => d.status === 'completed'))
const activeDownloads = computed(() => props.downloads.filter(d => d.status === 'downloading' || d.status === 'pending'))
const failedDownloads = computed(() => props.downloads.filter(d => d.status === 'failed'))
const hasAnyDownload = computed(() => props.downloads.length > 0)

function sourceLabel(src) {
  return { bt: 'BT', stream: '流媒体', bangumi: '流媒体', rss: 'RSS', manual: '手动' }[src] || src
}

function sourceVariant(src) {
  switch (src) {
    case 'bt': return 'wood'
    case 'stream':
    case 'bangumi': return 'grass'
    case 'rss': return 'sun'
    default: return 'default'
  }
}

function formatSize(bytes) {
  if (!bytes) return '—'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}

function formatTime(s) {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const diff = (Date.now() - d.getTime()) / 1000
  if (diff < 60) return '刚刚'
  if (diff < 3600) return Math.floor(diff / 60) + ' 分钟前'
  if (diff < 86400) return Math.floor(diff / 3600) + ' 小时前'
  return d.toLocaleDateString('zh-CN')
}

async function retryDownload(d) {
  try {
    await post(`/downloads/${d.id}/retry`)
    toast.success('已重新加入队列')
    emit('refresh')
  } catch (e) {
    toast.error(e.message || '操作失败')
  }
}

async function deleteDownload(d) {
  try {
    await del(`/downloads/${d.id}`)
    toast.success('已删除')
    emit('refresh')
  } catch (e) {
    toast.error(e.message || '删除失败')
  }
}
</script>
