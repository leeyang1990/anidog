<template>
  <div>
    <PageHeader title="RSS订阅管理" subtitle="管理您的番剧RSS订阅源，自动追踪更新">
      <template #actions>
        <button
          class="inline-flex items-center gap-1.5 h-9 px-4 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
          @click="showAddModal = true"
        >
          <n-icon size="16"><AddOutline /></n-icon>
          添加订阅源
        </button>
      </template>
    </PageHeader>

    <!-- Stat cards -->
    <div class="grid grid-cols-3 gap-4 mb-6">
      <div class="bg-card rounded-lg border p-4">
        <div class="text-sm text-muted-foreground">订阅源总数</div>
        <div class="mt-1 text-2xl font-semibold">{{ rssFeeds.length }}</div>
      </div>
      <div class="bg-card rounded-lg border p-4">
        <div class="text-sm text-muted-foreground">启用中</div>
        <div class="mt-1 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ rssFeeds.filter(f => f.enabled).length }}</div>
      </div>
      <div class="bg-card rounded-lg border p-4">
        <div class="text-sm text-muted-foreground">已禁用</div>
        <div class="mt-1 text-2xl font-semibold text-muted-foreground">{{ rssFeeds.filter(f => !f.enabled).length }}</div>
      </div>
    </div>

    <!-- RSS feeds table -->
    <div class="bg-card rounded-lg border">
      <div v-if="loading" class="flex justify-center py-12">
        <n-spin size="large" />
      </div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b bg-muted/50">
            <th class="px-4 py-3 text-left font-medium text-muted-foreground">名称</th>
            <th class="px-4 py-3 text-left font-medium text-muted-foreground">RSS地址</th>
            <th class="px-4 py-3 text-left font-medium text-muted-foreground w-24">状态</th>
            <th class="px-4 py-3 text-left font-medium text-muted-foreground w-24">解析器</th>
            <th class="px-4 py-3 text-left font-medium text-muted-foreground w-32">过滤规则</th>
            <th class="px-4 py-3 text-left font-medium text-muted-foreground w-40">最后更新</th>
            <th class="px-4 py-3 text-left font-medium text-muted-foreground w-44">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="rssFeeds.length === 0">
            <td colspan="7" class="px-4 py-8 text-center text-muted-foreground">暂无RSS订阅源</td>
          </tr>
          <tr
            v-for="feed in rssFeeds"
            :key="feed.id"
            class="border-b last:border-b-0 hover:bg-muted/50 transition-colors"
          >
            <td class="px-4 py-3 font-medium">{{ feed.name }}</td>
            <td class="px-4 py-3 text-muted-foreground max-w-[200px] truncate" :title="feed.url">{{ feed.url }}</td>
            <td class="px-4 py-3">
              <span
                class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
                :class="feed.enabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400' : 'bg-muted text-muted-foreground'"
              >
                {{ feed.enabled ? '启用' : '禁用' }}
              </span>
            </td>
            <td class="px-4 py-3">
              <span class="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-xs font-medium">{{ parserLabels[feed.parser] || feed.parser || 'Mikan' }}</span>
            </td>
            <td class="px-4 py-3 text-muted-foreground">
              <span v-if="feed.filter_rules && feed.filter_rules.length">{{ feed.filter_rules.length }} 条规则</span>
              <span v-else>无规则</span>
            </td>
            <td class="px-4 py-3 text-muted-foreground">{{ feed.last_check ? formatDate(feed.last_check) : '从未更新' }}</td>
            <td class="px-4 py-3">
              <div class="flex gap-2">
                <button class="text-sm text-primary hover:underline" @click="viewItems(feed)">查看</button>
                <button class="text-sm text-primary hover:underline" @click="refreshFeed(feed)">刷新</button>
                <button class="text-sm text-primary hover:underline" @click="editFeed(feed)">编辑</button>
                <button class="text-sm text-destructive hover:underline" @click="deleteFeed(feed)">删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Add RSS modal -->
    <n-modal
      v-model:show="showAddModal"
      preset="card"
      style="width: 600px; max-width: 90vw"
      :bordered="false"
    >
      <template #header>
        <div class="flex items-center gap-2">
          <n-icon size="20"><LogoRss /></n-icon>
          <span class="text-lg font-semibold">添加RSS订阅源</span>
        </div>
      </template>

      <div class="space-y-4">
        <div class="space-y-2">
          <label class="text-sm font-medium">订阅源名称</label>
          <input
            v-model="formValue.name"
            class="flex h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            placeholder="例如：Mikanani"
          />
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">RSS地址</label>
          <input
            v-model="formValue.url"
            class="flex h-9 w-full rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            placeholder="https://example.com/rss.xml"
          />
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">启用状态</label>
          <div class="flex items-center gap-2">
            <button
              class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors"
              :class="formValue.enabled ? 'bg-primary' : 'bg-muted'"
              @click="formValue.enabled = !formValue.enabled"
            >
              <span
                class="pointer-events-none block h-4 w-4 rounded-full bg-white transition-transform"
                :class="formValue.enabled ? 'translate-x-4' : 'translate-x-0'"
              />
            </button>
            <span class="text-sm text-muted-foreground">{{ formValue.enabled ? '启用' : '禁用' }}</span>
          </div>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">解析器类型</label>
          <select
            v-model="formValue.parser"
            class="flex h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <option v-for="opt in parserOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">过滤规则</label>
          <div class="flex flex-wrap gap-2">
            <span
              v-for="(rule, index) in formValue.filter_rules"
              :key="index"
              class="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-1 text-xs"
            >
              {{ rule }}
              <button class="text-muted-foreground hover:text-foreground" @click="formValue.filter_rules.splice(index, 1)">&times;</button>
            </span>
            <input
              class="flex h-7 w-32 rounded-md border border-input bg-background px-2 text-xs placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
              placeholder="添加规则"
              @keydown.enter.prevent="addFilterRule($event)"
            />
          </div>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">测试连接</label>
          <div class="flex items-center gap-3">
            <button
              class="h-9 px-4 rounded-md border border-input bg-background text-sm font-medium hover:bg-accent transition-colors"
              :disabled="testing"
              @click="testRSSFeed"
            >
              {{ testing ? '测试中...' : '测试RSS源' }}
            </button>
            <span
              v-if="testResult"
              class="text-sm"
              :class="testResult.success ? 'text-emerald-600 dark:text-emerald-400' : 'text-destructive'"
            >
              {{ testResult.message }}
            </span>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button
            class="h-9 px-4 rounded-md border border-input bg-background text-sm font-medium hover:bg-accent transition-colors"
            @click="showAddModal = false"
          >
            取消
          </button>
          <button
            class="h-9 px-4 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
            @click="handleSubmit"
          >
            确认添加
          </button>
        </div>
      </template>
    </n-modal>

    <!-- View RSS items modal -->
    <n-modal
      v-model:show="showItemsModal"
      preset="card"
      style="width: 900px; max-width: 95vw"
      :bordered="false"
    >
      <template #header>
        <div class="flex items-center gap-2">
          <n-icon size="20"><ListOutline /></n-icon>
          <span class="text-lg font-semibold">RSS项目 - {{ currentFeed?.name }}</span>
        </div>
      </template>

      <div v-if="loading" class="flex justify-center py-12">
        <n-spin size="large" />
      </div>
      <div v-else class="max-h-[500px] overflow-y-auto space-y-1">
        <div
          v-for="item in rssItems"
          :key="item.guid"
          class="flex items-center gap-3 p-3 rounded-md hover:bg-muted/50 transition-colors"
        >
          <span
            class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
            :class="item.downloaded ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400' : 'bg-muted text-muted-foreground'"
          >
            {{ item.downloaded ? '已下载' : '未下载' }}
          </span>
          <div class="flex-1 min-w-0">
            <div class="text-sm font-medium truncate">{{ item.title }}</div>
            <div class="text-xs text-muted-foreground mt-0.5">发布时间：{{ formatDate(item.publish_date) }}</div>
          </div>
          <button
            v-if="!item.downloaded"
            class="h-7 px-3 rounded-md bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 transition-colors shrink-0"
            @click="downloadItem(item)"
          >
            下载
          </button>
        </div>
        <div v-if="rssItems.length === 0" class="py-8 text-center text-sm text-muted-foreground">暂无RSS项目</div>
      </div>
    </n-modal>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { NIcon, NSpin, NModal, useMessage } from 'naive-ui'
