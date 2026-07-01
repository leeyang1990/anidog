<template>
  <div>
    <AcPageHeader title="📡 RSS订阅管理" subtitle="管理您的番剧RSS订阅源，自动追踪更新">
      <template #actions>
        <AcButton variant="primary" @click="showAddModal = true">
          <template #icon><AddOutline class="size-4" /></template>
          添加订阅源
        </AcButton>
      </template>
    </AcPageHeader>

    <!-- Stat cards -->
    <div class="grid grid-cols-3 gap-4 mb-6">
      <AcCard hoverable padding="md" rounded="2xl">
        <div class="text-sm text-muted-foreground font-bold">订阅源总数</div>
        <div class="mt-1 text-2xl font-bold font-num text-foreground">{{ rssFeeds.length }}</div>
      </AcCard>
      <AcCard hoverable padding="md" rounded="2xl">
        <div class="text-sm text-muted-foreground font-bold">启用中</div>
        <div class="mt-1 text-2xl font-bold font-num text-ac-leaf-dark">{{ rssFeeds.filter(f => f.enabled).length }}</div>
      </AcCard>
      <AcCard hoverable padding="md" rounded="2xl">
        <div class="text-sm text-muted-foreground font-bold">已禁用</div>
        <div class="mt-1 text-2xl font-bold font-num text-muted-foreground">{{ rssFeeds.filter(f => !f.enabled).length }}</div>
      </AcCard>
    </div>

    <!-- RSS feeds table -->
    <AcCard padding="none" rounded="2xl">
      <div v-if="loading" class="flex justify-center py-12"><AcSpinner :size="48" /></div>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b-2 border-dashed border-ac-sand bg-ac-sand/30 text-left text-xs">
              <th class="px-4 py-3 font-bold text-muted-foreground">名称</th>
              <th class="px-4 py-3 font-bold text-muted-foreground">RSS地址</th>
              <th class="px-4 py-3 font-bold text-muted-foreground w-24">状态</th>
              <th class="px-4 py-3 font-bold text-muted-foreground w-24">解析器</th>
              <th class="px-4 py-3 font-bold text-muted-foreground w-32">过滤规则</th>
              <th class="px-4 py-3 font-bold text-muted-foreground w-40">最后更新</th>
              <th class="px-4 py-3 font-bold text-muted-foreground w-44">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="rssFeeds.length === 0">
              <td colspan="7" class="px-4 py-8 text-center text-muted-foreground">暂无RSS订阅源</td>
            </tr>
            <tr
              v-for="feed in rssFeeds" :key="feed.id"
              class="border-b-2 border-dashed border-ac-sand last:border-b-0 hover:bg-ac-cream/50 transition-colors"
            >
              <td class="px-4 py-3 font-bold">{{ feed.name }}</td>
              <td class="px-4 py-3 text-muted-foreground max-w-[220px] truncate font-num text-xs" :title="feed.url">{{ feed.url }}</td>
              <td class="px-4 py-3">
                <AcTag :variant="feed.enabled ? 'leaf' : 'wood'">{{ feed.enabled ? '启用' : '禁用' }}</AcTag>
              </td>
              <td class="px-4 py-3">
                <AcTag variant="sky">{{ parserLabels[feed.parser] || feed.parser || 'Mikan' }}</AcTag>
              </td>
              <td class="px-4 py-3 text-muted-foreground text-xs">
                <span v-if="feed.filter_rules && feed.filter_rules.length">{{ feed.filter_rules.length }} 条规则</span>
                <span v-else>无规则</span>
              </td>
              <td class="px-4 py-3 text-muted-foreground text-xs font-num">{{ feed.last_check ? formatDate(feed.last_check) : '从未更新' }}</td>
              <td class="px-4 py-3">
                <div class="flex gap-2">
                  <button class="text-xs text-ac-grass-dark hover:underline font-bold" @click="viewItems(feed)">查看</button>
                  <button class="text-xs text-ac-sky-dark hover:underline font-bold" @click="refreshFeed(feed)">刷新</button>
                  <button class="text-xs text-ac-sun-dark hover:underline font-bold" @click="editFeed(feed)">编辑</button>
                  <button class="text-xs text-ac-heart-dark hover:underline font-bold" @click="deleteFeed(feed)">删除</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </AcCard>

    <!-- Add RSS modal -->
    <AcModal v-model:show="showAddModal" max-width="600px">
      <template #header>
        <div class="flex items-center gap-2">
          <LogoRss class="size-5 text-ac-sun-dark" />
          <span class="text-lg font-bold">添加RSS订阅源</span>
        </div>
      </template>
      <div class="space-y-4">
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">订阅源名称</label>
          <AcInput v-model="formValue.name" placeholder="例如：Mikanani" />
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">RSS地址</label>
          <AcInput v-model="formValue.url" placeholder="https://example.com/rss.xml" />
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">启用状态</label>
          <div class="flex items-center gap-3">
            <AcSwitch v-model="formValue.enabled" />
            <span class="text-sm text-muted-foreground">{{ formValue.enabled ? '启用' : '禁用' }}</span>
          </div>
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">解析器类型</label>
          <AcSelect v-model="formValue.parser" :options="parserOptions" />
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">过滤规则</label>
          <div class="flex flex-wrap gap-1.5 p-2 rounded-2xl border-2 border-ac-sand bg-card min-h-11">
            <span v-for="(rule, index) in formValue.filter_rules" :key="index"
              class="inline-flex items-center gap-1 rounded-full bg-ac-sand px-2.5 py-1 text-xs font-bold text-ac-wood-dark">
              {{ rule }}
              <button class="hover:text-ac-heart-dark" @click="formValue.filter_rules.splice(index, 1)">×</button>
            </span>
            <input class="flex-1 min-w-[120px] outline-none bg-transparent text-xs px-2 py-1"
              placeholder="输入后回车添加" @keydown.enter.prevent="addFilterRule($event)" />
          </div>
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">测试连接</label>
          <div class="flex items-center gap-3">
            <AcButton size="sm" variant="outline" :loading="testing" @click="testRSSFeed">
              {{ testing ? '测试中...' : '测试RSS源' }}
            </AcButton>
            <span v-if="testResult" class="text-sm font-bold"
              :class="testResult.success ? 'text-ac-leaf-dark' : 'text-ac-heart-dark'">
              {{ testResult.message }}
            </span>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <AcButton variant="ghost" @click="showAddModal = false">取消</AcButton>
          <AcButton variant="primary" @click="handleSubmit">确认添加</AcButton>
        </div>
      </template>
    </AcModal>

    <!-- View RSS items modal -->
    <AcModal v-model:show="showItemsModal" max-width="900px">
      <template #header>
        <div class="flex items-center gap-2">
          <ListOutline class="size-5 text-ac-grass-dark" />
          <span class="text-lg font-bold">RSS项目 - {{ currentFeed?.name }}</span>
        </div>
      </template>
      <div v-if="loading" class="flex justify-center py-12"><AcSpinner :size="48" /></div>
      <div v-else class="max-h-[500px] overflow-y-auto space-y-1">
        <div v-for="item in rssItems" :key="item.guid"
          class="flex items-center gap-3 p-3 rounded-2xl hover:bg-ac-sand/30 transition-colors">
          <AcTag :variant="item.downloaded ? 'leaf' : 'wood'">
            {{ item.downloaded ? '已下载' : '未下载' }}
          </AcTag>
          <div class="flex-1 min-w-0">
            <div class="text-sm font-bold truncate">{{ item.title }}</div>
            <div class="text-xs text-muted-foreground mt-0.5 font-num">发布时间：{{ formatDate(item.publish_date) }}</div>
          </div>
          <AcButton v-if="!item.downloaded" size="sm" variant="primary" @click="downloadItem(item)">下载</AcButton>
        </div>
        <div v-if="rssItems.length === 0" class="py-8 text-center text-sm text-muted-foreground">暂无RSS项目</div>
      </div>
    </AcModal>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useToast } from '@/composables/useToast'
