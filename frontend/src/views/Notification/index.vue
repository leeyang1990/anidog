<template>
  <div>
    <PageHeader title="通知设置" subtitle="配置番剧更新通知渠道">
      <template #actions>
        <button class="inline-flex items-center gap-1.5 bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-9 px-4 text-sm font-medium transition-colors" @click="openAddModal">
          <n-icon size="16"><AddOutline /></n-icon> 添加渠道
        </button>
      </template>
    </PageHeader>

    <n-spin :show="loading">
      <div v-if="channels.length" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div v-for="ch in channels" :key="ch.id" class="bg-card rounded-lg border p-6 hover:shadow-md transition-shadow">
          <div class="flex items-center gap-3">
            <div class="h-9 w-9 rounded-md bg-primary/10 flex items-center justify-center text-primary shrink-0">
              <n-icon size="18"><NotificationsOutline /></n-icon>
            </div>
            <div class="flex-1 min-w-0">
              <h3 class="text-sm font-medium truncate">{{ ch.name }}</h3>
              <span class="text-xs" :class="ch.enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-muted-foreground'">
                {{ ch.enabled ? '已启用' : '已禁用' }}
              </span>
            </div>
            <n-dropdown :options="channelActions" @select="key => handleAction(key, ch)" placement="bottom-end">
              <button class="p-1.5 rounded-md hover:bg-accent transition-colors" @click.stop>
                <n-icon size="16" class="text-muted-foreground"><EllipsisVerticalOutline /></n-icon>
              </button>
            </n-dropdown>
          </div>
          <div class="mt-3">
            <span class="inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium bg-secondary text-secondary-foreground">{{ getTypeLabel(ch.type) }}</span>
          </div>
          <div class="mt-4 flex gap-2">
            <button class="inline-flex items-center gap-1.5 rounded-md border border-input bg-background px-3 py-1.5 text-xs font-medium hover:bg-accent transition-colors" @click="testChannel(ch)" :disabled="testingId === ch.id">
              <n-icon size="14"><FlaskOutline /></n-icon>
              {{ testingId === ch.id ? '测试中...' : '测试' }}
            </button>
          </div>
        </div>
      </div>
      <div v-else class="text-center py-12">
        <n-icon size="48" class="text-muted-foreground/30"><NotificationsOutline /></n-icon>
        <p class="mt-4 text-sm text-muted-foreground">暂无通知渠道</p>
      </div>
    </n-spin>

    <!-- 添加/编辑弹窗 -->
    <n-modal v-model:show="showModal" preset="card" style="width: 560px; max-width: 90vw" :bordered="false">
      <template #header>
        <span class="text-base font-semibold">{{ editingChannel ? '编辑渠道' : '添加渠道' }}</span>
      </template>
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium mb-1.5">渠道名称</label>
          <input v-model="formValue.name" type="text" required placeholder="例如：我的Telegram" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1.5">渠道类型</label>
          <select v-model="formValue.type" required class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm">
            <option v-for="opt in typeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
        </div>
        <div class="flex items-center justify-between">
          <label class="text-sm font-medium">启用</label>
          <button type="button" role="switch" :aria-checked="formValue.enabled"
            class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors"
            :class="formValue.enabled ? 'bg-primary' : 'bg-input'"
            @click="formValue.enabled = !formValue.enabled">
            <span class="pointer-events-none block h-5 w-5 rounded-full bg-background shadow-lg ring-0 transition-transform"
              :class="formValue.enabled ? 'translate-x-5' : 'translate-x-0'" />
          </button>
        </div>

        <template v-if="formValue.type === 'telegram'">
          <div>
            <label class="block text-sm font-medium mb-1.5">Bot Token</label>
            <input v-model="formConfig.token" type="text" placeholder="123456:ABC-DEF..." class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1.5">Chat ID</label>
            <input v-model="formConfig.chat_id" type="text" placeholder="-1001234567890" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </template>

        <template v-if="formValue.type === 'bark'">
          <div>
            <label class="block text-sm font-medium mb-1.5">Bark URL</label>
            <input v-model="formConfig.url" type="text" placeholder="https://api.day.app" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1.5">Device Key</label>
            <input v-model="formConfig.key" type="text" placeholder="你的设备Key" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </template>

        <template v-if="formValue.type === 'webhook'">
          <div>
            <label class="block text-sm font-medium mb-1.5">Webhook URL</label>
            <input v-model="formConfig.url" type="text" placeholder="https://example.com/webhook" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </template>

        <template v-if="formValue.type === 'discord'">
          <div>
            <label class="block text-sm font-medium mb-1.5">Webhook URL</label>
            <input v-model="formConfig.url" type="text" placeholder="https://discord.com/api/webhooks/..." class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </template>

        <template v-if="formValue.type === 'server_chan'">
          <div>
            <label class="block text-sm font-medium mb-1.5">SendKey</label>
            <input v-model="formConfig.sendkey" type="text" placeholder="你的SendKey" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </template>

        <template v-if="formValue.type === 'wecom'">
          <div>
            <label class="block text-sm font-medium mb-1.5">企业ID (corpid)</label>
            <input v-model="formConfig.corpid" type="text" placeholder="ww1234567890" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1.5">应用Secret</label>
            <input v-model="formConfig.corpsecret" type="text" placeholder="应用的Secret" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1.5">应用AgentId</label>
            <input v-model="formConfig.agentid" type="text" placeholder="1000002" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </template>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2 pt-4">
          <button class="border border-input bg-background hover:bg-accent rounded-md h-9 px-4 text-sm font-medium transition-colors" @click="showModal = false">取消</button>
          <button class="bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-9 px-4 text-sm font-medium transition-colors" @click="handleSubmit" :disabled="submitting">{{ editingChannel ? '保存' : '添加' }}</button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, h } from 'vue'
