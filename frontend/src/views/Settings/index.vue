<template>
  <div>
    <AcPageHeader title="⚙️ 系统设置" subtitle="配置您的番剧管理系统偏好设置" />

    <AcTabs v-model="activeTab" :tabs="tabs" />
    <div class="mt-4">
      <!-- 下载偏好 Tab -->
      <div v-if="activeTab === 'download'">
        <DownloadPrefs />
      </div>

      <!-- 外观主题 -->
      <div v-if="activeTab === 'appearance'">
        <AcCard padding="lg" rounded="2xl">
          <div class="flex gap-5">
            <div class="size-11 shrink-0 rounded-2xl bg-ac-sun/40 flex items-center justify-center">
              <span class="text-xl">🎨</span>
            </div>
            <div class="flex-1 space-y-4">
              <div>
                <h3 class="text-lg font-bold tracking-tight text-foreground">主题皮肤</h3>
                <p class="text-sm text-muted-foreground">切换整套视觉风格 —— 选择后立即生效，并记住你的偏好</p>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                <button v-for="opt in SKINS" :key="opt.value"
                  type="button"
                  class="text-left p-4 rounded-2xl border-2 transition-all hover:-translate-y-0.5"
                  :class="skin === opt.value
                    ? 'border-primary bg-primary/10 shadow-md'
                    : 'border-border bg-card hover:border-primary/40'"
                  @click="setSkin(opt.value)">
                  <div class="flex items-center justify-between mb-1">
                    <span class="text-base font-bold text-foreground">{{ opt.label }}</span>
                    <span v-if="skin === opt.value"
                      class="inline-flex items-center gap-1 text-xs font-bold text-primary">
                      <CheckmarkOutline class="size-4" /> 当前
                    </span>
                  </div>
                  <p class="text-xs text-muted-foreground">{{ opt.description }}</p>

                  <!-- 颜色预览条 -->
                  <div class="mt-3 flex gap-1.5">
                    <span v-for="(c, i) in previewColors(opt.value)" :key="i"
                      class="h-4 flex-1 rounded-md border border-black/5"
                      :style="{ background: c }"></span>
                  </div>
                </button>
              </div>

              <p class="text-xs text-muted-foreground">
                💡 切换皮肤只改样式不改业务逻辑，下载、订阅、规则等数据完全保留。
              </p>
            </div>
          </div>
        </AcCard>
      </div>

      <!-- 重命名设置 -->
      <div v-if="activeTab === 'rename'">
        <AcCard padding="lg" rounded="2xl">
          <div class="flex gap-5">
            <div class="size-11 shrink-0 rounded-2xl bg-ac-grass-light/40 flex items-center justify-center">
              <CreateOutline class="size-5 text-ac-grass-dark" />
            </div>
            <div class="flex-1 space-y-4">
              <div>
                <h3 class="text-lg font-bold tracking-tight text-foreground">文件重命名</h3>
                <p class="text-sm text-muted-foreground">配置下载文件的自动重命名规则</p>
              </div>
              <div class="space-y-4">
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">重命名方式</label>
                  <AcSelect v-model="renameForm.rename_method" :options="renameOptions" />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">重命名示例</label>
                  <AcInput :model-value="renameExample" readonly />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">扫描间隔(秒)</label>
                  <AcInput v-model="renameForm.rename_interval" type="number" />
                </div>
                <AcButton variant="primary" :loading="saving.rename" @click="saveRenameSettings">
                  <template #icon><SaveOutline class="size-4" /></template>
                  保存设置
                </AcButton>
              </div>
            </div>
          </div>
        </AcCard>
      </div>

      <!-- 调度器设置 -->
      <div v-if="activeTab === 'scheduler'">
        <AcCard padding="lg" rounded="2xl">
          <div class="flex gap-5">
            <div class="size-11 shrink-0 rounded-2xl bg-ac-sun/40 flex items-center justify-center">
              <TimeOutline class="size-5 text-ac-sun-dark" />
            </div>
            <div class="flex-1 space-y-4">
              <div>
                <h3 class="text-lg font-bold tracking-tight text-foreground">后台调度</h3>
                <p class="text-sm text-muted-foreground">配置定时任务和自动刷新</p>
              </div>
              <div class="space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <label class="text-sm font-bold text-foreground">启用调度器</label>
                    <p class="text-xs text-muted-foreground">{{ schedulerForm.enabled ? '已启用' : '已禁用' }}</p>
                  </div>
                  <AcSwitch v-model="schedulerForm.enabled" />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">RSS刷新间隔(分钟)</label>
                  <AcInput v-model="schedulerForm.rss_interval" type="number" />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">语言偏好</label>
                  <AcSelect v-model="schedulerForm.language" :options="languageOptions" />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">HTTP代理</label>
                  <AcInput v-model="schedulerForm.http_proxy" placeholder="socks5://127.0.0.1:1080 (留空不使用代理)" />
                </div>
                <AcButton variant="primary" :loading="saving.scheduler" @click="saveSchedulerSettings">
                  <template #icon><SaveOutline class="size-4" /></template>
                  保存设置
                </AcButton>
              </div>
            </div>
          </div>
        </AcCard>
      </div>

      <!-- 网络代理 -->
      <div v-if="activeTab === 'network'">
        <AcCard padding="lg" rounded="2xl">
          <div class="flex gap-5">
            <div class="size-11 shrink-0 rounded-2xl bg-ac-sky/40 flex items-center justify-center">
              <GlobeOutline class="size-5 text-ac-sky-dark" />
            </div>
            <div class="flex-1 space-y-4">
              <div>
                <h3 class="text-lg font-bold tracking-tight text-foreground">HTTP 代理</h3>
                <p class="text-sm text-muted-foreground">
                  配置后，Bangumi API、BT Indexer 搜索、RSS 抓取、流媒体拦截等所有出站 HTTP 都会走此代理。
                </p>
              </div>
              <div class="space-y-2">
                <label class="text-sm font-bold text-foreground">代理地址</label>
                <AcInput v-model="proxyForm.http_proxy" placeholder="留空表示直连；Docker 部署建议 http://host.docker.internal:7890" />
                <p class="text-xs text-muted-foreground">
                  支持 <code class="font-num">http://</code>、<code class="font-num">https://</code>、<code class="font-num">socks5://</code>。容器内访问宿主机代理请用 <code class="font-num">host.docker.internal</code>。
                </p>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <AcButton variant="outline" :loading="proxyForm.testing" @click="testProxy">
                  <template #icon><PulseOutline class="size-4" /></template>
                  {{ proxyForm.testing ? '测试中...' : '测试连接' }}
                </AcButton>
                <AcButton variant="primary" :loading="saving.proxy" @click="saveProxy">
                  <template #icon><SaveOutline class="size-4" /></template>
                  保存
                </AcButton>
              </div>
              <div v-if="proxyForm.testResult" class="rounded-2xl border-2 p-3 text-sm"
                :class="proxyForm.testResult.ok ? 'border-ac-leaf bg-ac-leaf/10' : 'border-ac-heart bg-ac-heart/10'">
                <div class="font-bold" :class="proxyForm.testResult.ok ? 'text-ac-leaf-dark' : 'text-ac-heart-dark'">
                  {{ proxyForm.testResult.ok ? '✓ 连接成功' : '✗ 连接失败' }}
                  <span v-if="proxyForm.testResult.latency_ms !== undefined" class="text-muted-foreground font-normal font-num">
                    · {{ proxyForm.testResult.latency_ms }}ms
                  </span>
                </div>
                <div v-if="proxyForm.testResult.target" class="text-xs text-muted-foreground mt-1 font-num">
                  探测目标：{{ proxyForm.testResult.target }}
                </div>
                <div v-if="proxyForm.testResult.error" class="text-xs text-ac-heart-dark mt-1 break-all font-num">
                  {{ proxyForm.testResult.error }}
                </div>
              </div>
              <div class="rounded-2xl border-2 border-ac-sun bg-ac-sun/10 p-3 text-xs text-ac-sun-dark">
                ⚠ 保存后需要重启 backend 才能对已初始化的 HTTP 客户端和 rod 浏览器生效：<br />
                <code class="text-[11px] font-num">docker compose -f docker-compose.dev.yml restart backend</code>
              </div>
            </div>
          </div>
        </AcCard>
      </div>

      <!-- 用户设置 -->
      <div v-if="activeTab === 'user'">
        <AcCard padding="lg" rounded="2xl">
          <div class="flex gap-5">
            <div class="size-11 shrink-0 rounded-2xl bg-ac-leaf/30 flex items-center justify-center">
              <PersonCircleOutline class="size-5 text-ac-leaf-dark" />
            </div>
            <div class="flex-1 space-y-4">
              <div>
                <h3 class="text-lg font-bold tracking-tight text-foreground">账户信息</h3>
                <p class="text-sm text-muted-foreground">修改密码和个人信息</p>
              </div>
              <div class="space-y-4">
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">用户名</label>
                  <AcInput :model-value="userForm.username" disabled />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">旧密码</label>
                  <AcInput v-model="userForm.oldPassword" type="password" placeholder="请输入旧密码" />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">新密码</label>
                  <AcInput v-model="userForm.newPassword" type="password" placeholder="请输入新密码" />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">确认新密码</label>
                  <AcInput v-model="userForm.confirmPassword" type="password" placeholder="请再次输入新密码" />
                </div>
                <AcButton variant="primary" :loading="saving.user" @click="saveUserSettings">
                  <template #icon><CheckmarkOutline class="size-4" /></template>
                  修改密码
                </AcButton>
              </div>
            </div>
          </div>
        </AcCard>
      </div>

      <!-- 系统信息 -->
      <div v-if="activeTab === 'system'" class="space-y-4">
        <!-- 概览卡片 -->
        <div class="grid grid-cols-2 lg:grid-cols-3 gap-4">
          <AcCard hoverable padding="lg" rounded="2xl">
            <div class="size-11 rounded-2xl bg-ac-grass-light/40 flex items-center justify-center mb-3">
              <CodeSlashOutline class="size-5 text-ac-grass-dark" />
            </div>
            <div class="text-sm text-muted-foreground font-bold">系统版本</div>
            <div class="text-xl font-bold mt-1 font-num">{{ systemInfo.version || '1.0.0' }}</div>
            <div class="text-xs text-muted-foreground mt-1">{{ systemInfo.os }}/{{ systemInfo.arch }} · {{ systemInfo.goVersion }}</div>
          </AcCard>
          <AcCard hoverable padding="lg" rounded="2xl">
            <div class="size-11 rounded-2xl bg-ac-leaf/30 flex items-center justify-center mb-3">
              <TimeOutline class="size-5 text-ac-leaf-dark" />
            </div>
            <div class="text-sm text-muted-foreground font-bold">服务运行时间</div>
            <div class="text-xl font-bold mt-1 font-num">{{ systemInfo.uptime || '未知' }}</div>
            <div class="text-xs text-muted-foreground mt-1">主机已开机 {{ systemInfo.hostUptime || '—' }}</div>
          </AcCard>
          <AcCard hoverable padding="lg" rounded="2xl">
            <div class="size-11 rounded-2xl bg-ac-sky/40 flex items-center justify-center mb-3">
              <PulseOutline class="size-5 text-ac-sky-dark" />
            </div>
            <div class="text-sm text-muted-foreground font-bold">Goroutines</div>
            <div class="text-xl font-bold mt-1 font-num">{{ systemInfo.goroutines ?? '—' }}</div>
            <div class="text-xs text-muted-foreground mt-1">Go 堆 {{ fmtBytes(systemInfo.goMemory?.alloc) }} · GC {{ systemInfo.goMemory?.num_gc ?? 0 }} 次</div>
          </AcCard>
        </div>

        <!-- 资源使用率 -->
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <AcCard hoverable padding="lg" rounded="2xl">
            <div class="flex items-center gap-2 mb-2">
              <div class="size-9 rounded-xl bg-ac-sun/40 flex items-center justify-center">
                <HardwareChipOutline class="size-4 text-ac-sun-dark" />
              </div>
              <div class="text-sm text-muted-foreground font-bold flex-1">CPU 使用率</div>
              <div class="text-lg font-bold font-num">{{ fmtPct(systemInfo.cpuUsage) }}%</div>
            </div>
            <AcProgress :percent="clampPct(systemInfo.cpuUsage)" variant="sun" :show-text="false" />
            <div class="text-xs text-muted-foreground mt-1.5">{{ systemInfo.cpuCores || '?' }} 核</div>
          </AcCard>
          <AcCard hoverable padding="lg" rounded="2xl">
            <div class="flex items-center gap-2 mb-2">
              <div class="size-9 rounded-xl bg-ac-sky/40 flex items-center justify-center">
                <ServerOutline class="size-4 text-ac-sky-dark" />
              </div>
              <div class="text-sm text-muted-foreground font-bold flex-1">内存使用率</div>
              <div class="text-lg font-bold font-num">{{ fmtPct(systemInfo.memoryUsage) }}%</div>
            </div>
            <AcProgress :percent="clampPct(systemInfo.memoryUsage)" variant="sky" :show-text="false" />
            <div class="text-xs text-muted-foreground mt-1.5">
              {{ fmtBytes(systemInfo.memory?.used) }} / {{ fmtBytes(systemInfo.memory?.total) }}
            </div>
          </AcCard>
          <AcCard hoverable padding="lg" rounded="2xl">
            <div class="flex items-center gap-2 mb-2">
              <div class="size-9 rounded-xl bg-ac-heart/30 flex items-center justify-center">
                <ServerOutline class="size-4 text-ac-heart-dark" />
              </div>
              <div class="text-sm text-muted-foreground font-bold flex-1">磁盘使用率</div>
              <div class="text-lg font-bold font-num">{{ fmtPct(systemInfo.diskUsage) }}%</div>
            </div>
            <AcProgress :percent="clampPct(systemInfo.diskUsage)" variant="heart" :show-text="false" />
            <div class="text-xs text-muted-foreground mt-1.5">
              {{ fmtBytes(systemInfo.disk?.used) }} / {{ fmtBytes(systemInfo.disk?.total) }}
            </div>
          </AcCard>
        </div>

        <!-- 依赖服务状态 -->
        <AcCard padding="lg" rounded="2xl">
          <h3 class="text-base font-bold tracking-tight text-foreground mb-3">依赖服务</h3>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <!-- 数据库 -->
            <div class="flex items-center justify-between p-3 rounded-2xl border-2 border-ac-sand">
              <div class="flex items-center gap-2.5">
                <span class="inline-block size-2.5 rounded-full" :class="systemInfo.database?.connected ? 'bg-ac-leaf' : 'bg-ac-heart'"></span>
                <div>
                  <div class="text-sm font-bold">PostgreSQL</div>
                  <div class="text-xs text-muted-foreground">
                    {{ systemInfo.database?.connected ? '已连接' : '未连接' }}
                    <template v-if="systemInfo.database?.connected">
                      · 连接 {{ systemInfo.database.in_use }}/{{ systemInfo.database.open }} 活跃
                    </template>
                  </div>
                </div>
              </div>
            </div>
            <!-- qBittorrent -->
            <div class="flex items-center justify-between p-3 rounded-2xl border-2 border-ac-sand">
              <div class="flex items-center gap-2.5">
                <span class="inline-block size-2.5 rounded-full" :class="systemInfo.qbittorrent?.online ? 'bg-ac-leaf' : 'bg-ac-heart'"></span>
                <div>
                  <div class="text-sm font-bold">qBittorrent</div>
                  <div class="text-xs text-muted-foreground">
                    {{ systemInfo.qbittorrent?.online ? '在线' : '离线' }}
                    <template v-if="systemInfo.qbittorrent?.version"> · {{ systemInfo.qbittorrent.version }}</template>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="text-xs text-muted-foreground mt-3">
            每 {{ Math.round(SYS_REFRESH_MS / 1000) }} 秒自动刷新 · 最后更新 {{ lastUpdated || '—' }}
          </div>
        </AcCard>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useAuthStore } from '../../stores/auth'
