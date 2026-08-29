<template>
  <div>
    <AcPageHeader :title="t('settings.title')" :subtitle="t('settings.subtitle')" />

    <AcTabs v-model="activeTab" :tabs="tabs" />
    <div class="mt-4">
      <!-- 下载偏好 Tab -->
      <div v-if="activeTab === 'download'">
        <DownloadPrefs />
      </div>

      <!-- 外观主题 -->
      <div v-if="activeTab === 'appearance'">
        <AcCard padding="lg" rounded="2xl" class="mb-4">
          <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex gap-4">
              <div class="size-11 shrink-0 rounded-2xl bg-ac-sky/40 flex items-center justify-center">
                <GlobeOutline class="size-5 text-ac-sky-dark" />
              </div>
              <div>
                <h3 class="text-lg font-bold tracking-tight text-foreground">{{ t('settings.languageTitle') }}</h3>
                <p class="text-sm text-muted-foreground">{{ t('settings.languageDescription') }}</p>
              </div>
            </div>
            <LocaleSwitcher />
          </div>
        </AcCard>

        <AcCard padding="lg" rounded="2xl">
          <div class="flex gap-5">
            <div class="size-11 shrink-0 rounded-2xl bg-ac-sun/40 flex items-center justify-center">
              <span class="text-xl">🎨</span>
            </div>
            <div class="flex-1 space-y-4">
              <div>
                <h3 class="text-lg font-bold tracking-tight text-foreground">{{ t('settings.details.themeTitle') }}</h3>
                <p class="text-sm text-muted-foreground">{{ t('settings.details.themeDesc') }}</p>
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
                    <span class="text-base font-bold text-foreground">{{ t(opt.labelKey) }}</span>
                    <span v-if="skin === opt.value"
                      class="inline-flex items-center gap-1 text-xs font-bold text-primary">
                      <CheckmarkOutline class="size-4" /> {{ t('settings.details.current') }}
                    </span>
                  </div>
                  <p class="text-xs text-muted-foreground">{{ t(opt.descriptionKey) }}</p>

                  <!-- 颜色预览条 -->
                  <div class="mt-3 flex gap-1.5">
                    <span v-for="(c, i) in previewColors(opt.value)" :key="i"
                      class="h-4 flex-1 rounded-md border border-black/5"
                      :style="{ background: c }"></span>
                  </div>
                </button>
              </div>

              <p class="text-xs text-muted-foreground">
                {{ t('settings.details.themeNote') }}
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
                <h3 class="text-lg font-bold tracking-tight text-foreground">{{ t('settings.details.renameTitle') }}</h3>
                <p class="text-sm text-muted-foreground">{{ t('settings.details.renameDesc') }}</p>
              </div>
              <div class="rounded-2xl border-2 border-ac-sand bg-ac-cream/40 p-4 text-sm text-muted-foreground">
                {{ t('settings.details.renameFormat') }}
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
                <h3 class="text-lg font-bold tracking-tight text-foreground">{{ t('settings.details.schedulerTitle') }}</h3>
                <p class="text-sm text-muted-foreground">{{ t('settings.details.schedulerDesc') }}</p>
              </div>
              <div class="space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <label class="text-sm font-bold text-foreground">{{ t('settings.details.schedulerEnable') }}</label>
                    <p class="text-xs text-muted-foreground">{{ schedulerForm.enabled ? t('common.enabled') : t('common.disabled') }}</p>
                  </div>
                  <AcSwitch v-model="schedulerForm.enabled" />
                </div>
                <AcButton variant="primary" :loading="saving.scheduler" @click="saveSchedulerSettings">
                  <template #icon><SaveOutline class="size-4" /></template>
                  {{ t('common.saveSettings') }}
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
                <h3 class="text-lg font-bold tracking-tight text-foreground">{{ t('settings.details.proxyTitle') }}</h3>
                <p class="text-sm text-muted-foreground">{{ t('settings.details.proxyDesc') }}</p>
              </div>
              <div class="space-y-2">
                <label class="text-sm font-bold text-foreground">{{ t('settings.details.proxyAddress') }}</label>
                <AcInput v-model="proxyForm.http_proxy" :placeholder="t('settings.details.proxyPlaceholder')" />
                <p class="text-xs text-muted-foreground">{{ t('settings.details.proxyHint') }}</p>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <AcButton variant="outline" :loading="proxyForm.testing" @click="testProxy">
                  <template #icon><PulseOutline class="size-4" /></template>
                  {{ proxyForm.testing ? t('settings.details.testing') : t('settings.details.testConnection') }}
                </AcButton>
                <AcButton variant="primary" :loading="saving.proxy" @click="saveProxy">
                  <template #icon><SaveOutline class="size-4" /></template>
                  {{ t('common.save') }}
                </AcButton>
              </div>
              <div v-if="proxyForm.testResult" class="rounded-2xl border-2 p-3 text-sm"
                :class="proxyForm.testResult.ok ? 'border-ac-leaf bg-ac-leaf/10' : 'border-ac-heart bg-ac-heart/10'">
                <div class="font-bold" :class="proxyForm.testResult.ok ? 'text-ac-leaf-dark' : 'text-ac-heart-dark'">
                  {{ proxyForm.testResult.ok ? t('settings.details.connectionSuccess') : t('settings.details.connectionFailed') }}
                  <span v-if="proxyForm.testResult.latency_ms !== undefined" class="text-muted-foreground font-normal font-num">
                    · {{ proxyForm.testResult.latency_ms }}ms
                  </span>
                </div>
                <div v-if="proxyForm.testResult.target" class="text-xs text-muted-foreground mt-1 font-num">
                  {{ t('settings.details.probeTarget', { target: proxyForm.testResult.target }) }}
                </div>
                <div v-if="proxyForm.testResult.error" class="text-xs text-ac-heart-dark mt-1 break-all font-num">
                  {{ proxyForm.testResult.error }}
                </div>
              </div>
              <div class="rounded-2xl border-2 border-ac-sun bg-ac-sun/10 p-3 text-xs text-ac-sun-dark">
                {{ t('settings.details.proxyHotUpdate') }}
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
                <h3 class="text-lg font-bold tracking-tight text-foreground">{{ t('settings.details.accountTitle') }}</h3>
                <p class="text-sm text-muted-foreground">{{ t('settings.details.accountDesc') }}</p>
              </div>
              <div class="space-y-4">
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">{{ t('settings.details.username') }}</label>
                  <AcInput :model-value="userForm.username" disabled />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">{{ t('settings.details.oldPassword') }}</label>
                  <AcInput v-model="userForm.oldPassword" type="password" :placeholder="t('settings.details.oldPasswordPlaceholder')" />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">{{ t('settings.details.newPassword') }}</label>
                  <AcInput v-model="userForm.newPassword" type="password" :placeholder="t('settings.details.newPasswordPlaceholder')" />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-bold text-foreground">{{ t('settings.details.confirmPassword') }}</label>
                  <AcInput v-model="userForm.confirmPassword" type="password" :placeholder="t('settings.details.confirmPasswordPlaceholder')" />
                </div>
                <AcButton variant="primary" :loading="saving.user" @click="saveUserSettings">
                  <template #icon><CheckmarkOutline class="size-4" /></template>
                  {{ t('settings.details.changePassword') }}
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
            <div class="text-sm text-muted-foreground font-bold">{{ t('settings.details.systemVersion') }}</div>
            <div class="text-xl font-bold mt-1 font-num">{{ systemInfo.version || '1.0.0' }}</div>
            <div class="text-xs text-muted-foreground mt-1">{{ systemInfo.os }}/{{ systemInfo.arch }} · {{ systemInfo.goVersion }}</div>
          </AcCard>
          <AcCard hoverable padding="lg" rounded="2xl">
            <div class="size-11 rounded-2xl bg-ac-leaf/30 flex items-center justify-center mb-3">
              <TimeOutline class="size-5 text-ac-leaf-dark" />
            </div>
            <div class="text-sm text-muted-foreground font-bold">{{ t('settings.details.serviceUptime') }}</div>
            <div class="text-xl font-bold mt-1 font-num">{{ systemInfo.uptime || t('settings.details.unknown') }}</div>
            <div class="text-xs text-muted-foreground mt-1">{{ t('settings.details.hostUptime', { time: systemInfo.hostUptime || '—' }) }}</div>
          </AcCard>
          <AcCard hoverable padding="lg" rounded="2xl">
            <div class="size-11 rounded-2xl bg-ac-sky/40 flex items-center justify-center mb-3">
              <PulseOutline class="size-5 text-ac-sky-dark" />
            </div>
            <div class="text-sm text-muted-foreground font-bold">Goroutines</div>
            <div class="text-xl font-bold mt-1 font-num">{{ systemInfo.goroutines ?? '—' }}</div>
            <div class="text-xs text-muted-foreground mt-1">{{ t('settings.details.goHeap', { size: fmtBytes(systemInfo.goMemory?.alloc), count: systemInfo.goMemory?.num_gc ?? 0 }) }}</div>
          </AcCard>
        </div>

        <!-- 资源使用率 -->
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <AcCard hoverable padding="lg" rounded="2xl">
            <div class="flex items-center gap-2 mb-2">
              <div class="size-9 rounded-xl bg-ac-sun/40 flex items-center justify-center">
                <HardwareChipOutline class="size-4 text-ac-sun-dark" />
              </div>
              <div class="text-sm text-muted-foreground font-bold flex-1">{{ t('settings.details.cpuUsage') }}</div>
              <div class="text-lg font-bold font-num">{{ fmtPct(systemInfo.cpuUsage) }}%</div>
            </div>
            <AcProgress :percent="clampPct(systemInfo.cpuUsage)" variant="sun" :show-text="false" />
            <div class="text-xs text-muted-foreground mt-1.5">{{ t('settings.details.cores', { count: systemInfo.cpuCores || '?' }) }}</div>
          </AcCard>
          <AcCard hoverable padding="lg" rounded="2xl">
            <div class="flex items-center gap-2 mb-2">
              <div class="size-9 rounded-xl bg-ac-sky/40 flex items-center justify-center">
                <ServerOutline class="size-4 text-ac-sky-dark" />
              </div>
              <div class="text-sm text-muted-foreground font-bold flex-1">{{ t('settings.details.memoryUsage') }}</div>
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
              <div class="text-sm text-muted-foreground font-bold flex-1">{{ t('settings.details.storageUsage') }}</div>
              <div class="text-lg font-bold font-num">{{ fmtPct(systemInfo.diskUsage) }}%</div>
            </div>
            <AcProgress :percent="clampPct(systemInfo.diskUsage)" variant="heart" :show-text="false" />
            <div class="text-xs text-muted-foreground mt-1.5">
              {{ fmtBytes(systemInfo.disk?.used) }} / {{ fmtBytes(systemInfo.disk?.total) }}
            </div>
            <div v-if="systemInfo.disk?.path" class="text-xs text-muted-foreground mt-1 font-num truncate" :title="systemInfo.disk.path">
              {{ systemInfo.disk.path }} · {{ t('settings.details.remaining', { size: fmtBytes(systemInfo.disk?.free) }) }}
            </div>
          </AcCard>
        </div>

        <!-- 依赖服务状态 -->
        <AcCard padding="lg" rounded="2xl">
          <h3 class="text-base font-bold tracking-tight text-foreground mb-3">{{ t('settings.details.dependencies') }}</h3>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <!-- 数据库 -->
            <div class="flex items-center justify-between p-3 rounded-2xl border-2 border-ac-sand">
              <div class="flex items-center gap-2.5">
                <span class="inline-block size-2.5 rounded-full" :class="systemInfo.database?.connected ? 'bg-ac-leaf' : 'bg-ac-heart'"></span>
                <div>
                  <div class="text-sm font-bold">PostgreSQL</div>
                  <div class="text-xs text-muted-foreground">
                    {{ systemInfo.database?.connected ? t('settings.details.connected') : t('settings.details.disconnected') }}
                    <template v-if="systemInfo.database?.connected">
                      · {{ t('settings.details.connectionUsage', { used: systemInfo.database.in_use, open: systemInfo.database.open }) }}
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
                    {{ systemInfo.qbittorrent?.online ? t('settings.details.online') : t('settings.details.offline') }}
                    <template v-if="systemInfo.qbittorrent?.version"> · {{ systemInfo.qbittorrent.version }}</template>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="text-xs text-muted-foreground mt-3">
            {{ t('settings.details.autoRefresh', { seconds: Math.round(SYS_REFRESH_MS / 1000), time: lastUpdated || '—' }) }}
          </div>
        </AcCard>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../../stores/auth'
