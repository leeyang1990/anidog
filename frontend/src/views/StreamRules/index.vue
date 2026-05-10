<template>
  <div>
    <PageHeader title="规则管理" subtitle="管理番剧源解析规则（兼容 Kazumi 格式）">
      <template #actions>
        <div class="flex gap-2">
          <button class="border border-input bg-background hover:bg-accent rounded-md h-9 px-4 text-sm font-medium transition-colors"
            @click="showImportModal = true">
            <n-icon size="14"><CloudUploadOutline /></n-icon> 导入
          </button>
          <button class="bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-9 px-4 text-sm font-medium transition-colors"
            @click="openCreateModal">
            <n-icon size="14"><AddOutline /></n-icon> 添加规则
          </button>
        </div>
      </template>
    </PageHeader>

    <!-- 规则列表 -->
    <n-spin :show="loading">
      <div v-if="rules.length" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div v-for="rule in rules" :key="rule.id" class="bg-card rounded-lg border p-6 hover:shadow-md transition-shadow">
          <div class="flex items-center justify-between mb-3">
            <div class="flex items-center gap-2">
              <div class="h-8 w-8 rounded-md bg-primary/10 flex items-center justify-center">
                <n-icon size="16" class="text-primary"><GlobeOutline /></n-icon>
              </div>
              <div>
                <h3 class="text-sm font-medium">{{ rule.display_name || rule.name }}</h3>
                <p class="text-xs text-muted-foreground">v{{ rule.version }}</p>
              </div>
            </div>
            <span :class="rule.enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-muted-foreground'" class="text-xs font-medium">
              {{ rule.enabled ? '启用' : '禁用' }}
            </span>
          </div>
          <p class="text-xs text-muted-foreground truncate mb-3">{{ rule.base_url }}</p>
          <div class="flex items-center gap-2">
            <button class="text-xs text-primary hover:underline" @click="openEditModal(rule)">编辑</button>
            <button class="text-xs text-muted-foreground hover:text-foreground" @click="testRule(rule)">测试</button>
            <button class="text-xs text-destructive hover:underline" @click="deleteRule(rule)">删除</button>
          </div>
        </div>
      </div>
      <div v-else class="text-center py-12 text-sm text-muted-foreground">暂无规则，请添加或导入</div>
    </n-spin>

    <!-- 添加/编辑规则弹窗 -->
    <n-modal v-model:show="showEditModal" preset="card" :style="{ width: '600px', maxWidth: '90vw' }" :title="editingRule ? '编辑规则' : '添加规则'" :bordered="false">
      <form @submit.prevent="handleSaveRule" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <!-- name -->
          <div class="space-y-2">
            <label class="text-sm font-medium">规则名称 *</label>
            <input v-model="ruleForm.name" required class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" placeholder="如 nyafun" />
          </div>
          <!-- display_name -->
          <div class="space-y-2">
            <label class="text-sm font-medium">显示名称</label>
            <input v-model="ruleForm.display_name" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" placeholder="如 NYA FUN动漫" />
          </div>
        </div>
        <!-- base_url -->
        <div class="space-y-2">
          <label class="text-sm font-medium">站点 URL *</label>
          <input v-model="ruleForm.base_url" required class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" placeholder="https://example.com" />
        </div>
        <!-- search_url -->
        <div class="space-y-2">
          <label class="text-sm font-medium">搜索 URL *</label>
          <input v-model="ruleForm.search_url" required class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" placeholder="https://example.com/search?q=@keyword" />
          <p class="text-xs text-muted-foreground">@keyword 会被替换为搜索词</p>
        </div>
        <!-- XPath 选择器 -->
        <div class="space-y-2">
          <label class="text-sm font-medium">搜索列表 XPath *</label>
          <input v-model="ruleForm.search_list_xpath" required class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm font-mono placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" placeholder="//div[@class='search-list']" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <label class="text-sm font-medium">标题 XPath *</label>
            <input v-model="ruleForm.search_name_xpath" required class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm font-mono placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" placeholder="//a/h3" />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">链接 XPath *</label>
            <input v-model="ruleForm.search_result_xpath" required class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm font-mono placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" placeholder="//a" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <label class="text-sm font-medium">线路 XPath</label>
            <input v-model="ruleForm.chapter_roads_xpath" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm font-mono placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" placeholder="//ul[@class='road']" />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">集数 XPath *</label>
            <input v-model="ruleForm.chapter_result_xpath" required class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm font-mono placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring" placeholder="//li/a" />
          </div>
        </div>
        <!-- 开关 -->
        <div class="flex items-center gap-6">
          <label class="flex items-center gap-2 text-sm">
            <input type="checkbox" v-model="ruleForm.use_post" class="rounded border-input" /> POST 搜索
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input type="checkbox" v-model="ruleForm.use_webview" class="rounded border-input" /> 需要 JS 渲染
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input type="checkbox" v-model="ruleForm.multi_sources" class="rounded border-input" /> 多线路
          </label>
        </div>
        <div class="flex justify-end gap-2 pt-2">
          <button type="button" class="border border-input bg-background hover:bg-accent rounded-md h-9 px-4 text-sm font-medium transition-colors" @click="showEditModal = false">取消</button>
          <button type="submit" class="bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-9 px-4 text-sm font-medium transition-colors" :disabled="saving">{{ saving ? '保存中...' : '保存' }}</button>
        </div>
      </form>
    </n-modal>

    <!-- 导入弹窗 -->
    <n-modal v-model:show="showImportModal" preset="card" style="width: 480px; max-width: 90vw" title="导入规则" :bordered="false">
      <div class="space-y-4">
        <p class="text-sm text-muted-foreground">上传 JSON 文件导入规则，兼容 Kazumi 规则格式</p>
        <input type="file" accept=".json" ref="fileInput" @change="handleImport" class="text-sm" />
      </div>
    </n-modal>

    <!-- 测试弹窗 -->
    <n-modal v-model:show="showTestModal" preset="card" style="width: 600px; max-width: 90vw" :title="`测试规则: ${testingRule?.name || ''}`" :bordered="false">
      <div class="space-y-4">
        <div class="flex gap-2">
          <input v-model="testKeyword" placeholder="输入搜索关键词..." class="h-9 flex-1 rounded-md border border-input bg-background px-3 text-sm" @keydown.enter="executeTest" />
          <button class="bg-primary text-primary-foreground rounded-md h-9 px-4 text-sm font-medium" @click="executeTest" :disabled="testLoading">测试</button>
        </div>
        <n-spin :show="testLoading">
          <div v-if="testResults.length" class="space-y-2 max-h-[300px] overflow-y-auto">
            <div v-for="(r, i) in testResults" :key="i" class="p-3 rounded-md border hover:bg-muted/50">
              <p class="text-sm font-medium">{{ r.name }}</p>
              <p class="text-xs text-muted-foreground truncate">{{ r.url }}</p>
            </div>
          </div>
          <div v-else-if="testExecuted" class="text-center py-6 text-sm text-muted-foreground">无搜索结果</div>
        </n-spin>
      </div>
    </n-modal>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useMessage, NIcon, NSpin, NModal } from 'naive-ui'
