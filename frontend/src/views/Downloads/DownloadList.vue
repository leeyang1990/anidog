<template>
  <div class="space-y-4">
    <!-- 顶部状态栏 -->
    <div class="flex items-center justify-between gap-4 flex-wrap">
      <div class="flex items-center gap-2 text-sm">
        <span class="text-muted-foreground">共 <span class="text-foreground font-bold font-num">{{ tasks.length }}</span> 个</span>
        <span class="text-muted-foreground">·</span>
        <span class="text-ac-grass-dark inline-flex items-center gap-1 font-num font-bold">
          <ArrowDownOutline class="size-3.5" />{{ formatSpeed(globalDownloadSpeed) }}
        </span>
        <span v-if="globalUploadSpeed > 0" class="text-muted-foreground">·</span>
        <span v-if="globalUploadSpeed > 0" class="text-ac-leaf-dark inline-flex items-center gap-1 font-num font-bold">
          <ArrowUpOutline class="size-3.5" />{{ formatSpeed(globalUploadSpeed) }}
        </span>
      </div>
      <div class="flex items-center gap-2">
        <AcButton variant="outline" size="sm" :loading="checkingAllUpdates" @click="handleCheckAllUpdates">
          <template #icon><RefreshOutline class="size-3.5" /></template>
          {{ checkingAllUpdates ? '检查中...' : '检查追番更新' }}
        </AcButton>
        <AcButton variant="primary" size="sm" @click="showAddModal = true">
          <template #icon><AddOutline class="size-3.5" /></template>
          添加
        </AcButton>
      </div>
    </div>

    <!-- 过滤器 + 搜索 + 批量操作 -->
    <div class="flex items-center gap-2 flex-wrap">
      <div class="flex gap-1 bg-ac-sand p-1 rounded-2xl">
        <button v-for="f in filters" :key="f.key"
          class="h-7 px-3 rounded-xl text-xs font-bold transition-all"
          :class="activeFilter === f.key ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
          @click="activeFilter = f.key">
          {{ f.label }}
          <span v-if="f.count > 0" class="ml-1 opacity-70 font-num">{{ f.count }}</span>
        </button>
      </div>

      <div class="flex-1 min-w-[200px] max-w-xs">
        <AcInput v-model="searchKeyword" placeholder="搜索..." size="sm">
          <template #prefix><SearchOutline class="size-3.5" /></template>
        </AcInput>
      </div>

      <div v-if="selectedIds.size > 0" class="flex items-center gap-2 ml-auto">
        <span class="text-xs text-muted-foreground">已选 <span class="font-num font-bold text-foreground">{{ selectedIds.size }}</span> 个</span>
        <AcButton variant="ghost" size="sm" :disabled="!canBatchPause" @click="batchAction('pause')">
          <template #icon><PauseOutline class="size-3" /></template>暂停
        </AcButton>
        <AcButton variant="ghost" size="sm" :disabled="!canBatchResume" @click="batchAction('resume')">
          <template #icon><PlayOutline class="size-3" /></template>继续
        </AcButton>
        <AcButton variant="ghost" size="sm" class="!text-ac-heart-dark hover:!bg-ac-heart/10" @click="batchDelete">
          <template #icon><TrashOutline class="size-3" /></template>删除
        </AcButton>
      </div>

      <div v-else class="flex items-center gap-2 ml-auto">
        <AcButton variant="ghost" size="sm" :disabled="!hasActive" @click="pauseAll">
          <template #icon><PauseOutline class="size-3" /></template>全部暂停
        </AcButton>
        <AcButton variant="ghost" size="sm" :disabled="!hasPaused" @click="resumeAll">
          <template #icon><PlayOutline class="size-3" /></template>全部继续
        </AcButton>
      </div>
    </div>

    <!-- 任务列表 -->
    <AcCard padding="none" rounded="2xl">
      <AcEmpty v-if="!filteredTasks.length" :title="searchKeyword ? '未找到匹配的任务' : '暂无下载任务'" class="py-12" />
      <div v-else class="divide-y-2 divide-dashed divide-ac-sand">
        <div class="flex items-center gap-3 px-4 py-2 bg-ac-sand/40 text-xs text-muted-foreground font-bold rounded-t-3xl">
          <div class="w-4 shrink-0">
            <input type="checkbox" class="accent-ac-grass cursor-pointer size-4" :checked="allSelected" :indeterminate.prop="someSelected" @change="toggleSelectAll" />
          </div>
          <div class="w-5 shrink-0" />
          <div class="flex-1">名称</div>
          <div class="w-16 shrink-0 text-center">源</div>
          <div class="w-28 shrink-0 text-right">大小</div>
          <div class="w-24 shrink-0 text-right">速度</div>
          <div class="w-32 shrink-0">进度</div>
          <div class="w-32 shrink-0 text-right">时间</div>
          <div class="w-20 shrink-0 text-right">操作</div>
        </div>

        <div v-for="task in filteredTasks" :key="task.id"
          class="flex items-center gap-3 px-4 py-2.5 hover:bg-ac-cream/60 transition-colors"
          :class="selectedIds.has(task.id) ? 'bg-ac-grass-light/20' : ''">
          <div class="w-4 shrink-0">
            <input type="checkbox" class="accent-ac-grass cursor-pointer size-4" :checked="selectedIds.has(task.id)" @change="toggleSelect(task.id)" />
          </div>

          <div class="w-5 shrink-0 flex items-center justify-center">
            <component :is="statusIcon(task.status)" class="size-4" :class="statusIconClass(task.status)" />
          </div>

          <div class="flex-1 min-w-0">
            <div class="text-sm truncate font-bold" :title="task.name">{{ task.name }}</div>
            <div v-if="task.status === 'failed' && task.error_message" class="text-xs text-ac-heart-dark truncate mt-0.5">
              {{ task.error_message }}
            </div>
            <div v-else-if="task.episode_number" class="text-xs text-muted-foreground mt-0.5">
              第 <span class="font-num">{{ String(task.episode_number).padStart(2,'0') }}</span> 集
            </div>
          </div>

          <div class="w-16 shrink-0 flex items-center justify-center">
            <AcTag :variant="sourceTagVariant(task.source)">{{ sourceLabel(task.source) }}</AcTag>
          </div>

          <div class="w-28 shrink-0 text-right text-xs text-muted-foreground font-num">{{ formatTaskSize(task) }}</div>

          <div class="w-24 shrink-0 text-right text-xs font-num">
            <span v-if="task.status === 'downloading'" class="text-ac-grass-dark font-bold">{{ formatSpeed(task.download_speed || 0) }}</span>
            <span v-else class="text-muted-foreground">—</span>
          </div>

          <div class="w-32 shrink-0">
            <div v-if="task.status === 'downloading' || task.status === 'paused'">
              <div class="flex items-center justify-between text-xs mb-0.5 font-num">
                <span class="text-muted-foreground">{{ (task.progress || 0).toFixed(1) }}%</span>
                <span v-if="task.eta && task.status === 'downloading'" class="text-muted-foreground">{{ task.eta }}</span>
              </div>
              <AcProgress :value="task.progress || 0" :height="4" :variant="task.status === 'paused' ? 'wood' : 'grass'" />
            </div>
            <div v-else-if="task.status === 'completed'">
              <AcProgress :value="100" :height="4" variant="leaf" />
            </div>
            <div v-else-if="task.status === 'queued'" class="text-xs text-muted-foreground">队列中</div>
            <div v-else class="text-xs text-muted-foreground">—</div>
          </div>

          <div class="w-32 shrink-0 text-right text-xs text-muted-foreground font-num">
            <div v-if="task.status === 'completed' && task.completed_at">
              <span :title="'完成于 ' + formatAbsoluteTime(task.completed_at)">✓ {{ formatRelativeTime(task.completed_at) }}</span>
            </div>
            <div v-else-if="task.created_at">
              <span :title="'创建于 ' + formatAbsoluteTime(task.created_at)">{{ formatRelativeTime(task.created_at) }}</span>
            </div>
            <span v-else>—</span>
          </div>

          <div class="w-20 shrink-0 flex items-center justify-end gap-1">
            <button v-if="task.status === 'downloading'" class="p-1.5 rounded-lg hover:bg-ac-sand transition-colors" @click="togglePause(task)" title="暂停">
              <PauseOutline class="size-3.5" />
            </button>
            <button v-else-if="task.status === 'paused'" class="p-1.5 rounded-lg hover:bg-ac-sand transition-colors" @click="togglePause(task)" title="继续">
              <PlayOutline class="size-3.5" />
            </button>
            <button v-else-if="task.status === 'failed'" class="p-1.5 rounded-lg hover:bg-ac-sand transition-colors" @click="retryTask(task)" title="重试">
              <ReloadOutline class="size-3.5" />
            </button>
            <button v-else-if="task.status === 'completed'" class="p-1.5 rounded-lg hover:bg-ac-sand transition-colors" @click="openFolder(task)" title="打开位置">
              <FolderOpenOutline class="size-3.5" />
            </button>
            <button class="p-1.5 rounded-lg hover:bg-ac-heart/10 text-ac-heart-dark transition-colors" @click="deleteTask(task)" title="删除">
              <TrashOutline class="size-3.5" />
            </button>
          </div>
        </div>
      </div>
    </AcCard>

    <!-- 添加下载弹窗 -->
    <AcModal v-model:show="showAddModal" title="添加下载">
      <div class="space-y-4">
        <div class="space-y-1.5">
          <label class="text-sm font-bold">下载链接</label>
          <AcTextarea v-model="addForm.url" rows="3" placeholder="magnet:?xt=... 或 https://..." />
          <p v-if="addForm.url && !/^(magnet:|https?:\/\/)/.test(addForm.url)" class="text-xs text-ac-heart-dark">
            请输入有效的磁力链接或 URL
          </p>
        </div>
        <div class="space-y-1.5">
          <label class="text-sm font-bold">保存路径</label>
          <DirectoryPicker v-model="addForm.save_path" />
          <p class="text-xs text-muted-foreground">留空使用默认下载目录</p>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <AcButton variant="ghost" @click="showAddModal = false">取消</AcButton>
          <AcButton variant="primary" :disabled="!canSubmitAdd" @click="submitAddDownload">确认</AcButton>
        </div>
      </template>
    </AcModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, shallowRef } from 'vue'
