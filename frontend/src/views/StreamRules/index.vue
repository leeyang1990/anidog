<template>
  <div>
    <AcPageHeader title="🌐 规则管理" subtitle="管理番剧源解析规则（兼容 Kazumi 格式）">
      <template #actions>
        <div class="flex gap-2">
          <AcButton variant="outline" @click="showImportModal = true">
            <template #icon><CloudUploadOutline class="size-4" /></template>
            导入
          </AcButton>
          <AcButton variant="primary" @click="openCreateModal">
            <template #icon><AddOutline class="size-4" /></template>
            添加规则
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
            {{ rule.enabled ? '启用' : '禁用' }}
          </AcTag>
        </div>
        <p class="text-xs text-muted-foreground truncate mb-3 font-num">{{ rule.base_url }}</p>
        <div class="flex items-center gap-2">
          <AcButton size="sm" variant="ghost" @click="openEditModal(rule)">编辑</AcButton>
          <AcButton size="sm" variant="ghost" @click="testRule(rule)">测试</AcButton>
          <AcButton size="sm" variant="ghost" @click="deleteRule(rule)">
            <span class="text-ac-heart-dark">删除</span>
          </AcButton>
        </div>
      </AcCard>
    </div>

    <AcEmpty v-else title="暂无规则" description="请添加或导入规则 🌱" class="py-12" />

    <!-- 添加/编辑规则弹窗 -->
    <AcModal v-model:show="showEditModal" :title="editingRule ? '编辑规则' : '添加规则'" max-width="640px">
      <form @submit.prevent="handleSaveRule" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">规则名称 *</label>
            <AcInput v-model="ruleForm.name" placeholder="如 nyafun" />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">显示名称</label>
            <AcInput v-model="ruleForm.display_name" placeholder="如 NYA FUN动漫" />
          </div>
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">站点 URL *</label>
          <AcInput v-model="ruleForm.base_url" placeholder="https://example.com" />
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">搜索 URL *</label>
          <AcInput v-model="ruleForm.search_url" placeholder="https://example.com/search?q=@keyword" />
          <p class="text-xs text-muted-foreground">@keyword 会被替换为搜索词</p>
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">搜索列表 XPath *</label>
          <AcInput v-model="ruleForm.search_list_xpath" placeholder="//div[@class='search-list']" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">标题 XPath *</label>
            <AcInput v-model="ruleForm.search_name_xpath" placeholder="//a/h3" />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">链接 XPath *</label>
            <AcInput v-model="ruleForm.search_result_xpath" placeholder="//a" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">线路 XPath</label>
            <AcInput v-model="ruleForm.chapter_roads_xpath" placeholder="//ul[@class='road']" />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-bold text-foreground">集数 XPath *</label>
            <AcInput v-model="ruleForm.chapter_result_xpath" placeholder="//li/a" />
          </div>
        </div>
        <div class="flex items-center gap-6 flex-wrap">
          <label class="flex items-center gap-2 text-sm cursor-pointer">
            <AcCheckbox v-model="ruleForm.use_post" /> POST 搜索
          </label>
          <label class="flex items-center gap-2 text-sm cursor-pointer">
            <AcCheckbox v-model="ruleForm.use_webview" /> 需要 JS 渲染
          </label>
          <label class="flex items-center gap-2 text-sm cursor-pointer">
            <AcCheckbox v-model="ruleForm.multi_sources" /> 多线路
          </label>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-2">
          <AcButton variant="ghost" @click="showEditModal = false">取消</AcButton>
          <AcButton variant="primary" :loading="saving" @click="handleSaveRule">{{ saving ? '保存中...' : '保存' }}</AcButton>
        </div>
      </template>
    </AcModal>

    <!-- 导入弹窗 -->
    <AcModal v-model:show="showImportModal" title="📥 导入规则" max-width="480px">
      <div class="space-y-4">
        <p class="text-sm text-muted-foreground">上传 JSON 文件导入规则，兼容 Kazumi 规则格式</p>
        <input type="file" accept=".json" ref="fileInput" @change="handleImport"
          class="block w-full text-sm font-num text-foreground file:mr-4 file:py-2 file:px-4 file:rounded-2xl file:border-0 file:bg-ac-grass file:text-white file:font-bold file:cursor-pointer hover:file:bg-ac-grass-dark" />
      </div>
    </AcModal>

    <!-- 测试弹窗 -->
    <AcModal v-model:show="showTestModal" :title="`🧪 测试规则: ${testingRule?.name || ''}`" max-width="640px">
      <div class="space-y-4">
        <div class="flex gap-2">
          <div class="flex-1">
            <AcInput v-model="testKeyword" placeholder="输入搜索关键词..." @keyup-enter="executeTest" />
          </div>
          <AcButton variant="primary" :loading="testLoading" @click="executeTest">测试</AcButton>
        </div>
        <div v-if="testLoading" class="flex justify-center py-6"><AcSpinner :size="32" /></div>
        <div v-else-if="testResults.length" class="space-y-2 max-h-[320px] overflow-y-auto">
          <div v-for="(r, i) in testResults" :key="i" class="p-3 rounded-2xl border-2 border-ac-sand hover:bg-ac-sand/30 transition-colors">
            <p class="text-sm font-bold">{{ r.name }}</p>
            <p class="text-xs text-muted-foreground truncate font-num">{{ r.url }}</p>
          </div>
        </div>
        <div v-else-if="testExecuted" class="text-center py-6 text-sm text-muted-foreground">无搜索结果</div>
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

const toast = useToast()
const { confirm } = useConfirm()
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
  } catch { toast.error('获取规则失败') }
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
      toast.success('规则已更新')
    } else {
      await post('/stream-rules', ruleForm.value)
      toast.success('规则已创建')
    }
    showEditModal.value = false
    await fetchRules()
  } catch (e) { toast.error(e.message || '保存失败') }
  finally { saving.value = false }
}

async function deleteRule(rule) {
  const ok = await confirm({ title: '删除规则', content: `确定删除规则 "${rule.name}"？`, variant: 'danger' })
  if (!ok) return
  try {
    await del(`/stream-rules/${rule.id}`)
    toast.success('已删除')
    await fetchRules()
  } catch { toast.error('删除失败') }
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
    try { data = JSON.parse(respText) } catch { toast.error('响应解析失败'); return }
    toast.success(`导入 ${data.imported} 条规则`)
    showImportModal.value = false
    await fetchRules()
  } catch { toast.error('导入失败') }
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
  } catch (e) { toast.error('测试失败: ' + (e.message || '')) }
  finally { testLoading.value = false }
}

onMounted(fetchRules)
</script>
