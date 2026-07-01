<template>
  <div class="space-y-6">
    <!-- 主动下载源（orchestrator 按优先级填坑） -->
    <AcCard padding="lg" rounded="2xl">
      <div class="flex gap-5">
        <div class="size-11 shrink-0 rounded-2xl bg-ac-grass-light/40 flex items-center justify-center">
          <LayersOutline class="size-5 text-ac-grass-dark" />
        </div>
        <div class="flex-1 space-y-4">
          <div>
            <h3 class="text-lg font-bold tracking-tight text-foreground">主动下载源</h3>
            <p class="text-sm text-muted-foreground">系统会按优先级主动为缺失集"填坑"，先找到就入队</p>
          </div>

          <div class="space-y-3">
            <label class="flex items-center justify-between p-3 rounded-2xl border-2 border-ac-sand">
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-sm font-bold">🎬 流媒体源</span>
                  <span v-if="form.source_enabled_stream" class="text-xs text-ac-leaf-dark font-bold">已启用</span>
                  <span v-else class="text-xs text-muted-foreground font-bold">已禁用</span>
                </div>
                <p class="text-xs text-muted-foreground mt-0.5">
                  从 aafun/AGE 等流媒体站点抓视频流（速度快，画质一般）·
                  <router-link to="/stream-rules" class="text-ac-grass-dark hover:underline font-bold">管理规则</router-link>
                </p>
              </div>
              <AcSwitch v-model="form.source_enabled_stream" />
            </label>

            <label class="flex items-center justify-between p-3 rounded-2xl border-2 border-ac-sand">
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-sm font-bold">🧲 BT 种子搜索</span>
                  <span v-if="form.source_enabled_bt" class="text-xs text-ac-leaf-dark font-bold">已启用</span>
                  <span v-else class="text-xs text-muted-foreground font-bold">已禁用</span>
                </div>
                <p class="text-xs text-muted-foreground mt-0.5">
                  从公开 BT 站聚合搜索种子（画质高、速度取决于做种数）
                </p>
              </div>
              <AcSwitch v-model="form.source_enabled_bt" />
            </label>
          </div>

          <div v-if="form.source_enabled_bt" class="pt-2">
            <label class="text-xs font-bold text-muted-foreground mb-2 block">启用的 BT Indexer</label>
            <div class="flex flex-wrap gap-2">
              <button v-for="ix in indexerOptions" :key="ix.value"
                type="button"
                class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-bold border-2 transition-colors"
                :class="isIndexerEnabled(ix.value)
                  ? 'bg-ac-grass text-white border-ac-grass-dark'
                  : 'bg-card border-ac-sand text-muted-foreground hover:border-ac-grass'"
                @click="toggleIndexer(ix.value)">
                <CheckmarkOutline v-if="isIndexerEnabled(ix.value)" class="size-3" />
                {{ ix.label }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </AcCard>

    <!-- RSS 订阅（被动通道） -->
    <AcCard padding="lg" rounded="2xl">
      <div class="flex gap-5">
        <div class="size-11 shrink-0 rounded-2xl bg-ac-sun/40 flex items-center justify-center">
          <span class="text-xl">📡</span>
        </div>
        <div class="flex-1 space-y-4">
          <div>
            <h3 class="text-lg font-bold tracking-tight text-foreground">RSS 订阅（被动）</h3>
            <p class="text-sm text-muted-foreground">RSS 是按规则被动接收 feed 的独立通道，与上面的"主动下载"互不干扰</p>
          </div>

          <label class="flex items-center justify-between p-3 rounded-2xl border-2 border-ac-sand">
            <div>
              <div class="flex items-center gap-2">
                <span class="text-sm font-bold">启用 RSS 定时刷新与规则下载</span>
                <span v-if="form.source_enabled_rss" class="text-xs text-ac-leaf-dark font-bold">已启用</span>
                <span v-else class="text-xs text-muted-foreground font-bold">已禁用</span>
              </div>
              <p class="text-xs text-muted-foreground mt-0.5">
                定时刷新已订阅的 Mikan 等 feed，命中规则即下载 ·
                <router-link to="/rss" class="text-ac-grass-dark hover:underline font-bold">管理 Feed</router-link>
              </p>
            </div>
            <AcSwitch v-model="form.source_enabled_rss" />
          </label>
        </div>
      </div>
    </AcCard>

    <!-- 质量偏好 -->
    <AcCard padding="lg" rounded="2xl">
      <div class="flex gap-5">
        <div class="size-11 shrink-0 rounded-2xl bg-ac-sun/40 flex items-center justify-center">
          <DiamondOutline class="size-5 text-ac-sun-dark" />
        </div>
        <div class="flex-1 space-y-4">
          <div>
            <h3 class="text-lg font-bold tracking-tight text-foreground">质量偏好</h3>
            <p class="text-sm text-muted-foreground">系统按这些偏好对所有候选打分，选出"最佳匹配"</p>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="text-sm font-bold text-foreground">分辨率</label>
              <AcSelect v-model="form.quality" :options="qualityOptions" />
            </div>

            <div class="space-y-2">
              <label class="text-sm font-bold text-foreground">首选语言</label>
              <div class="flex flex-wrap gap-3 h-9 items-center">
                <label v-for="opt in langOptions" :key="opt.value" class="inline-flex items-center gap-1.5 text-sm cursor-pointer">
                  <AcCheckbox :model-value="form.languages.includes(opt.value)" @update:model-value="toggleLang(opt.value)" />
                  {{ opt.label }}
                </label>
              </div>
            </div>

            <div class="space-y-2">
              <label class="text-sm font-bold text-foreground">最小体积 (MB)</label>
              <AcInput v-model="form.min_size_mb" type="number" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-bold text-foreground">最大体积 (MB，0=不限)</label>
              <AcInput v-model="form.max_size_mb" type="number" />
            </div>
          </div>

          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">字幕组白名单</label>
            <div class="flex flex-wrap gap-1.5 p-2 rounded-2xl border-2 border-ac-sand bg-card min-h-11">
              <span v-for="(g, idx) in form.groups" :key="g"
                class="inline-flex items-center gap-1 rounded-full bg-ac-sand px-2.5 py-1 text-xs font-bold text-ac-wood-dark">
                {{ g }}
                <button type="button" @click="form.groups.splice(idx, 1)" class="hover:text-ac-heart-dark">×</button>
              </span>
              <input v-model="newGroup" placeholder="输入后回车添加"
                @keydown.enter.prevent="addGroup"
                class="flex-1 min-w-[120px] outline-none bg-transparent text-xs px-2 py-1" />
            </div>
            <p class="text-xs text-muted-foreground">命中白名单的字幕组评分 +100，其他 -20</p>
          </div>
        </div>
      </div>
    </AcCard>

    <!-- 调度策略 -->
    <AcCard padding="lg" rounded="2xl">
      <div class="flex gap-5">
        <div class="size-11 shrink-0 rounded-2xl bg-ac-sky/40 flex items-center justify-center">
          <TimerOutline class="size-5 text-ac-sky-dark" />
        </div>
        <div class="flex-1 space-y-4">
          <div>
            <h3 class="text-lg font-bold tracking-tight text-foreground">调度策略</h3>
            <p class="text-sm text-muted-foreground">每隔一段时间自动检查所有追番的缺失集</p>
          </div>

          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">主动源优先级（从上到下）</label>
            <div class="space-y-1.5">
              <div v-for="(src, idx) in form.priority" :key="src"
                class="flex items-center gap-2 p-2.5 rounded-2xl border-2 border-ac-sand bg-card">
                <span class="text-sm font-num text-muted-foreground w-6 font-bold">{{ idx + 1 }}.</span>
                <span class="text-sm flex-1">{{ sourceLabel(src) }}</span>
                <button type="button"
                  class="size-7 rounded-xl hover:bg-ac-sand inline-flex items-center justify-center disabled:opacity-30"
                  :disabled="idx === 0" @click="movePriority(idx, -1)">↑</button>
                <button type="button"
                  class="size-7 rounded-xl hover:bg-ac-sand inline-flex items-center justify-center disabled:opacity-30"
                  :disabled="idx === form.priority.length - 1" @click="movePriority(idx, 1)">↓</button>
              </div>
            </div>
            <p class="text-xs text-muted-foreground">主源找到就入队；未找到依次尝试后备源（RSS 是被动通道，不参与此优先级）</p>
          </div>

          <div class="space-y-2 max-w-xs">
            <label class="text-sm font-bold text-foreground">定时检查间隔（分钟）</label>
            <AcInput v-model="form.check_interval" type="number" />
            <p class="text-xs text-muted-foreground">最小 5 分钟，建议 30 分钟</p>
          </div>
        </div>
      </div>
    </AcCard>

    <!-- 下载与归档（原"下载管理 → 设置"弹窗迁移至此） -->
    <AcCard padding="lg" rounded="2xl">
      <div class="flex gap-5">
        <div class="size-11 shrink-0 rounded-2xl bg-ac-leaf/30 flex items-center justify-center">
          <FolderOpenOutline class="size-5 text-ac-leaf-dark" />
        </div>
        <div class="flex-1 space-y-4">
          <div>
            <h3 class="text-lg font-bold tracking-tight text-foreground">下载与归档</h3>
            <p class="text-sm text-muted-foreground">下载落盘目录、并发数与文件重命名规则</p>
          </div>

          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">默认下载目录</label>
            <DirectoryPicker v-model="form.download_dir" />
            <p class="text-xs text-muted-foreground">所有源的文件按 <span class="font-num">&lt;番剧名 (年份)&gt;/Season NN</span> 归档到此目录下</p>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="text-sm font-bold text-foreground">并发下载数</label>
              <AcInput v-model="form.max_concurrent" type="number" />
              <p class="text-xs text-muted-foreground">同时进行的下载任务上限</p>
            </div>
            <div class="space-y-2">
              <label class="text-sm font-bold text-foreground">重命名扫描间隔（秒）</label>
              <AcInput v-model="form.rename_interval" type="number" />
              <p class="text-xs text-muted-foreground">最小 60 秒，建议 300 秒</p>
            </div>
          </div>

          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">文件重命名方式</label>
            <AcSelect v-model="form.rename_method" :options="renameOptions" />
            <p class="text-xs text-muted-foreground">示例：<span class="font-num">{{ renameExample }}</span></p>
          </div>
        </div>
      </div>
    </AcCard>

    <div class="flex items-center justify-end gap-3 pt-2">
      <AcButton variant="ghost" @click="load">重置</AcButton>
      <AcButton variant="primary" :loading="saving" @click="save">{{ saving ? '保存中...' : '保存设置' }}</AcButton>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { LayersOutline, DiamondOutline, TimerOutline, CheckmarkOutline, FolderOpenOutline } from '@vicons/ionicons5'
import { get, put } from '@/utils/api'
import { useToast } from '@/composables/useToast'
import DirectoryPicker from '@/components/Common/DirectoryPicker.vue'
import { AcCard, AcButton, AcInput, AcSelect, AcSwitch, AcCheckbox } from '@/components/ac'

const toast = useToast()

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
  priority: ['bt', 'stream'],
  check_interval: 30,
  // 下载与归档（原"下载管理 → 设置"弹窗）
  download_dir: '/downloads',
  max_concurrent: 3,
  rename_method: 'pn',
  rename_interval: 300,
}

