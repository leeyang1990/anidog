<template>
  <div class="space-y-6">
    <!-- 源启用 -->
    <div class="bg-muted/50 rounded-lg p-6">
      <div class="flex gap-5">
        <div class="h-10 w-10 shrink-0 rounded-md bg-primary/10 flex items-center justify-center">
          <n-icon size="22"><LayersOutline /></n-icon>
        </div>
        <div class="flex-1 space-y-4">
          <div>
            <h3 class="text-lg font-semibold tracking-tight">下载源</h3>
            <p class="text-sm text-muted-foreground">启用/禁用各类下载源；系统会按优先级自动填坑</p>
          </div>

          <!-- Stream / BT / RSS 大类开关 -->
          <div class="space-y-3">
            <label class="flex items-center justify-between p-3 rounded-md border">
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium">🎬 流媒体源</span>
                  <span v-if="form.source_enabled_stream" class="text-xs text-emerald-600">已启用</span>
                  <span v-else class="text-xs text-muted-foreground">已禁用</span>
                </div>
                <p class="text-xs text-muted-foreground mt-0.5">
                  从 aafun/AGE 等流媒体站点抓视频流（速度快，画质一般）·
                  <router-link to="/stream-rules" class="text-primary hover:underline">管理规则</router-link>
                </p>
              </div>
              <SwitchButton v-model="form.source_enabled_stream" />
            </label>

            <label class="flex items-center justify-between p-3 rounded-md border">
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium">🧲 BT 种子搜索</span>
                  <span v-if="form.source_enabled_bt" class="text-xs text-emerald-600">已启用</span>
                  <span v-else class="text-xs text-muted-foreground">已禁用</span>
                </div>
                <p class="text-xs text-muted-foreground mt-0.5">
                  从公开 BT 站聚合搜索种子（画质高、速度取决于做种数）
                </p>
              </div>
              <SwitchButton v-model="form.source_enabled_bt" />
            </label>

            <label class="flex items-center justify-between p-3 rounded-md border">
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium">📡 RSS 订阅</span>
                  <span v-if="form.source_enabled_rss" class="text-xs text-emerald-600">已启用</span>
                  <span v-else class="text-xs text-muted-foreground">已禁用</span>
                </div>
                <p class="text-xs text-muted-foreground mt-0.5">
                  按已订阅的 Mikan 等 RSS 源自动下载 ·
                  <router-link to="/rss" class="text-primary hover:underline">管理 Feed</router-link>
                </p>
              </div>
              <SwitchButton v-model="form.source_enabled_rss" />
            </label>
          </div>

          <!-- BT Indexer 精细开关 -->
          <div v-if="form.source_enabled_bt" class="pt-2">
            <label class="text-xs font-medium text-muted-foreground mb-2 block">启用的 BT Indexer</label>
            <div class="flex flex-wrap gap-2">
              <button v-for="ix in indexerOptions" :key="ix.value"
                type="button"
                class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium border transition-colors"
                :class="isIndexerEnabled(ix.value)
                  ? 'bg-primary text-primary-foreground border-primary'
                  : 'bg-background border-border text-muted-foreground hover:border-primary/50'"
                @click="toggleIndexer(ix.value)">
                <n-icon v-if="isIndexerEnabled(ix.value)" size="12"><CheckmarkOutline /></n-icon>
                {{ ix.label }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 质量偏好 -->
    <div class="bg-muted/50 rounded-lg p-6">
      <div class="flex gap-5">
        <div class="h-10 w-10 shrink-0 rounded-md bg-primary/10 flex items-center justify-center">
          <n-icon size="22"><DiamondOutline /></n-icon>
        </div>
        <div class="flex-1 space-y-4">
          <div>
            <h3 class="text-lg font-semibold tracking-tight">质量偏好</h3>
            <p class="text-sm text-muted-foreground">系统按这些偏好对所有候选打分，选出"最佳匹配"</p>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="text-sm font-medium">分辨率</label>
              <select v-model="form.quality"
                class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring">
                <option value="720p">720p</option>
                <option value="1080p">1080p（推荐）</option>
                <option value="2160p">2160p / 4K</option>
              </select>
            </div>

            <div class="space-y-2">
              <label class="text-sm font-medium">首选语言</label>
              <div class="flex flex-wrap gap-2 h-9 items-center">
                <label v-for="opt in langOptions" :key="opt.value"
                  class="inline-flex items-center gap-1 text-sm">
                  <input type="checkbox" :value="opt.value"
                    :checked="form.languages.includes(opt.value)"
                    @change="toggleLang(opt.value)"
                    class="accent-primary" />
                  {{ opt.label }}
                </label>
              </div>
            </div>

            <div class="space-y-2">
              <label class="text-sm font-medium">最小体积 (MB)</label>
              <input v-model.number="form.min_size_mb" type="number" min="0"
                class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">最大体积 (MB，0=不限)</label>
              <input v-model.number="form.max_size_mb" type="number" min="0"
                class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm" />
            </div>
          </div>

          <div class="space-y-2">
            <label class="text-sm font-medium">字幕组白名单</label>
            <div class="flex flex-wrap gap-1.5 p-2 rounded-md border border-input bg-background min-h-10">
              <span v-for="(g, idx) in form.groups" :key="g"
                class="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-1 text-xs">
                {{ g }}
                <button type="button" @click="form.groups.splice(idx, 1)"
                  class="hover:text-destructive">×</button>
              </span>
              <input v-model="newGroup" placeholder="输入后回车添加"
                @keydown.enter.prevent="addGroup"
                class="flex-1 min-w-[120px] outline-none bg-transparent text-xs" />
            </div>
            <p class="text-xs text-muted-foreground">命中白名单的字幕组评分 +100，其他 -20</p>
          </div>
        </div>
      </div>
    </div>

    <!-- 调度策略 -->
    <div class="bg-muted/50 rounded-lg p-6">
      <div class="flex gap-5">
        <div class="h-10 w-10 shrink-0 rounded-md bg-primary/10 flex items-center justify-center">
          <n-icon size="22"><TimerOutline /></n-icon>
        </div>
        <div class="flex-1 space-y-4">
          <div>
            <h3 class="text-lg font-semibold tracking-tight">调度策略</h3>
            <p class="text-sm text-muted-foreground">每隔一段时间自动检查所有追番的缺失集</p>
          </div>

          <div class="space-y-2">
            <label class="text-sm font-medium">源优先级（从上到下）</label>
            <div class="space-y-1.5">
              <div v-for="(src, idx) in form.priority" :key="src"
                class="flex items-center gap-2 p-2.5 rounded-md border bg-background">
                <span class="text-sm font-mono text-muted-foreground w-6">{{ idx + 1 }}.</span>
                <span class="text-sm flex-1">{{ sourceLabel(src) }}</span>
                <button type="button"
                  class="h-7 w-7 rounded hover:bg-muted inline-flex items-center justify-center disabled:opacity-30"
                  :disabled="idx === 0"
                  @click="movePriority(idx, -1)">↑</button>
                <button type="button"
                  class="h-7 w-7 rounded hover:bg-muted inline-flex items-center justify-center disabled:opacity-30"
                  :disabled="idx === form.priority.length - 1"
                  @click="movePriority(idx, 1)">↓</button>
              </div>
            </div>
            <p class="text-xs text-muted-foreground">主源找到就入队；未找到依次尝试后备源</p>
          </div>

          <div class="space-y-2 max-w-xs">
            <label class="text-sm font-medium">定时检查间隔（分钟）</label>
            <input v-model.number="form.check_interval" type="number" min="5" max="1440"
              class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm" />
            <p class="text-xs text-muted-foreground">最小 5 分钟，建议 30 分钟</p>
          </div>
        </div>
      </div>
    </div>

    <div class="flex items-center justify-end gap-3 pt-2">
      <button @click="load"
        class="h-10 px-4 rounded-md border border-input bg-background hover:bg-accent text-sm">
        重置
      </button>
      <button @click="save" :disabled="saving"
        class="bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-10 px-6 text-sm font-medium disabled:opacity-50">
        {{ saving ? '保存中...' : '保存设置' }}
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useMessage, NIcon } from 'naive-ui'
import {
  LayersOutline, DiamondOutline, TimerOutline, CheckmarkOutline,
} from '@vicons/ionicons5'
import { get, put } from '@/utils/api'
import SwitchButton from './SwitchButton.vue'