import { AddOutline, GlobeOutline, CloudUploadOutline } from '@vicons/ionicons5'
import { get, post, put, del } from '@/utils/api'
import PageHeader from '@/components/Common/PageHeader.vue'

const message = useMessage()
const loading = ref(false)
const saving = ref(false)
const rules = ref([])

// 编辑弹窗
const showEditModal = ref(false)
const editingRule = ref(null)
const ruleForm = ref(getEmptyForm())

// 导入弹窗
const showImportModal = ref(false)
const fileInput = ref(null)

// 测试弹窗
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
  } catch (e) {
    message.error('获取规则失败')
  } finally {
    loading.value = false
  }
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
      message.success('规则已更新')
    } else {
      await post('/stream-rules', ruleForm.value)
      message.success('规则已创建')
    }
    showEditModal.value = false
    await fetchRules()
  } catch (e) {
    message.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function deleteRule(rule) {
  if (!confirm(`确定删除规则 "${rule.name}"？`)) return
  try {
    await del(`/stream-rules/${rule.id}`)
    message.success('已删除')
    await fetchRules()
  } catch (e) {
    message.error('删除失败')
  }
}

async function handleImport(e) {
  const file = e.target.files[0]
  if (!file) return
  const formData = new FormData()
  formData.append('file', file)
  try {
    // 手动发送 multipart 请求
    const token = localStorage.getItem('token')
    const resp = await fetch('/api/v1/stream-rules/import', {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
      body: formData,
    })
    const respText = await resp.text()
    let data
    try {
      data = JSON.parse(respText)
    } catch (e) {
      message.error('响应解析失败')
      return
    }
    message.success(`导入 ${data.imported} 条规则`)
    showImportModal.value = false
    await fetchRules()
  } catch (e) {
    message.error('导入失败')
  }
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
  } catch (e) {
    message.error('测试失败: ' + (e.message || ''))
  } finally {
    testLoading.value = false
  }
}

onMounted(fetchRules)
</script>
