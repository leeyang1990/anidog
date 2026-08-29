<template>
  <div>
    <AcPageHeader :title="t('pages.rules.title')" :subtitle="t('pages.rules.subtitle')">
      <template #actions>
        <div class="flex gap-2">
          <AcButton variant="outline" @click="showImportModal = true">
            <template #icon><CloudUploadOutline class="size-4" /></template>
            {{ t('pages.rules.import') }}
          </AcButton>
          <AcButton variant="primary" @click="openCreateModal">
            <template #icon><AddOutline class="size-4" /></template>
            {{ t('pages.rules.add') }}
          </AcButton>
        </div>
      </template>
    </AcPageHeader>

    <div v-if="loading" class="flex justify-center py-12"><AcSpinner :size="48" /></div>

    <div v-else-if="rules.length" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <AcCard v-for="rule in rules" :key="rule.id" hoverable padding="lg" rounded="2xl">
        <div class="flex items-center justify-between mb-3">
          <div class="flex items-center gap-2">
            <div class="size-9 rounded-2xl bg-ac-grass-light/40 flex items-center justify-center">
              <GlobeOutline class="size-4 text-ac-grass-dark" />
            </div>
            <div>
              <h3 class="text-sm font-bold text-foreground">{{ rule.display_name || rule.name }}</h3>
              <p class="text-xs text-muted-foreground font-num">v{{ rule.version }}</p>
            </div>
          </div>
          <AcTag :variant="rule.enabled ? 'leaf' : 'wood'">
            {{ rule.enabled ? t('pages.rules.enabled') : t('pages.rules.disabled') }}
          </AcTag>
        </div>
        <p class="text-xs text-muted-foreground truncate mb-3 font-num">{{ rule.base_url }}</p>
        <div class="flex items-center gap-2">
          <AcButton size="sm" variant="ghost" @click="openEditModal(rule)">{{ t('pages.rules.edit') }}</AcButton>
          <AcButton size="sm" variant="ghost" @click="testRule(rule)">{{ t('pages.rules.test') }}</AcButton>
          <AcButton size="sm" variant="ghost" @click="deleteRule(rule)">
            <span class="text-ac-heart-dark">{{ t('pages.rules.delete') }}</span>
          </AcButton>
        </div>
      </AcCard>
    </div>

    <AcEmpty v-else :title="t('pages.rules.empty')" :description="t('pages.rules.emptyDesc')" class="py-12" />

    <!-- 添加/编辑规则弹窗 -->
    <AcModal v-model:show="showEditModal" :title="editingRule ? t('pages.rules.editTitle') : t('pages.rules.addTitle')" max-width="640px">
      <form @submit.prevent="handleSaveRule" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">{{ t('pages.rules.name') }}</label>
            <AcInput v-model="ruleForm.name" placeholder="如 nyafun" />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">{{ t('pages.rules.displayName') }}</label>
            <AcInput v-model="ruleForm.display_name" placeholder="如 NYA FUN动漫" />
          </div>
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">{{ t('pages.rules.siteUrl') }}</label>
          <AcInput v-model="ruleForm.base_url" placeholder="https://example.com" />
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">{{ t('pages.rules.searchUrl') }}</label>
          <AcInput v-model="ruleForm.search_url" placeholder="https://example.com/search?q=@keyword" />
          <p class="text-xs text-muted-foreground">{{ t('pages.rules.keywordHint') }}</p>
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">{{ t('pages.rules.searchListXpath') }}</label>
          <AcInput v-model="ruleForm.search_list_xpath" placeholder="//div[@class='search-list']" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">{{ t('pages.rules.titleXpath') }}</label>
            <AcInput v-model="ruleForm.search_name_xpath" placeholder="//a/h3" />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">{{ t('pages.rules.linkXpath') }}</label>
            <AcInput v-model="ruleForm.search_result_xpath" placeholder="//a" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">{{ t('pages.rules.roadXpath') }}</label>
            <AcInput v-model="ruleForm.chapter_roads_xpath" placeholder="//ul[@class='road']" />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">{{ t('pages.rules.episodeXpath') }}</label>
            <AcInput v-model="ruleForm.chapter_result_xpath" placeholder="//li/a" />
          </div>
        </div>
        <div class="flex items-center gap-6 flex-wrap">
          <label class="flex items-center gap-2 text-sm cursor-pointer">
            <AcCheckbox v-model="ruleForm.use_post" /> {{ t('pages.rules.postSearch') }}
          </label>
          <label class="flex items-center gap-2 text-sm cursor-pointer">
            <AcCheckbox v-model="ruleForm.use_webview" /> {{ t('pages.rules.jsRender') }}
          </label>
          <label class="flex items-center gap-2 text-sm cursor-pointer">
            <AcCheckbox v-model="ruleForm.multi_sources" /> {{ t('pages.rules.multiRoutes') }}
          </label>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-2">
          <AcButton variant="ghost" @click="showEditModal = false">{{ t('common.cancel') }}</AcButton>
          <AcButton variant="primary" :loading="saving" @click="handleSaveRule">{{ saving ? t('pages.rules.saving') : t('pages.rules.save') }}</AcButton>
        </div>
      </template>
    </AcModal>

    <!-- 导入弹窗 -->
    <AcModal v-model:show="showImportModal" :title="t('pages.rules.importTitle')" max-width="480px">
      <div class="space-y-4">
        <p class="text-sm text-muted-foreground">{{ t('pages.rules.importDesc') }}</p>
        <input type="file" accept=".json" ref="fileInput" @change="handleImport"
          class="block w-full text-sm font-num text-foreground file:mr-4 file:py-2 file:px-4 file:rounded-2xl file:border-0 file:bg-ac-grass file:text-white file:font-bold file:cursor-pointer hover:file:bg-ac-grass-dark" />
      </div>
    </AcModal>

    <!-- 测试弹窗 -->
    <AcModal v-model:show="showTestModal" :title="t('pages.rules.testTitle', { name: testingRule?.name || '' })" max-width="640px">
      <div class="space-y-4">
        <div class="flex gap-2">
          <div class="flex-1">
            <AcInput v-model="testKeyword" :placeholder="t('pages.rules.testPlaceholder')" @keyup-enter="executeTest" />
          </div>
          <AcButton variant="primary" :loading="testLoading" @click="executeTest">{{ t('pages.rules.test') }}</AcButton>
        </div>
        <div v-if="testLoading" class="flex justify-center py-6"><AcSpinner :size="32" /></div>
        <div v-else-if="testResults.length" class="space-y-2 max-h-[320px] overflow-y-auto">
          <div v-for="(r, i) in testResults" :key="i" class="p-3 rounded-2xl border-2 border-ac-sand hover:bg-ac-sand/30 transition-colors">
            <p class="text-sm font-bold">{{ r.name }}</p>
            <p class="text-xs text-muted-foreground truncate font-num">{{ r.url }}</p>
          </div>
        </div>
        <div v-else-if="testExecuted" class="text-center py-6 text-sm text-muted-foreground">{{ t('pages.rules.noResults') }}</div>
      </div>
    </AcModal>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { AddOutline, GlobeOutline, CloudUploadOutline } from '@vicons/ionicons5'