import { useToast } from '../../composables/useToast'
import { useConfirm } from '../../composables/useConfirm'
import {
  PauseOutline, PlayOutline, TrashOutline,
  CheckmarkCircleOutline, CloseCircleOutline, TimeOutline,
  CloudDownloadOutline, ArrowDownOutline, ArrowUpOutline,
  AddOutline, SearchOutline,
  ReloadOutline, RefreshOutline, FolderOpenOutline,
} from '@vicons/ionicons5'
import { get, post, del } from '@/utils/api'
import DirectoryPicker from '@/components/Common/DirectoryPicker.vue'
import { AcButton, AcInput, AcCard, AcEmpty, AcTag, AcProgress, AcModal, AcTextarea } from '../../components/ac'

const toast = useToast()
const { confirm } = useConfirm()

const tasks = ref([])
const globalDownloadSpeed = ref(0)
const globalUploadSpeed = ref(0)
const searchKeyword = ref('')
const activeFilter = ref('all')
const selectedIds = ref(new Set())
const showAddModal = ref(false)
const checkingAllUpdates = ref(false)
const updateTimer = ref(null)

const addForm = ref({ url: '', save_path: '' })

let ws = null
let wsReconnectTimer = null

function connectWs() {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const clientId = 'downloads-' + Math.random().toString(36).slice(2, 10)
  const url = `${proto}//${window.location.host}/ws/${clientId}`
  try { ws = new WebSocket(url) } catch (e) { return }
  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data)
      if (msg.type === 'download_progress') {
        const { id, progress } = msg.data || {}
        if (!id) return
        const t = tasks.value.find(x => x.torrent_id === id || x.id === id)
        if (t) t.progress = progress
      } else if (msg.type === 'download_complete') {
        const { id } = msg.data || {}
        const t = tasks.value.find(x => x.torrent_id === id || x.id === id)
        if (t) { t.progress = 100; t.status = 'completed' }
        fetchTasks()
      }
    } catch {}
  }
  ws.onclose = () => {
    ws = null
    if (!wsReconnectTimer) wsReconnectTimer = setTimeout(() => { wsReconnectTimer = null; connectWs() }, 3000)
  }
  ws.onerror = () => { try { ws && ws.close() } catch {} }
}