const form = reactive({ ...JSON.parse(JSON.stringify(defaults)) })
const newGroup = ref('')
const saving = ref(false)

const qualityOptions = [
  { label: '720p', value: '720p' },
  { label: '1080p（推荐）', value: '1080p' },
  { label: '2160p / 4K', value: '2160p' },
]

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

const renameOptions = [
  { label: '不重命名 (none)', value: 'none' },
  { label: '标准命名：标题 S01E01.mkv (pn)', value: 'pn' },
  { label: '高级命名：官方标题 S01E01.mkv (advance)', value: 'advance' },
  { label: '字幕标准：标题 S01E01.zh.srt (subtitle_pn)', value: 'subtitle_pn' },
  { label: '字幕高级：官方标题 S01E01.zh.srt (subtitle_advance)', value: 'subtitle_advance' },
]

const renameExample = computed(() => ({
  none: '保持原文件名',
  pn: '葬送的芙莉莲 S01E01.mkv',
  advance: 'Sousou no Frieren S01E01.mkv',
  subtitle_pn: '葬送的芙莉莲 S01E01.zh.srt',
  subtitle_advance: 'Sousou no Frieren S01E01.zh.srt',
}[form.rename_method] || ''))

function sourceLabel(src) {
  return { bt: '🧲 BT 种子', stream: '🎬 流媒体' }[src] || src
}

