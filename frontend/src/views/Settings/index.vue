<template>
  <div>
    <PageHeader title="系统设置" subtitle="配置您的番剧管理系统偏好设置" />

    <!-- Tabs -->
    <nav class="flex border-b mb-6">
      <button v-for="tab in tabs" :key="tab.key"
        class="px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors"
        :class="activeTab === tab.key ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'"
        @click="activeTab = tab.key"
      >{{ tab.label }}</button>
    </nav>

    <!-- 下载偏好 Tab -->
    <div v-if="activeTab === 'download'">
      <DownloadPrefs />
    </div>

    <!-- 重命名设置 -->
    <div v-if="activeTab === 'rename'" class="space-y-6">
      <div class="bg-muted/50 rounded-lg p-6">
        <div class="flex gap-5">
          <div class="h-10 w-10 shrink-0 rounded-md bg-primary/10 flex items-center justify-center">
            <n-icon size="22"><CreateOutline /></n-icon>
          </div>
          <div class="flex-1 space-y-4">
            <div>
              <h3 class="text-lg font-semibold tracking-tight">文件重命名</h3>
              <p class="text-sm text-muted-foreground">配置下载文件的自动重命名规则</p>
            </div>
            <div class="space-y-4">
              <div class="space-y-2">
                <label class="text-sm font-medium">重命名方式</label>
                <select v-model="renameForm.rename_method"
                  class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring">
                  <option v-for="opt in renameOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
                </select>
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium">重命名示例</label>
                <input :value="renameExample" readonly
                  class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring opacity-70" />
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium">扫描间隔(秒)</label>
                <input v-model.number="renameForm.rename_interval" type="number" min="30" max="3600"
                  class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
              </div>
              <button :disabled="saving.rename" @click="saveRenameSettings"
                class="bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-10 px-6 text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2">
                <n-icon v-if="!saving.rename"><SaveOutline /></n-icon>
                <svg v-else class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
                保存设置
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 调度器设置 -->
    <div v-if="activeTab === 'scheduler'" class="space-y-6">
      <div class="bg-muted/50 rounded-lg p-6">
        <div class="flex gap-5">
          <div class="h-10 w-10 shrink-0 rounded-md bg-primary/10 flex items-center justify-center">
            <n-icon size="22"><TimeOutline /></n-icon>
          </div>
          <div class="flex-1 space-y-4">
            <div>
              <h3 class="text-lg font-semibold tracking-tight">后台调度</h3>
              <p class="text-sm text-muted-foreground">配置定时任务和自动刷新</p>
            </div>
            <div class="space-y-4">
              <div class="flex items-center justify-between">
                <div>
                  <label class="text-sm font-medium">启用调度器</label>
                  <p class="text-sm text-muted-foreground">{{ schedulerForm.enabled ? '已启用' : '已禁用' }}</p>
                </div>
                <button type="button" role="switch" :aria-checked="schedulerForm.enabled"
                  class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors"
                  :class="schedulerForm.enabled ? 'bg-primary' : 'bg-input'"
                  @click="schedulerForm.enabled = !schedulerForm.enabled">
                  <span class="pointer-events-none block h-5 w-5 rounded-full bg-background shadow-lg ring-0 transition-transform"
                    :class="schedulerForm.enabled ? 'translate-x-5' : 'translate-x-0'" />
                </button>
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium">RSS刷新间隔(分钟)</label>
                <input v-model.number="schedulerForm.rss_interval" type="number" min="5" max="1440"
                  class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium">语言偏好</label>
                <select v-model="schedulerForm.language"
                  class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring">
                  <option v-for="opt in languageOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
                </select>
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium">HTTP代理</label>
                <input v-model="schedulerForm.http_proxy" type="text"
                  placeholder="socks5://127.0.0.1:1080 (留空不使用代理)"
                  class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
              </div>
              <button :disabled="saving.scheduler" @click="saveSchedulerSettings"
                class="bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-10 px-6 text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2">
                <n-icon v-if="!saving.scheduler"><SaveOutline /></n-icon>
                <svg v-else class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
                保存设置
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 用户设置 -->
    <div v-if="activeTab === 'user'" class="space-y-6">
      <div class="bg-muted/50 rounded-lg p-6">
        <div class="flex gap-5">
          <div class="h-10 w-10 shrink-0 rounded-md bg-primary/10 flex items-center justify-center">
            <n-icon size="22"><PersonCircleOutline /></n-icon>
          </div>
          <div class="flex-1 space-y-4">
            <div>
              <h3 class="text-lg font-semibold tracking-tight">账户信息</h3>
              <p class="text-sm text-muted-foreground">修改密码和个人信息</p>
            </div>
            <div class="space-y-4">
              <div class="space-y-2">
                <label class="text-sm font-medium">用户名</label>
                <input :value="userForm.username" disabled
                  class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring opacity-70 cursor-not-allowed" />
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium">旧密码</label>
                <input v-model="userForm.oldPassword" type="password" required
                  placeholder="请输入旧密码"
                  class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium">新密码</label>
                <input v-model="userForm.newPassword" type="password" required
                  placeholder="请输入新密码"
                  class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium">确认新密码</label>
                <input v-model="userForm.confirmPassword" type="password" required
                  placeholder="请再次输入新密码"
                  class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
              </div>
              <button :disabled="saving.user" @click="saveUserSettings"
                class="bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-10 px-6 text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2">
                <n-icon v-if="!saving.user"><CheckmarkOutline /></n-icon>
                <svg v-else class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
                修改密码
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 系统信息 -->
    <div v-if="activeTab === 'system'" class="space-y-6">
      <div class="grid grid-cols-2 lg:grid-cols-3 gap-4">
        <div class="bg-card text-card-foreground rounded-lg border p-6 hover:shadow-md transition-shadow">
          <div class="h-10 w-10 rounded-md bg-primary/10 flex items-center justify-center mb-3">
            <n-icon size="22"><CodeSlashOutline /></n-icon>
          </div>
          <div class="text-sm text-muted-foreground font-medium">系统版本</div>
          <div class="text-xl font-bold mt-1">{{ systemInfo.version || '1.0.0' }}</div>
        </div>
        <div class="bg-card text-card-foreground rounded-lg border p-6 hover:shadow-md transition-shadow">
          <div class="h-10 w-10 rounded-md bg-green-500/10 flex items-center justify-center mb-3">
            <n-icon size="22"><TimeOutline /></n-icon>
          </div>
          <div class="text-sm text-muted-foreground font-medium">运行时间</div>
          <div class="text-xl font-bold mt-1">{{ systemInfo.uptime || '未知' }}</div>
        </div>
        <div class="bg-card text-card-foreground rounded-lg border p-6 hover:shadow-md transition-shadow">
          <div class="h-10 w-10 rounded-md bg-amber-500/10 flex items-center justify-center mb-3">
            <n-icon size="22"><HardwareChipOutline /></n-icon>
          </div>
          <div class="flex items-baseline justify-between mb-2">
            <div class="text-sm text-muted-foreground font-medium">CPU 使用率</div>
            <div class="text-xl font-bold">{{ systemInfo.cpuUsage || '0' }}%</div>
          </div>
          <div class="h-2 rounded-full bg-muted">
            <div class="h-full rounded-full bg-amber-500 transition-all" :style="{ width: (parseFloat(systemInfo.cpuUsage) || 0) + '%' }" />
          </div>
        </div>
        <div class="bg-card text-card-foreground rounded-lg border p-6 hover:shadow-md transition-shadow">
          <div class="h-10 w-10 rounded-md bg-blue-500/10 flex items-center justify-center mb-3">
            <n-icon size="22"><ServerOutline /></n-icon>
          </div>
          <div class="flex items-baseline justify-between mb-2">
            <div class="text-sm text-muted-foreground font-medium">内存使用率</div>
            <div class="text-xl font-bold">{{ systemInfo.memoryUsage || '0' }}%</div>
          </div>
          <div class="h-2 rounded-full bg-muted">
            <div class="h-full rounded-full bg-blue-500 transition-all" :style="{ width: (parseFloat(systemInfo.memoryUsage) || 0) + '%' }" />
          </div>
        </div>
        <div class="bg-card text-card-foreground rounded-lg border p-6 hover:shadow-md transition-shadow">
          <div class="h-10 w-10 rounded-md bg-violet-500/10 flex items-center justify-center mb-3">
            <n-icon size="22"><ServerOutline /></n-icon>
          </div>
          <div class="flex items-baseline justify-between mb-2">
            <div class="text-sm text-muted-foreground font-medium">磁盘使用率</div>
            <div class="text-xl font-bold">{{ systemInfo.diskUsage || '0' }}%</div>
          </div>
          <div class="h-2 rounded-full bg-muted">
            <div class="h-full rounded-full bg-violet-500 transition-all" :style="{ width: (parseFloat(systemInfo.diskUsage) || 0) + '%' }" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { NIcon } from 'naive-ui'
