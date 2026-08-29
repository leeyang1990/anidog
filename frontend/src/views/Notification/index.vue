<template>
  <div>
    <AcPageHeader :title="t('pages.notifications.title')" :subtitle="t('pages.notifications.subtitle')">
      <template #actions>
        <AcButton variant="primary" @click="openAddModal">
          <template #icon><AddOutline class="size-4" /></template>
          {{ t('pages.notifications.addChannel') }}
        </AcButton>
      </template>
    </AcPageHeader>

    <div v-if="loading" class="flex justify-center py-12"><AcSpinner :size="48" /></div>

    <div v-else-if="channels.length" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <AcCard v-for="ch in channels" :key="ch.id" hoverable padding="lg" rounded="2xl">
        <div class="flex items-center gap-3">
          <div class="size-10 rounded-2xl bg-ac-grass-light/40 flex items-center justify-center text-ac-grass-dark shrink-0">
            <NotificationsOutline class="size-5" />
          </div>
          <div class="flex-1 min-w-0">
            <h3 class="text-sm font-bold truncate text-foreground">{{ ch.name }}</h3>
            <span class="text-xs font-bold" :class="ch.enabled ? 'text-ac-leaf-dark' : 'text-muted-foreground'">
              {{ ch.enabled ? t('common.enabled') : t('common.disabled') }}
            </span>
          </div>
          <AcDropdown :options="channelActions" placement="bottom-end" @select="key => handleAction(key, ch)">
            <template #trigger>
              <button type="button" class="p-2 rounded-2xl hover:bg-ac-sand/60 transition-colors">
                <EllipsisVerticalOutline class="size-4 text-muted-foreground" />
              </button>
            </template>
          </AcDropdown>
        </div>
        <div class="mt-3">
          <AcTag variant="wood">{{ getTypeLabel(ch.type) }}</AcTag>
        </div>
        <div class="mt-4 flex gap-2">
          <AcButton size="sm" variant="outline" :loading="testingId === ch.id" @click="testChannel(ch)">
            <template #icon><FlaskOutline class="size-3.5" /></template>
            {{ testingId === ch.id ? t('pages.notifications.testing') : t('pages.notifications.test') }}
          </AcButton>
        </div>
      </AcCard>
    </div>

    <AcEmpty v-else :title="t('pages.notifications.empty')" :description="t('pages.notifications.emptyDesc')" class="py-12" />

    <!-- 添加/编辑弹窗 -->
    <AcModal v-model:show="showModal" :title="editingChannel ? t('pages.notifications.editTitle') : t('pages.notifications.addTitle')" max-width="560px">
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-bold mb-1.5 text-foreground">{{ t('pages.notifications.name') }}</label>
          <AcInput v-model="formValue.name" :placeholder="t('pages.notifications.namePlaceholder')" />
        </div>
        <div>
          <label class="block text-sm font-bold mb-1.5 text-foreground">{{ t('pages.notifications.type') }}</label>
          <AcSelect v-model="formValue.type" :options="typeOptions" />
        </div>
        <div class="flex items-center justify-between">
          <label class="text-sm font-bold text-foreground">{{ t('pages.notifications.enable') }}</label>
          <AcSwitch v-model="formValue.enabled" />
        </div>

        <template v-if="formValue.type === 'telegram'">
          <div>
            <label class="block text-sm font-bold mb-1.5 text-foreground">Bot Token</label>
            <AcInput v-model="formConfig.token" placeholder="123456:ABC-DEF..." />
          </div>
          <div>
            <label class="block text-sm font-bold mb-1.5 text-foreground">Chat ID</label>
            <AcInput v-model="formConfig.chat_id" placeholder="-1001234567890" />
          </div>
        </template>

        <template v-if="formValue.type === 'bark'">
          <div>
            <label class="block text-sm font-bold mb-1.5 text-foreground">Bark URL</label>
            <AcInput v-model="formConfig.url" placeholder="https://api.day.app" />
          </div>
          <div>
            <label class="block text-sm font-bold mb-1.5 text-foreground">Device Key</label>
            <AcInput v-model="formConfig.key" :placeholder="t('pages.notifications.deviceKeyPlaceholder')" />
          </div>
        </template>

        <template v-if="formValue.type === 'webhook'">
          <div>
            <label class="block text-sm font-bold mb-1.5 text-foreground">Webhook URL</label>
            <AcInput v-model="formConfig.url" placeholder="https://example.com/webhook" />
          </div>
        </template>

        <template v-if="formValue.type === 'discord'">
          <div>
            <label class="block text-sm font-bold mb-1.5 text-foreground">Webhook URL</label>
            <AcInput v-model="formConfig.url" placeholder="https://discord.com/api/webhooks/..." />
          </div>
        </template>

        <template v-if="formValue.type === 'server_chan'">
          <div>
            <label class="block text-sm font-bold mb-1.5 text-foreground">SendKey</label>
            <AcInput v-model="formConfig.sendkey" :placeholder="t('pages.notifications.sendKeyPlaceholder')" />
          </div>
        </template>

        <template v-if="formValue.type === 'wecom'">
          <div>
            <label class="block text-sm font-bold mb-1.5 text-foreground">{{ t('pages.notifications.corpId') }}</label>
            <AcInput v-model="formConfig.corpid" placeholder="ww1234567890" />
          </div>
          <div>
            <label class="block text-sm font-bold mb-1.5 text-foreground">{{ t('pages.notifications.secret') }}</label>
            <AcInput v-model="formConfig.corpsecret" :placeholder="t('pages.notifications.secretPlaceholder')" />
          </div>
          <div>
            <label class="block text-sm font-bold mb-1.5 text-foreground">{{ t('pages.notifications.agentId') }}</label>
            <AcInput v-model="formConfig.agentid" placeholder="1000002" />
          </div>
        </template>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <AcButton variant="ghost" @click="showModal = false">{{ t('common.cancel') }}</AcButton>
          <AcButton variant="primary" :loading="submitting" @click="handleSubmit">{{ editingChannel ? t('pages.notifications.save') : t('pages.notifications.add') }}</AcButton>
        </div>
      </template>
    </AcModal>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { get, post, put, del } from '@/utils/api'