import {
  AddOutline,
  RefreshOutline,
  TrashOutline,
  CreateOutline,
  EyeOutline,
  LogoRss,
  CheckmarkCircleOutline,
  CheckmarkOutline,
  FlaskOutline,
  ListOutline,
  TimeOutline,
  DownloadOutline
} from '@vicons/ionicons5'
import { get, post, del } from '@/utils/api'
import PageHeader from '@/components/Common/PageHeader.vue'

const message = useMessage()

const loading = ref(false)
const testing = ref(false)
const testResult = ref(null)
const rssFeeds = ref([])
const rssItems = ref([])
const showAddModal = ref(false)
const showItemsModal = ref(false)
const currentFeed = ref(null)

const formValue = reactive({
  name: '',
  url: '',
  enabled: true,
  parser: 'mikan',
  filter_rules: []
})

const parserOptions = [
  { label: 'Mikan', value: 'mikan' },
  { label: 'TMDB', value: 'tmdb' },
  { label: '原始', value: 'raw' }
]

const parserLabels = { mikan: 'Mikan', tmdb: 'TMDB', raw: '原始' }

const addFilterRule = (event) => {
  const value = event.target.value.trim()
  if (value) {
    formValue.filter_rules.push(value)
    event.target.value = ''
  }
}

const fetchRSSFeeds = async () => {
  loading.value = true
  try {
    const data = await get('/rss')
    rssFeeds.value = data
  } catch (error) {
    message.error(error.message || '获取RSS源失败')
  } finally {
    loading.value = false
  }
}