import { useAuthStore } from '../../stores/auth'
import { get, put } from '../../utils/api'
import PageHeader from '@/components/Common/PageHeader.vue'
import DownloadPrefs from './DownloadPrefs.vue'
import {
  FolderOpenOutline, SaveOutline,
  PersonCircleOutline,
  CheckmarkOutline, CodeSlashOutline, TimeOutline,
  HardwareChipOutline, ServerOutline, CreateOutline
} from '@vicons/ionicons5'

const message = useMessage()
const authStore = useAuthStore()

const activeTab = ref('download')

const tabs = [
  { key: 'download', label: '下载偏好' },
  { key: 'rename', label: '重命名设置' },
  { key: 'scheduler', label: '调度器' },
  { key: 'user', label: '用户设置' },
  { key: 'system', label: '系统信息' }
]

const saving = ref({ basic: false, user: false, rename: false, scheduler: false })

const basicForm = ref({ downloadDir: '', maxConcurrent: 3 })

const renameForm = reactive({
  rename_method: 'pn',
  rename_interval: 300
})

const renameOptions = [
  { label: '不重命名 (none)', value: 'none' },
  { label: '标准命名: 标题 S01E01.mkv (pn)', value: 'pn' },
  { label: '高级命名: 官方标题 S01E01.mkv (advance)', value: 'advance' },
  { label: '字幕标准: 标题 S01E01.zh.srt (subtitle_pn)', value: 'subtitle_pn' },
  { label: '字幕高级: 官方标题 S01E01.zh.srt (subtitle_advance)', value: 'subtitle_advance' }
]