import { useToast } from '../../composables/useToast'
import { get, put, post } from '../../utils/api'
import DownloadPrefs from './DownloadPrefs.vue'
import {
  SaveOutline, PersonCircleOutline,
  CheckmarkOutline, CodeSlashOutline, TimeOutline,
  HardwareChipOutline, ServerOutline, CreateOutline,
  GlobeOutline, PulseOutline
} from '@vicons/ionicons5'
import { AcPageHeader, AcTabs, AcCard, AcButton, AcInput, AcSelect, AcSwitch, AcProgress } from '../../components/ac'
import { useSkin } from '../../composables/useSkin'

const { skin, setSkin, SKINS } = useSkin()

// 各皮肤的色板预览（仅用于选择卡的"色带"展示）
function previewColors(s) {
  if (s === 'classic') {
    return ['#FFFFFF', '#6366F1', '#A855F7', '#1F2937']
  }
  // ac-grove
  return ['#F7F4E9', '#7CB342', '#FFB74D', '#5D4037']
}

const toast = useToast()
const authStore = useAuthStore()

const activeTab = ref('download')

const tabs = [
  { key: 'download', label: '下载偏好' },
  { key: 'appearance', label: '外观主题' },
  { key: 'rename', label: '重命名设置' },
  { key: 'scheduler', label: '调度器' },
  { key: 'network', label: '网络代理' },
  { key: 'user', label: '用户设置' },
  { key: 'system', label: '系统信息' }
]