const filters = computed(() => [
  { key: 'all', label: '全部', count: tasks.value.length },
  { key: 'downloading', label: '下载中', count: statusCount(['downloading', 'queued']) },
  { key: 'paused', label: '已暂停', count: statusCount(['paused']) },
  { key: 'completed', label: '已完成', count: statusCount(['completed']) },
  { key: 'failed', label: '失败', count: statusCount(['failed']) },
])
function statusCount(statuses) { return tasks.value.filter(t => statuses.includes(t.status)).length }

const filteredTasks = computed(() => {
  let list = tasks.value
  if (activeFilter.value !== 'all') {
    const m = { downloading: ['downloading', 'queued'], paused: ['paused'], completed: ['completed'], failed: ['failed'] }
    list = list.filter(t => m[activeFilter.value]?.includes(t.status))
  }
  if (searchKeyword.value) {
    const kw = searchKeyword.value.toLowerCase()
    list = list.filter(t => (t.name || '').toLowerCase().includes(kw))
  }
  return list
})

const hasActive = computed(() => tasks.value.some(t => t.status === 'downloading'))
const hasPaused = computed(() => tasks.value.some(t => t.status === 'paused'))
const allSelected = computed(() => filteredTasks.value.length > 0 && filteredTasks.value.every(t => selectedIds.value.has(t.id)))
const someSelected = computed(() => !allSelected.value && filteredTasks.value.some(t => selectedIds.value.has(t.id)))
const canSubmitAdd = computed(() => addForm.value.url && /^(magnet:|https?:\/\/)/.test(addForm.value.url))
const canBatchPause = computed(() => {
  for (const id of selectedIds.value) { const t = tasks.value.find(x => x.id === id); if (t && t.status === 'downloading') return true }
  return false
})
const canBatchResume = computed(() => {
  for (const id of selectedIds.value) { const t = tasks.value.find(x => x.id === id); if (t && t.status === 'paused') return true }
  return false
})