import { useMessage, NIcon, NDropdown, NModal, NSpin } from 'naive-ui'
import { get, post, put, del } from '@/utils/api'
import PageHeader from '@/components/Common/PageHeader.vue'
import {
  AddOutline, NotificationsOutline, EllipsisVerticalOutline,
  FlaskOutline, CreateOutline, TrashOutline
} from '@vicons/ionicons5'

const message = useMessage()

const loading = ref(false)
const testingId = ref(null)
const submitting = ref(false)
const channels = ref([])
const showModal = ref(false)
const editingChannel = ref(null)

const formValue = reactive({
  name: '',
  type: 'telegram',
  enabled: true
})

const formConfig = reactive({})

const typeOptions = [
  { label: 'Telegram', value: 'telegram' },
  { label: 'Bark', value: 'bark' },
  { label: 'Webhook', value: 'webhook' },
  { label: 'Discord', value: 'discord' },
  { label: 'Server Chan', value: 'server_chan' },
  { label: '企业微信', value: 'wecom' },
]

const channelActions = [
  { label: '编辑', key: 'edit', icon: () => h(NIcon, null, { default: () => h(CreateOutline) }) },
  { label: '删除', key: 'delete', icon: () => h(NIcon, null, { default: () => h(TrashOutline) }) }
]

function getTypeLabel(type) {
  return typeOptions.find(t => t.value === type)?.label || type
}

function openAddModal() {
  editingChannel.value = null
  Object.assign(formValue, { name: '', type: 'telegram', enabled: true })
  Object.keys(formConfig).forEach(k => delete formConfig[k])
  showModal.value = true
}

function openEditModal(ch) {
  editingChannel.value = ch
  Object.assign(formValue, { name: ch.name, type: ch.type, enabled: ch.enabled })
  try {
    const config = JSON.parse(ch.config || '{}')
    Object.keys(formConfig).forEach(k => delete formConfig[k])
    Object.assign(formConfig, config)
  } catch { Object.keys(formConfig).forEach(k => delete formConfig[k]) }
  showModal.value = true
}

async function fetchChannels() {
  loading.value = true
  try {
    const data = await get('/notifications')
    // 后端 GET /notifications 直接返回数组（不是 { items: [] } 包装）
    // 兼容两种形态：直接数组 / { items } 包装
    if (Array.isArray(data)) {
      channels.value = data
    } else {
      channels.value = data?.items || []
    }
  } catch { message.error('获取通知渠道失败') }
  finally { loading.value = false }
}

async function handleSubmit() {
  if (!formValue.name) {
    message.warning('请输入渠道名称')
    return
  }
  submitting.value = true
  try {
    const payload = {
      ...formValue,
      config: JSON.stringify({ ...formConfig })
    }
    if (editingChannel.value) {
      await put(`/notifications/${editingChannel.value.id}`, payload)
      message.success('更新成功')
    } else {
      await post('/notifications', payload)
      message.success('添加成功')
    }
    showModal.value = false
    await fetchChannels()
  } catch (e) {
    if (e.message) message.error(e.message)
  } finally { submitting.value = false }
}

async function testChannel(ch) {
  testingId.value = ch.id
  try {
    const data = await post(`/notifications/${ch.id}/test`)
    if (data.success) message.success('测试通知发送成功')
    else message.error(data.message || '测试失败')
  } catch { message.error('测试失败') }
  finally { testingId.value = null }
}

function handleAction(key, ch) {
  if (key === 'edit') openEditModal(ch)
  else if (key === 'delete') deleteChannel(ch)
}

async function deleteChannel(ch) {
  try {
    await del(`/notifications/${ch.id}`)
    message.success('已删除')
    await fetchChannels()
  } catch { message.error('删除失败') }
}

onMounted(fetchChannels)
</script>