const handleSubmit = async () => {
  if (!formValue.name || formValue.name.length < 2) {
    message.warning('请输入订阅源名称（至少2个字符）')
    return
  }
  if (!formValue.url || !/^https?:\/\//.test(formValue.url)) {
    message.warning('请输入有效的RSS地址')
    return
  }
  try {
    await post('/rss', formValue)
    message.success('添加RSS源成功')
    showAddModal.value = false
    resetForm()
    await fetchRSSFeeds()
  } catch (error) {
    message.error(error.message || '添加RSS源失败')
  }
}

const testRSSFeed = async () => {
  if (!formValue.url) {
    message.warning('请先输入RSS地址')
    return
  }

  testing.value = true
  testResult.value = null

  try {
    const data = await post('/rss/test', { url: formValue.url })
    testResult.value = {
      success: true,
      message: `测试成功，获取到 ${data.count} 个项目`
    }
  } catch (error) {
    testResult.value = {
      success: false,
      message: '测试失败：' + (error.message || '未知错误')
    }
  } finally {
    testing.value = false
  }
}

const viewItems = async (feed) => {
  currentFeed.value = feed
  loading.value = true

  try {
    const data = await get(`/rss/${feed.id}/items`)
    rssItems.value = data
    showItemsModal.value = true
  } catch (error) {
    message.error(error.message || '获取RSS项目失败')
  } finally {
    loading.value = false
  }
}

const refreshFeed = async (feed) => {
  try {
    await post(`/rss/${feed.id}/refresh`)
    message.success(`已触发刷新，稍后将出现在条目列表`)
    // 30 秒后自动刷新列表以便用户能看到新条目
    setTimeout(fetchRSSFeeds, 30000)
  } catch (error) {
    message.error(error.message || '刷新RSS源失败')
  }
}

const editFeed = (feed) => {
  message.info('编辑功能待实现')
}

const deleteFeed = async (feed) => {
  try {
    await del(`/rss/${feed.id}`)
    message.success('删除RSS源成功')
    await fetchRSSFeeds()
  } catch (error) {
    message.error(error.message || '删除RSS源失败')
  }
}

const downloadItem = async (item) => {
  try {
    await post('/downloads', {
      magnet_link: item.link,
      title: item.title
    })
    message.success('已添加到下载队列')
    item.downloaded = true
  } catch (error) {
    message.error(error.message || '添加下载任务失败')
  }
}

const showRules = (feed) => {
  message.info(`过滤规则：${feed.filter_rules.join(', ')}`)
}

const resetForm = () => {
  formValue.name = ''
  formValue.url = ''
  formValue.enabled = true
  formValue.parser = 'mikan'
  formValue.filter_rules = []
  testResult.value = null
}

const formatDate = (dateString) => {
  if (!dateString) return ''
  const date = new Date(dateString)
  return date.toLocaleString('zh-CN')
}

onMounted(() => {
  fetchRSSFeeds()
})
</script>