const STATUS_ICONS = {
  downloading: shallowRef(CloudDownloadOutline),
  queued: shallowRef(TimeOutline),
  paused: shallowRef(PauseOutline),
  completed: shallowRef(CheckmarkCircleOutline),
  failed: shallowRef(CloseCircleOutline),
}
function statusIcon(status) { return STATUS_ICONS[status]?.value || CloudDownloadOutline }
function statusIconClass(status) {
  return { downloading: 'text-ac-grass-dark', queued: 'text-muted-foreground', paused: 'text-ac-sun-dark', completed: 'text-ac-leaf-dark', failed: 'text-ac-heart-dark' }[status] || 'text-muted-foreground'
}

function toggleSelect(id) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) next.delete(id); else next.add(id)
  selectedIds.value = next
}
function toggleSelectAll() {
  const next = new Set(selectedIds.value)
  if (allSelected.value) filteredTasks.value.forEach(t => next.delete(t.id))
  else filteredTasks.value.forEach(t => next.add(t.id))
  selectedIds.value = next
}
function clearSelection() { selectedIds.value = new Set() }

async function fetchTasks() {
  try {
    const data = await get('/downloads', { params: { page_size: 500 } })
    tasks.value = data.tasks || []
    globalDownloadSpeed.value = data.download_speed || 0
    globalUploadSpeed.value = data.upload_speed || 0
  } catch (e) {
    if (e && e.status === 401) {
      if (updateTimer.value) { clearInterval(updateTimer.value); updateTimer.value = null }
      return
    }
  }
}