const message = useMessage()

const defaults = {
  quality: '1080p',
  groups: ['LoliHouse', '桜都字幕组', 'ANi', '喵萌奶茶屋', '北宇治字幕组'],
  languages: ['simplified'],
  min_size_mb: 100,
  max_size_mb: 0,
  source_enabled_stream: true,
  source_enabled_bt: true,
  source_enabled_rss: true,
  indexer_enabled: { mikan: true, dmhy: true, bangumimoe: true, nyaa: false },
  priority: ['bt', 'stream', 'rss'],
  check_interval: 30,
}

const form = reactive({ ...JSON.parse(JSON.stringify(defaults)) })
const newGroup = ref('')
const saving = ref(false)

const indexerOptions = [
  { label: 'Mikan（蜜柑）', value: 'mikan' },
  { label: 'Dmhy（动漫花园）', value: 'dmhy' },
  { label: 'BangumiMoe（萌番组）', value: 'bangumimoe' },
  { label: 'Nyaa（英语圈）', value: 'nyaa' },
]

const langOptions = [
  { label: '简中', value: 'simplified' },
  { label: '繁中', value: 'traditional' },
  { label: '日文', value: 'japanese' },
  { label: '英文', value: 'english' },
]

function sourceLabel(src) {
  return { bt: '🧲 BT 种子', stream: '🎬 流媒体', rss: '📡 RSS' }[src] || src
}