const saving = ref({ basic: false, user: false, rename: false, scheduler: false, proxy: false })

const renameForm = reactive({ rename_method: 'pn', rename_interval: 300 })

const renameOptions = [
  { label: '不重命名 (none)', value: 'none' },
  { label: '标准命名: 标题 S01E01.mkv (pn)', value: 'pn' },
  { label: '高级命名: 官方标题 S01E01.mkv (advance)', value: 'advance' },
  { label: '字幕标准: 标题 S01E01.zh.srt (subtitle_pn)', value: 'subtitle_pn' },
  { label: '字幕高级: 官方标题 S01E01.zh.srt (subtitle_advance)', value: 'subtitle_advance' }
]

const renameExample = computed(() => ({
  none: '保持原文件名',
  pn: '葬送的芙莉莲 S01E01.mkv',
  advance: 'Sousou no Frieren S01E01.mkv',
  subtitle_pn: '葬送的芙莉莲 S01E01.zh.srt',
  subtitle_advance: 'Sousou no Frieren S01E01.zh.srt'
}[renameForm.rename_method] || ''))

const schedulerForm = reactive({ enabled: true, rss_interval: 30, language: 'zh', http_proxy: '' })

const languageOptions = [
  { label: '中文', value: 'zh' },
  { label: '日本語', value: 'ja' },
  { label: 'English', value: 'en' }
]