import { get, post, put, del } from '@/utils/api'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { AcPageHeader, AcButton, AcCard, AcTag, AcEmpty, AcSpinner, AcModal, AcInput, AcCheckbox } from '@/components/ac'
import { useI18n } from 'vue-i18n'

const toast = useToast()
const { confirm } = useConfirm()
const { t } = useI18n({ useScope: 'global' })
const loading = ref(false)
const saving = ref(false)
const rules = ref([])

const showEditModal = ref(false)
const editingRule = ref(null)
const ruleForm = ref(getEmptyForm())

const showImportModal = ref(false)
const fileInput = ref(null)

const showTestModal = ref(false)
const testingRule = ref(null)
const testKeyword = ref('')
const testResults = ref([])
const testLoading = ref(false)
const testExecuted = ref(false)

function getEmptyForm() {
  return {
    name: '', display_name: '', base_url: '', search_url: '',
    search_list_xpath: '', search_name_xpath: '', search_result_xpath: '',
    chapter_roads_xpath: '', chapter_result_xpath: '',
    use_post: false, use_webview: false, multi_sources: true,
  }
}

async function fetchRules() {
  loading.value = true
  try {
    const data = await get('/stream-rules')
    rules.value = Array.isArray(data) ? data : (data.items || [])
  } catch { toast.error(t('pages.rules.loadFailed')) }
  finally { loading.value = false }
}

function openCreateModal() {
  editingRule.value = null
  ruleForm.value = getEmptyForm()
  showEditModal.value = true
}

function openEditModal(rule) {
  editingRule.value = rule
  ruleForm.value = { ...rule }
  showEditModal.value = true
}

async function handleSaveRule() {
  saving.value = true
  try {
    if (editingRule.value) {
      await put(`/stream-rules/${editingRule.value.id}`, ruleForm.value)
      toast.success(t('pages.rules.updated'))
    } else {
      await post('/stream-rules', ruleForm.value)
      toast.success(t('pages.rules.created'))
    }
    showEditModal.value = false
    await fetchRules()
  } catch (e) { toast.error(e.message || t('pages.rules.saveFailed')) }
  finally { saving.value = false }
}

async function deleteRule(rule) {
  const ok = await confirm({ title: t('pages.rules.deleteTitle'), content: t('pages.rules.deleteConfirm', { name: rule.name }), variant: 'danger' })
  if (!ok) return
  try {
    await del(`/stream-rules/${rule.id}`)
    toast.success(t('pages.rules.deleted'))
    await fetchRules()
  } catch { toast.error(t('pages.rules.deleteFailed')) }
}

async function handleImport(e) {
  const file = e.target.files[0]
  if (!file) return
  const formData = new FormData()
  formData.append('file', file)
  try {
    const token = localStorage.getItem('token')
    const resp = await fetch('/api/v1/stream-rules/import', {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
      body: formData,
    })
    const respText = await resp.text()
    let data
    try { data = JSON.parse(respText) } catch { toast.error(t('pages.rules.parseFailed')); return }
    toast.success(t('pages.rules.imported', { count: data.imported }))
    showImportModal.value = false
    await fetchRules()
  } catch { toast.error(t('pages.rules.importFailed')) }
}

function testRule(rule) {
  testingRule.value = rule
  testKeyword.value = ''
  testResults.value = []
  testExecuted.value = false
  showTestModal.value = true
}

async function executeTest() {
  if (!testKeyword.value.trim() || !testingRule.value) return
  testLoading.value = true
  testExecuted.value = false
  try {
    const data = await post(`/stream-rules/${testingRule.value.id}/test`, { keyword: testKeyword.value })
    testResults.value = data.results || []
    testExecuted.value = true
  } catch (e) { toast.error(t('pages.rules.testFailed', { message: e.message || '' })) }
  finally { testLoading.value = false }
}

onMounted(fetchRules)
</script>
