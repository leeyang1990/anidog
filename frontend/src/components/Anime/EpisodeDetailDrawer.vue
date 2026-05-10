<template>
  <n-drawer :show="show" @update:show="$emit('update:show', $event)" :width="480" placement="right">
    <n-drawer-content :title="`第 ${String(episode).padStart(2,'0')} 集 · ${animeTitle}`" closable>
      <div class="space-y-5">
        <!-- 已完成的下载 -->
        <section v-if="completedDownloads.length">
          <h3 class="text-sm font-semibold mb-2 flex items-center gap-2">
            <n-icon size="14" class="text-emerald-600"><CheckmarkCircleOutline /></n-icon>
            已下载 ({{ completedDownloads.length }})
          </h3>
          <div class="space-y-2">
            <div v-for="d in completedDownloads" :key="d.id"
              class="p-3 rounded-md border bg-card space-y-1 text-xs">
              <div class="flex items-center gap-2">
                <span class="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold"
                  :class="sourceBadgeClass(d.source)">
                  {{ sourceLabel(d.source) }}
                </span>
                <span class="font-mono text-muted-foreground">{{ formatSize(d.total_bytes || d.downloaded_bytes) }}</span>
                <span class="ml-auto text-muted-foreground">{{ formatTime(d.completed_at || d.updated_at) }}</span>
              </div>
              <div class="truncate" :title="d.name">{{ d.name }}</div>
              <div class="flex gap-2 pt-1">
                <button class="text-primary hover:underline" @click="retryDownload(d)">重新下载</button>
                <button class="text-destructive hover:underline" @click="deleteDownload(d)">删除</button>
              </div>
            </div>
          </div>
        </section>

        <!-- 进行中的下载 -->
        <section v-if="activeDownloads.length">
          <h3 class="text-sm font-semibold mb-2 flex items-center gap-2">
            <n-icon size="14" class="text-primary animate-pulse"><CloudDownloadOutline /></n-icon>
            下载中 ({{ activeDownloads.length }})
          </h3>
          <div class="space-y-2">
            <div v-for="d in activeDownloads" :key="d.id"
              class="p-3 rounded-md border bg-card text-xs space-y-1.5">
              <div class="flex items-center gap-2">
                <span class="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold"
                  :class="sourceBadgeClass(d.source)">
                  {{ sourceLabel(d.source) }}
                </span>
                <span class="truncate flex-1" :title="d.name">{{ d.name }}</span>
              </div>
              <div class="h-1 rounded-full bg-muted overflow-hidden">
                <div class="h-full bg-primary transition-all" :style="{ width: (d.progress || 0) + '%' }"></div>
              </div>
              <div class="text-muted-foreground">{{ (d.progress || 0).toFixed(1) }}%</div>
            </div>
          </div>
        </section>

        <!-- 失败的下载 -->
        <section v-if="failedDownloads.length">
          <h3 class="text-sm font-semibold mb-2 flex items-center gap-2">
            <n-icon size="14" class="text-destructive"><CloseCircleOutline /></n-icon>
            失败 ({{ failedDownloads.length }})
          </h3>
          <div class="space-y-2">
            <div v-for="d in failedDownloads" :key="d.id"
              class="p-3 rounded-md border bg-card text-xs space-y-1">
              <div class="flex items-center gap-2">
                <span class="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold bg-destructive/20 text-destructive">
                  {{ sourceLabel(d.source) }} 失败
                </span>
              </div>
              <div class="truncate" :title="d.name">{{ d.name }}</div>
              <div class="flex gap-2 pt-1">
                <button class="text-primary hover:underline" @click="retryDownload(d)">重试</button>
                <button class="text-destructive hover:underline" @click="deleteDownload(d)">删除记录</button>
              </div>
            </div>
          </div>
        </section>

        <!-- 诊断信息 -->
        <section v-if="diagnosis && diagnosis.sources">
          <h3 class="text-sm font-semibold mb-2 flex items-center gap-2">
            <n-icon size="14" class="text-amber-500"><AlertCircleOutline /></n-icon>
            源检查诊断
          </h3>
          <p v-if="!hasAnyDownload" class="text-xs text-muted-foreground mb-2">
            系统已尝试所有已启用的源，详情如下：
          </p>
          <div class="space-y-2">
            <div v-for="(info, src) in diagnosis.sources" :key="src"
              class="p-3 rounded-md border bg-muted/30 text-xs space-y-1">
              <div class="flex items-center gap-2">
                <span class="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold"
                  :class="sourceBadgeClass(src)">
                  {{ sourceLabel(src) }}
                </span>
                <span class="text-muted-foreground">{{ info.checked_at }}</span>
              </div>
              <div>
                <span class="text-muted-foreground">返回：</span>
                <span class="font-mono">{{ info.result_count }} 条</span>
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
            <button @click="$emit('manual-search', episode)"
              class="text-xs inline-flex items-center gap-1 h-7 px-3 rounded-md bg-primary text-primary-foreground hover:bg-primary/90">
              🔍 查看所有候选种子
            </button>
            <router-link to="/settings" class="text-xs text-primary hover:underline inline-flex items-center">
              调整下载偏好 →
            </router-link>
          </div>
        </section>

        <!-- 空状态 -->
        <div v-if="!hasAnyDownload && !diagnosis" class="text-center py-8 text-sm text-muted-foreground space-y-3">
          <div>等待下次检查...</div>
          <div class="flex gap-2 justify-center">
            <button @click="$emit('manual-search', episode)"
              class="text-xs inline-flex items-center gap-1 h-7 px-3 rounded-md bg-primary text-primary-foreground hover:bg-primary/90">
              🔍 立即手动搜索
            </button>
            <button @click="$emit('refresh')" class="text-xs inline-flex items-center gap-1 h-7 px-3 rounded-md border border-input bg-background hover:bg-accent">
              手动刷新
            </button>
          </div>
        </div>
      </div>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup>