import { useToast } from '../../composables/useToast'
import { get, put, post } from '../../utils/api'
import DownloadPrefs from './DownloadPrefs.vue'
import LocaleSwitcher from '../../components/Common/LocaleSwitcher.vue'
import {
  SaveOutline, PersonCircleOutline,
  CheckmarkOutline, CodeSlashOutline, TimeOutline,
  HardwareChipOutline, ServerOutline, CreateOutline,
  GlobeOutline, PulseOutline
} from '@vicons/ionicons5'
import { AcPageHeader, AcTabs, AcCard, AcButton, AcInput, AcSelect, AcSwitch, AcProgress } from '../../components/ac'
import { useSkin } from '../../composables/useSkin'

const { skin, setSkin, SKINS } = useSkin()
const { t } = useI18n({ useScope: 'global' })

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

const tabs = computed(() => [
  { key: 'download', label: t('settings.tabs.download') },
  { key: 'appearance', label: t('settings.tabs.appearance') },
  { key: 'rename', label: t('settings.tabs.rename') },
  { key: 'scheduler', label: t('settings.tabs.scheduler') },
  { key: 'network', label: t('settings.tabs.network') },
  { key: 'user', label: t('settings.tabs.user') },
  { key: 'system', label: t('settings.tabs.system') }
])