const userForm = ref({
  username: authStore.user?.username || '',
  oldPassword: '', newPassword: '', confirmPassword: ''
})

const systemInfo = ref({ version: '', uptime: '', cpuUsage: 0, memoryUsage: 0, diskUsage: 0 })
const lastUpdated = ref('')
const SYS_REFRESH_MS = 5000
let sysTimer = null

// —— 系统信息格式化辅助 ——
function fmtPct(v) {
  const n = parseFloat(v)
  return Number.isFinite(n) ? n.toFixed(1) : '0.0'
}
function clampPct(v) {
  const n = parseFloat(v)
  if (!Number.isFinite(n)) return 0
  return Math.max(0, Math.min(100, n))
}
function fmtBytes(bytes) {
  const n = Number(bytes)
  if (!Number.isFinite(n) || n <= 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0, val = n
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(val >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

const proxyForm = reactive({ http_proxy: '', testing: false, testResult: null })

async function fetchSettings() {
  try {
    const data = await get('/settings')
    if (data.rename_method) renameForm.rename_method = data.rename_method
    if (data.rename_interval) renameForm.rename_interval = data.rename_interval
    if (data.enable_scheduler !== undefined) schedulerForm.enabled = data.enable_scheduler
    if (data.rss_check_interval) schedulerForm.rss_interval = data.rss_check_interval
    if (data.language) schedulerForm.language = data.language
    if (data.http_proxy) schedulerForm.http_proxy = data.http_proxy
    proxyForm.http_proxy = data.http_proxy || ''
  } catch (e) { console.error('获取设置失败:', e) }
}

async function fetchSystemInfo() {
  try {
    const data = await get('/system/info')
    systemInfo.value = data
    const d = new Date()
    lastUpdated.value = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
  } catch (e) { console.error('获取系统信息失败:', e) }
}

async function saveRenameSettings() { toast.warning('设置暂不支持在线修改，请修改配置文件后重启服务') }
async function saveSchedulerSettings() { toast.warning('设置暂不支持在线修改，请修改配置文件后重启服务') }

async function testProxy() {
  proxyForm.testing = true
  proxyForm.testResult = null
  try {
    const resp = await post('/settings/test-proxy', { proxy: proxyForm.http_proxy || '' })
    proxyForm.testResult = resp
  } catch (e) {
    proxyForm.testResult = { ok: false, error: e?.message || '请求失败' }
  } finally { proxyForm.testing = false }
}

async function saveProxy() {
  saving.value.proxy = true
  try {
    await put('/settings', { http_proxy: proxyForm.http_proxy || '' })
    toast.success('已保存，重启 backend 后生效')
  } catch (e) { toast.error(e?.message || '保存失败') }
  finally { saving.value.proxy = false }
}

async function saveUserSettings() {
  if (!userForm.value.oldPassword) { toast.error('请输入旧密码'); return }
  if (!userForm.value.newPassword) { toast.error('请输入新密码'); return }
  if (userForm.value.newPassword !== userForm.value.confirmPassword) { toast.error('两次输入的密码不一致'); return }
  try {
    saving.value.user = true
    await put('/users/password', { old_password: userForm.value.oldPassword, new_password: userForm.value.newPassword })
    toast.success('密码修改成功')
    userForm.value.oldPassword = ''
    userForm.value.newPassword = ''
    userForm.value.confirmPassword = ''
  } catch (e) {
    if (e?.message) toast.error(e.message)
  } finally { saving.value.user = false }
}

onMounted(() => { fetchSettings(); fetchSystemInfo() })

// 仅当停留在"系统信息" Tab 时轮询，离开就停，避免无谓请求
function startSysPolling() {
  stopSysPolling()
  fetchSystemInfo()
  sysTimer = setInterval(fetchSystemInfo, SYS_REFRESH_MS)
}
function stopSysPolling() {
  if (sysTimer) { clearInterval(sysTimer); sysTimer = null }
}
watch(activeTab, (tab) => {
  if (tab === 'system') startSysPolling()
  else stopSysPolling()
}, { immediate: true })
onUnmounted(stopSysPolling)
</script>