import { AddOutline, LogoRss, ListOutline } from '@vicons/ionicons5'
import { get, post, del } from '@/utils/api'
import { AcPageHeader, AcButton, AcCard, AcTag, AcSpinner, AcModal, AcInput, AcSelect, AcSwitch } from '@/components/ac'

const toast = useToast()

const loading = ref(false)
const testing = ref(false)
const testResult = ref(null)
const rssFeeds = ref([])
const rssItems = ref([])
const showAddModal = ref(false)
const showItemsModal = ref(false)
const currentFeed = ref(null)

const formValue = reactive({ name: '', url: '', enabled: true, parser: 'mikan', filter_rules: [] })

const parserOptions = [
  { label: 'Mikan', value: 'mikan' },
  { label: 'TMDB', value: 'tmdb' },
  { label: '原始', value: 'raw' }
]

const parserLabels = { mikan: 'Mikan', tmdb: 'TMDB', raw: '原始' }

function addFilterRule(event) {
  const value = event.target.value.trim()
  if (value) {
    formValue.filter_rules.push(value)
    event.target.value = ''
  }
}

async function fetchRSSFeeds() {
  loading.value = true
  try {
    const data = await get('/rss')
    rssFeeds.value = data
  } catch (error) { toast.error(error.message || '获取RSS源失败') }
  finally { loading.value = false }
}