function isIndexerEnabled(name) { return !!form.indexer_enabled[name] }
function toggleIndexer(name) { form.indexer_enabled[name] = !form.indexer_enabled[name] }

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
    // 兼容旧数据：过滤掉 'rss'（已不再作为主动源）
    form.priority = form.priority.filter(s => s !== 'rss')
    if (form.priority.length === 0) form.priority = [...defaults.priority]
    form.check_interval = parseInt(map['download.check_interval']) || defaults.check_interval

    // 下载与归档（沿用原 DownloadList 的扁平 key：media_root/download_dir/max_concurrent/rename_*）
    form.download_dir = map['download_dir'] || map['media_root'] || defaults.download_dir
    form.max_concurrent = parseInt(map['max_concurrent']) || parseInt(map['stream_max_concurrent']) || defaults.max_concurrent
    form.rename_method = map['rename_method'] || defaults.rename_method
    form.rename_interval = parseInt(map['rename_interval']) || defaults.rename_interval
  } catch (e) { console.error('加载偏好失败', e) }
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
    // 下载与归档（扁平 key，与后端白名单一致）
    'download_dir': form.download_dir,
    'max_concurrent': String(form.max_concurrent),
    'rename_method': form.rename_method,
    'rename_interval': String(form.rename_interval),
  }
  for (const [n, on] of Object.entries(form.indexer_enabled)) {
    payload['download.indexer_enabled.' + n] = String(on)
  }
  try {
    await put('/settings', payload)
    toast.success('下载偏好已保存')
  } catch (e) { toast.error(e.message || '保存失败') }
  finally { saving.value = false }
}

onMounted(load)
</script>