const renameExample = computed(() => {
  const examples = {
    none: '保持原文件名',
    pn: '葬送的芙莉莲 S01E01.mkv',
    advance: 'Sousou no Frieren S01E01.mkv',
    subtitle_pn: '葬送的芙莉莲 S01E01.zh.srt',
    subtitle_advance: 'Sousou no Frieren S01E01.zh.srt'
  }
  return examples[renameForm.rename_method] || ''
})

const schedulerForm = reactive({
  enabled: true,
  rss_interval: 30,
  language: 'zh',
  http_proxy: ''
})

const languageOptions = [
  { label: '中文', value: 'zh' },
  { label: '日本語', value: 'ja' },
  { label: 'English', value: 'en' }
]

const userForm = ref({
  username: authStore.user?.username || '',
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const systemInfo = ref({ version: '', uptime: '', cpuUsage: 0, memoryUsage: 0, diskUsage: 0 })

async function fetchSettings() {
  try {
    const data = await get('/settings')
    basicForm.value = { downloadDir: data.downloadDir, maxConcurrent: data.maxConcurrent }
    if (data.rename_method) renameForm.rename_method = data.rename_method
    if (data.rename_interval) renameForm.rename_interval = data.rename_interval
    if (data.enable_scheduler !== undefined) schedulerForm.enabled = data.enable_scheduler
    if (data.rss_check_interval) schedulerForm.rss_interval = data.rss_check_interval
    if (data.language) schedulerForm.language = data.language
    if (data.http_proxy) schedulerForm.http_proxy = data.http_proxy
  } catch (e) {
    console.error('获取设置失败:', e)
  }
}

async function fetchSystemInfo() {
  try {
    const data = await get('/system/info')
    systemInfo.value = data
  } catch (e) {
    console.error('获取系统信息失败:', e)
  }
}

async function saveBasicSettings() {
  message.warning('设置暂不支持在线修改，请修改配置文件后重启服务')
}

async function saveRenameSettings() {
  message.warning('设置暂不支持在线修改，请修改配置文件后重启服务')
}

async function saveSchedulerSettings() {
  message.warning('设置暂不支持在线修改，请修改配置文件后重启服务')
}

async function saveUserSettings() {
  if (!userForm.value.oldPassword) {
    message.error('请输入旧密码')
    return
  }
  if (!userForm.value.newPassword) {
    message.error('请输入新密码')
    return
  }
  if (userForm.value.newPassword !== userForm.value.confirmPassword) {
    message.error('两次输入的密码不一致')
    return
  }
  try {
    saving.value.user = true
    await put('/users/password', { old_password: userForm.value.oldPassword, new_password: userForm.value.newPassword })
    message.success('密码修改成功')
    userForm.value.oldPassword = ''
    userForm.value.newPassword = ''
    userForm.value.confirmPassword = ''
  } catch (e) {
    if (e?.message) message.error(e.message)
  } finally { saving.value.user = false }
}

onMounted(() => { fetchSettings(); fetchSystemInfo() })
</script>