function isIndexerEnabled(name) {
  return !!form.indexer_enabled[name]
}

function toggleIndexer(name) {
  form.indexer_enabled[name] = !form.indexer_enabled[name]
}

function toggleLang(lang) {
  const idx = form.languages.indexOf(lang)
  if (idx >= 0) form.languages.splice(idx, 1)
  else form.languages.push(lang)
}

function addGroup() {
  const g = newGroup.value.trim()
  if (g && !form.groups.includes(g)) form.groups.push(g)
  newGroup.value = ''
}

function movePriority(idx, dir) {
  const newIdx = idx + dir
  if (newIdx < 0 || newIdx >= form.priority.length) return
  const arr = form.priority
  ;[arr[idx], arr[newIdx]] = [arr[newIdx], arr[idx]]
}

async function load() {
  try {
    const data = await get('/settings')
    const map = Array.isArray(data) ? Object.fromEntries(data.map(s => [s.key, s.value])) : (data || {})

    form.quality = map['download.quality'] || defaults.quality
    try { form.groups = JSON.parse(map['download.groups'] || '[]') } catch { form.groups = [...defaults.groups] }
    if (form.groups.length === 0) form.groups = [...defaults.groups]
    try { form.languages = JSON.parse(map['download.languages'] || '[]') } catch { form.languages = [...defaults.languages] }
    if (form.languages.length === 0) form.languages = [...defaults.languages]

    form.min_size_mb = parseInt(map['download.min_size_mb']) || defaults.min_size_mb
    form.max_size_mb = parseInt(map['download.max_size_mb']) || defaults.max_size_mb

    form.source_enabled_stream = boolVal(map['download.source_enabled.stream'], true)
    form.source_enabled_bt = boolVal(map['download.source_enabled.bt'], true)
    form.source_enabled_rss = boolVal(map['download.source_enabled.rss'], true)

    for (const n of Object.keys(defaults.indexer_enabled)) {
      const v = map['download.indexer_enabled.' + n]
      form.indexer_enabled[n] = boolVal(v, defaults.indexer_enabled[n])
    }

    try { form.priority = JSON.parse(map['download.priority'] || '[]') } catch { form.priority = [...defaults.priority] }
    if (!Array.isArray(form.priority) || form.priority.length === 0) form.priority = [...defaults.priority]

    form.check_interval = parseInt(map['download.check_interval']) || defaults.check_interval
  } catch (e) {
    console.error('加载偏好失败', e)
  }
}

function boolVal(v, fallback) {
  if (v === undefined || v === null || v === '') return fallback
  return v === 'true' || v === '1' || v === true
}

async function save() {
  saving.value = true
  const payload = {
    'download.quality': form.quality,
    'download.groups': JSON.stringify(form.groups),
    'download.languages': JSON.stringify(form.languages),
    'download.min_size_mb': String(form.min_size_mb),
    'download.max_size_mb': String(form.max_size_mb),
    'download.source_enabled.stream': String(form.source_enabled_stream),
    'download.source_enabled.bt': String(form.source_enabled_bt),
    'download.source_enabled.rss': String(form.source_enabled_rss),
    'download.priority': JSON.stringify(form.priority),
    'download.check_interval': String(form.check_interval),
  }
  for (const [n, on] of Object.entries(form.indexer_enabled)) {
    payload['download.indexer_enabled.' + n] = String(on)
  }
  try {
    await put('/settings', payload)
    message.success('下载偏好已保存')
  } catch (e) {
    message.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