import { useToast } from '@/composables/useToast'
import {
  AddOutline, NotificationsOutline, EllipsisVerticalOutline,
  FlaskOutline, CreateOutline, TrashOutline
} from '@vicons/ionicons5'
import { AcPageHeader, AcButton, AcCard, AcTag, AcEmpty, AcSpinner, AcModal, AcInput, AcSelect, AcSwitch, AcDropdown } from '@/components/ac'
import { useI18n } from 'vue-i18n'

const toast = useToast()
const { t } = useI18n({ useScope: 'global' })

const loading = ref(false)
const testingId = ref(null)
const submitting = ref(false)
const channels = ref([])
const showModal = ref(false)
const editingChannel = ref(null)

const formValue = reactive({ name: '', type: 'telegram', enabled: true })
const formConfig = reactive({})

const typeOptions = computed(() => [
  { label: 'Telegram', value: 'telegram' },
  { label: 'Bark', value: 'bark' },
  { label: 'Webhook', value: 'webhook' },
  { label: 'Discord', value: 'discord' },
  { label: 'Server Chan', value: 'server_chan' },
  { label: t('pages.notifications.wecom'), value: 'wecom' },
])

const channelActions = computed(() => [
  { label: t('pages.notifications.edit'), key: 'edit' },
  { label: t('pages.notifications.delete'), key: 'delete', danger: true }
])

function getTypeLabel(type) { return typeOptions.value.find(option => option.value === type)?.label || type }

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
    channels.value = Array.isArray(data) ? data : (data?.items || [])
  } catch { toast.error(t('pages.notifications.loadFailed')) }
  finally { loading.value = false }
}

async function handleSubmit() {
  if (!formValue.name) { toast.warning(t('pages.notifications.enterName')); return }
  submitting.value = true
  try {
    const payload = { ...formValue, config: JSON.stringify({ ...formConfig }) }
    if (editingChannel.value) {
      await put(`/notifications/${editingChannel.value.id}`, payload)
      toast.success(t('pages.notifications.updated'))
    } else {
      await post('/notifications', payload)
      toast.success(t('pages.notifications.added'))
    }
    showModal.value = false
    await fetchChannels()
  } catch (e) { if (e.message) toast.error(e.message) }
  finally { submitting.value = false }
}

async function testChannel(ch) {
  testingId.value = ch.id
  try {
    const data = await post(`/notifications/${ch.id}/test`)
    if (data.success) toast.success(t('pages.notifications.testSuccess'))
    else toast.error(data.message || t('pages.notifications.testFailed'))
  } catch { toast.error(t('pages.notifications.testFailed')) }
  finally { testingId.value = null }
}

function handleAction(key, ch) {
  if (key === 'edit') openEditModal(ch)
  else if (key === 'delete') deleteChannel(ch)
}

async function deleteChannel(ch) {
  try {
    await del(`/notifications/${ch.id}`)
    toast.success(t('pages.notifications.deleted'))
    await fetchChannels()
  } catch { toast.error(t('pages.notifications.deleteFailed')) }
}

onMounted(fetchChannels)
</script>