import { computed } from 'vue'
import { NDrawer, NDrawerContent, NIcon, useMessage } from 'naive-ui'
import {
  CheckmarkCircleOutline, CloudDownloadOutline, CloseCircleOutline, AlertCircleOutline,
} from '@vicons/ionicons5'
import { post, del } from '@/utils/api'

const props = defineProps({
  show: Boolean,
  animeId: Number,
  animeTitle: { type: String, default: '' },
  episode: { type: Number, default: 0 },
  downloads: { type: Array, default: () => [] },
  diagnosis: { type: Object, default: null },
})

const emit = defineEmits(['update:show', 'refresh', 'manual-search'])
const message = useMessage()

const completedDownloads = computed(() => props.downloads.filter(d => d.status === 'completed'))
const activeDownloads = computed(() => props.downloads.filter(d => d.status === 'downloading' || d.status === 'pending'))
const failedDownloads = computed(() => props.downloads.filter(d => d.status === 'failed'))
const hasAnyDownload = computed(() => props.downloads.length > 0)

function sourceLabel(src) {
  return { bt: 'BT', stream: '流媒体', bangumi: '流媒体', rss: 'RSS', manual: '手动' }[src] || src
}

function sourceBadgeClass(src) {
  switch (src) {
    case 'bt': return 'bg-violet-500/20 text-violet-700 dark:text-violet-400'
    case 'stream':
    case 'bangumi': return 'bg-primary/20 text-primary'
    case 'rss': return 'bg-amber-500/20 text-amber-700 dark:text-amber-400'
    default: return 'bg-muted text-muted-foreground'
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
    message.success('已重新加入队列')
    emit('refresh')
  } catch (e) {
    message.error(e.message || '操作失败')
  }
}

async function deleteDownload(d) {
  try {
    await del(`/downloads/${d.id}`)
    message.success('已删除')
    emit('refresh')
  } catch (e) {
    message.error(e.message || '删除失败')
  }
}
</script>