const saving = ref({ basic: false, user: false, scheduler: false, proxy: false })

const schedulerForm = reactive({ enabled: true })

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
    if (data.enable_scheduler !== undefined) schedulerForm.enabled = data.enable_scheduler
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

async function saveSchedulerSettings() {
  saving.value.scheduler = true
  try {
    await put('/settings', { enable_scheduler: String(schedulerForm.enabled) })
    toast.success(schedulerForm.enabled ? t('settings.details.schedulerOn') : t('settings.details.schedulerOff'))
  } catch (e) {
    toast.error(e.message || t('settings.details.schedulerSaveFailed'))
  } finally {
    saving.value.scheduler = false
  }
}

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
    toast.success(t('settings.details.proxySaved'))
  } catch (e) { toast.error(e?.message || t('settings.details.saveFailed')) }
  finally { saving.value.proxy = false }
}

async function saveUserSettings() {
  if (!userForm.value.oldPassword) { toast.error(t('settings.details.enterOldPassword')); return }
  if (!userForm.value.newPassword) { toast.error(t('settings.details.enterNewPassword')); return }
  if (userForm.value.newPassword !== userForm.value.confirmPassword) { toast.error(t('settings.details.passwordMismatch')); return }
  try {
    saving.value.user = true
    await put('/users/password', { old_password: userForm.value.oldPassword, new_password: userForm.value.newPassword })
    toast.success(t('settings.details.passwordChanged'))
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