async function togglePause(task) {
  try {
    const action = task.status === 'downloading' ? 'pause' : 'resume'
    await post(`/downloads/${task.id}/${action}`)
    await fetchTasks()
  } catch (e) { toast.error(e.message || '操作失败') }
}
async function deleteTask(task) {
  const ok = await confirm({ title: '删除任务', content: `确定删除 "${task.name}"？`, variant: 'danger', confirmText: '删除' })
  if (!ok) return
  try { await del(`/downloads/${task.id}`); await fetchTasks() }
  catch (e) { toast.error(e.message || '删除失败') }
}
async function retryTask(task) {
  try { await post(`/downloads/${task.id}/retry`); toast.success('已加入队列'); await fetchTasks() }
  catch (e) { toast.error(e.message || '重试失败') }
}
async function pauseAll() {
  try { await post('/downloads/pause-all'); await fetchTasks() }
  catch (e) { toast.error(e.message || '操作失败') }
}
async function resumeAll() {
  try { await post('/downloads/resume-all'); await fetchTasks() }
  catch (e) { toast.error(e.message || '操作失败') }
}
async function batchAction(action) {
  const ids = [...selectedIds.value]
  await Promise.all(ids.map(id => post(`/downloads/${id}/${action}`).catch(() => null)))
  clearSelection(); await fetchTasks()
}
async function batchDelete() {
  const ok = await confirm({ title: '批量删除', content: `确定删除 ${selectedIds.value.size} 个任务？`, variant: 'danger', confirmText: '删除' })
  if (!ok) return
  const ids = [...selectedIds.value]
  await Promise.all(ids.map(id => del(`/downloads/${id}`).catch(() => null)))
  clearSelection(); await fetchTasks()
}
async function submitAddDownload() {
  if (!canSubmitAdd.value) return
  try {
    await post('/downloads', addForm.value)
    showAddModal.value = false
    addForm.value = { url: '', save_path: '' }
    await fetchTasks()
  } catch (e) { toast.error(e.message || '添加失败') }
}
function openFolder() { toast.info('本地打开功能需要桌面端支持') }

async function handleCheckAllUpdates() {
  checkingAllUpdates.value = true
  try { await post('/anime/check-all-updates'); toast.success('已触发全部追番更新检查') }
  catch (e) { toast.error(e.message || '检查更新失败') }
  finally { setTimeout(() => { checkingAllUpdates.value = false }, 2000) }
}

function formatSize(bytes) {
  if (!bytes || bytes < 0) return '—'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), u.length - 1)
  return (bytes / Math.pow(1024, i)).toFixed(bytes < 1024 * 1024 ? 0 : 1) + ' ' + u[i]
}
function formatTaskSize(task) {
  const total = task.total_bytes || 0
  const done = task.downloaded_bytes || 0
  if (task.status === 'completed') return formatSize(done || total)
  if (task.status === 'downloading' || task.status === 'paused') {
    if (total > 0) return `${formatSize(done)} / ${formatSize(total)}`
    if (done > 0) return formatSize(done)
  }
  if (total > 0) return formatSize(total)
  return '—'
}
function formatSpeed(bps) { return !bps || bps <= 0 ? '0 B/s' : formatSize(bps) + '/s' }
function formatRelativeTime(s) {
  if (!s) return ''
  const d = new Date(s); const diff = (Date.now() - d.getTime()) / 1000
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)} 天前`
  return d.toLocaleDateString('zh-CN')
}
function formatAbsoluteTime(s) {
  if (!s) return ''
  const d = new Date(s)
  return isNaN(d.getTime()) ? s : d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
function sourceLabel(src) { return { bt: 'BT', stream: '流媒体', bangumi: '流媒体', rss: 'RSS', manual: '手动' }[src] || src || '—' }
function sourceTagVariant(src) {
  return { bt: 'sun', stream: 'grass', bangumi: 'grass', rss: 'sky', manual: 'wood' }[src] || 'default'
}

onMounted(() => {
  fetchTasks(); connectWs()
  updateTimer.value = setInterval(fetchTasks, 5000)
})
onUnmounted(() => {
  if (updateTimer.value) clearInterval(updateTimer.value)
  if (wsReconnectTimer) { clearTimeout(wsReconnectTimer); wsReconnectTimer = null }
  if (ws) { try { ws.close() } catch {}; ws = null }
})
</script>