async function handleSubmit() {
  if (!formValue.name || formValue.name.length < 2) { toast.warning('请输入订阅源名称（至少2个字符）'); return }
  if (!formValue.url || !/^https?:\/\//.test(formValue.url)) { toast.warning('请输入有效的RSS地址'); return }
  try {
    await post('/rss', formValue)
    toast.success('添加RSS源成功')
    showAddModal.value = false
    resetForm()
    await fetchRSSFeeds()
  } catch (error) { toast.error(error.message || '添加RSS源失败') }
}

async function testRSSFeed() {
  if (!formValue.url) { toast.warning('请先输入RSS地址'); return }
  testing.value = true
  testResult.value = null
  try {
    const data = await post('/rss/test', { url: formValue.url })
    testResult.value = { success: true, message: `测试成功，获取到 ${data.count} 个项目` }
  } catch (error) {
    testResult.value = { success: false, message: '测试失败：' + (error.message || '未知错误') }
  } finally { testing.value = false }
}

async function viewItems(feed) {
  currentFeed.value = feed
  loading.value = true
  try {
    const data = await get(`/rss/${feed.id}/items`)
    rssItems.value = data
    showItemsModal.value = true
  } catch (error) { toast.error(error.message || '获取RSS项目失败') }
  finally { loading.value = false }
}

async function refreshFeed(feed) {
  try {
    await post(`/rss/${feed.id}/refresh`)
    toast.success(`已触发刷新，稍后将出现在条目列表`)
    setTimeout(fetchRSSFeeds, 30000)
  } catch (error) { toast.error(error.message || '刷新RSS源失败') }
}

function editFeed() { toast.info('编辑功能待实现') }

async function deleteFeed(feed) {
  try {
    await del(`/rss/${feed.id}`)
    toast.success('删除RSS源成功')
    await fetchRSSFeeds()
  } catch (error) { toast.error(error.message || '删除RSS源失败') }
}

async function downloadItem(item) {
  try {
    await post('/downloads', { magnet_link: item.link, title: item.title })
    toast.success('已添加到下载队列')
    item.downloaded = true
  } catch (error) { toast.error(error.message || '添加下载任务失败') }
}

function resetForm() {
  formValue.name = ''
  formValue.url = ''
  formValue.enabled = true
  formValue.parser = 'mikan'
  formValue.filter_rules = []
  testResult.value = null
}

function formatDate(dateString) {
  if (!dateString) return ''
  return new Date(dateString).toLocaleString('zh-CN')
}

onMounted(fetchRSSFeeds)
</script>
