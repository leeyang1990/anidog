<template>
  <div>
    <AcPageHeader :title="t('pages.rss.title')" :subtitle="t('pages.rss.subtitle')">
      <template #actions>
        <AcButton variant="primary" @click="showAddModal = true">
          <template #icon><AddOutline class="size-4" /></template>
          {{ t('pages.rss.add') }}
        </AcButton>
      </template>
    </AcPageHeader>

    <!-- Stat cards -->
    <div class="grid grid-cols-3 gap-4 mb-6">
      <AcCard hoverable padding="md" rounded="2xl">
        <div class="text-sm text-muted-foreground font-bold">{{ t('pages.rss.total') }}</div>
        <div class="mt-1 text-2xl font-bold font-num text-foreground">{{ rssFeeds.length }}</div>
      </AcCard>
      <AcCard hoverable padding="md" rounded="2xl">
        <div class="text-sm text-muted-foreground font-bold">{{ t('pages.rss.active') }}</div>
        <div class="mt-1 text-2xl font-bold font-num text-ac-leaf-dark">{{ rssFeeds.filter(f => f.enabled).length }}</div>
      </AcCard>
      <AcCard hoverable padding="md" rounded="2xl">
        <div class="text-sm text-muted-foreground font-bold">{{ t('pages.rss.inactive') }}</div>
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
              <th class="px-4 py-3 font-bold text-muted-foreground">{{ t('pages.rss.name') }}</th>
              <th class="px-4 py-3 font-bold text-muted-foreground">{{ t('pages.rss.address') }}</th>
              <th class="px-4 py-3 font-bold text-muted-foreground w-24">{{ t('pages.rss.status') }}</th>
              <th class="px-4 py-3 font-bold text-muted-foreground w-24">{{ t('pages.rss.parser') }}</th>
              <th class="px-4 py-3 font-bold text-muted-foreground w-32">{{ t('pages.rss.filters') }}</th>
              <th class="px-4 py-3 font-bold text-muted-foreground w-40">{{ t('pages.rss.lastUpdate') }}</th>
              <th class="px-4 py-3 font-bold text-muted-foreground w-44">{{ t('pages.rss.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="rssFeeds.length === 0">
              <td colspan="7" class="px-4 py-8 text-center text-muted-foreground">{{ t('pages.rss.empty') }}</td>
            </tr>
            <tr
              v-for="feed in rssFeeds" :key="feed.id"
              class="border-b-2 border-dashed border-ac-sand last:border-b-0 hover:bg-ac-cream/50 transition-colors"
            >
              <td class="px-4 py-3 font-bold">{{ feed.name }}</td>
              <td class="px-4 py-3 text-muted-foreground max-w-[220px] truncate font-num text-xs" :title="feed.url">{{ feed.url }}</td>
              <td class="px-4 py-3">
                <AcTag :variant="feed.enabled ? 'leaf' : 'wood'">{{ feed.enabled ? t('pages.rss.enabled') : t('pages.rss.disabled') }}</AcTag>
              </td>
              <td class="px-4 py-3">
                <AcTag variant="sky">{{ parserLabels[feed.parser] || feed.parser || 'Mikan' }}</AcTag>
              </td>
              <td class="px-4 py-3 text-muted-foreground text-xs">
                <span v-if="feed.filter_rules && feed.filter_rules.length">{{ t('pages.rss.ruleCount', { count: feed.filter_rules.length }) }}</span>
                <span v-else>{{ t('pages.rss.noRules') }}</span>
              </td>
              <td class="px-4 py-3 text-muted-foreground text-xs font-num">{{ feed.last_check ? formatDate(feed.last_check) : t('pages.rss.never') }}</td>
              <td class="px-4 py-3">
                <div class="flex gap-2">
                  <button class="text-xs text-ac-grass-dark hover:underline font-bold" @click="viewItems(feed)">{{ t('pages.rss.view') }}</button>
                  <button class="text-xs text-ac-sky-dark hover:underline font-bold" @click="refreshFeed(feed)">{{ t('pages.rss.refresh') }}</button>
                  <button class="text-xs text-ac-sun-dark hover:underline font-bold" @click="editFeed(feed)">{{ t('pages.rss.edit') }}</button>
                  <button class="text-xs text-ac-heart-dark hover:underline font-bold" @click="deleteFeed(feed)">{{ t('pages.rss.delete') }}</button>
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
          <span class="text-lg font-bold">{{ t('pages.rss.addTitle') }}</span>
        </div>
      </template>
      <div class="space-y-4">
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">{{ t('pages.rss.feedName') }}</label>
          <AcInput v-model="formValue.name" :placeholder="t('pages.rss.feedNamePlaceholder')" />
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">{{ t('pages.rss.address') }}</label>
          <AcInput v-model="formValue.url" placeholder="https://example.com/rss.xml" />
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">{{ t('pages.rss.enableStatus') }}</label>
          <div class="flex items-center gap-3">
            <AcSwitch v-model="formValue.enabled" />
            <span class="text-sm text-muted-foreground">{{ formValue.enabled ? t('pages.rss.enabled') : t('pages.rss.disabled') }}</span>
          </div>
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">{{ t('pages.rss.parserType') }}</label>
          <AcSelect v-model="formValue.parser" :options="parserOptions" />
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">{{ t('pages.rss.filterRules') }}</label>
          <div class="flex flex-wrap gap-1.5 p-2 rounded-2xl border-2 border-ac-sand bg-card min-h-11">
            <span v-for="(rule, index) in formValue.filter_rules" :key="index"
              class="inline-flex items-center gap-1 rounded-full bg-ac-sand px-2.5 py-1 text-xs font-bold text-ac-wood-dark">
              {{ rule }}
              <button class="hover:text-ac-heart-dark" @click="formValue.filter_rules.splice(index, 1)">×</button>
            </span>
            <input class="flex-1 min-w-[120px] outline-none bg-transparent text-xs px-2 py-1"
              :placeholder="t('pages.rss.filterPlaceholder')" @keydown.enter.prevent="addFilterRule($event)" />
          </div>
        </div>
        <div class="space-y-2">
          <label class="text-sm font-bold text-foreground">{{ t('pages.rss.testConnection') }}</label>
          <div class="flex items-center gap-3">
            <AcButton size="sm" variant="outline" :loading="testing" @click="testRSSFeed">
              {{ testing ? t('pages.rss.testing') : t('pages.rss.testFeed') }}
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
          <AcButton variant="ghost" @click="showAddModal = false">{{ t('common.cancel') }}</AcButton>
          <AcButton variant="primary" @click="handleSubmit">{{ t('pages.rss.confirmAdd') }}</AcButton>
        </div>
      </template>
    </AcModal>

    <!-- View RSS items modal -->
    <AcModal v-model:show="showItemsModal" max-width="900px">
      <template #header>
        <div class="flex items-center gap-2">
          <ListOutline class="size-5 text-ac-grass-dark" />
          <span class="text-lg font-bold">{{ t('pages.rss.itemsTitle', { name: currentFeed?.name || '' }) }}</span>
        </div>
      </template>
      <div v-if="loading" class="flex justify-center py-12"><AcSpinner :size="48" /></div>
      <div v-else class="max-h-[500px] overflow-y-auto space-y-1">
        <div v-for="item in rssItems" :key="item.guid"
          class="flex items-center gap-3 p-3 rounded-2xl hover:bg-ac-sand/30 transition-colors">
          <AcTag :variant="item.downloaded ? 'leaf' : 'wood'">
            {{ item.downloaded ? t('pages.rss.downloaded') : t('pages.rss.notDownloaded') }}
          </AcTag>
          <div class="flex-1 min-w-0">
            <div class="text-sm font-bold truncate">{{ item.title }}</div>
            <div class="text-xs text-muted-foreground mt-0.5 font-num">{{ t('pages.rss.publishedAt', { time: formatDate(item.publish_date) }) }}</div>
          </div>
          <AcButton v-if="!item.downloaded" size="sm" variant="primary" @click="downloadItem(item)">{{ t('pages.rss.download') }}</AcButton>
        </div>
        <div v-if="rssItems.length === 0" class="py-8 text-center text-sm text-muted-foreground">{{ t('pages.rss.noItems') }}</div>
      </div>
    </AcModal>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useToast } from '@/composables/useToast'
import { AddOutline, LogoRss, ListOutline } from '@vicons/ionicons5'
import { get, post, del } from '@/utils/api'
import { AcPageHeader, AcButton, AcCard, AcTag, AcSpinner, AcModal, AcInput, AcSelect, AcSwitch } from '@/components/ac'
import { useI18n } from 'vue-i18n'

const toast = useToast()
const { t, locale } = useI18n({ useScope: 'global' })

const loading = ref(false)
const testing = ref(false)
const testResult = ref(null)
const rssFeeds = ref([])
const rssItems = ref([])
const showAddModal = ref(false)
const showItemsModal = ref(false)
const currentFeed = ref(null)

const formValue = reactive({ name: '', url: '', enabled: true, parser: 'mikan', filter_rules: [] })

const parserOptions = computed(() => [
  { label: 'Mikan', value: 'mikan' },
  { label: 'TMDB', value: 'tmdb' },
  { label: t('pages.rss.raw'), value: 'raw' }
])

const parserLabels = computed(() => ({ mikan: 'Mikan', tmdb: 'TMDB', raw: t('pages.rss.raw') }))

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
  } catch (error) { toast.error(error.message || t('pages.rss.loadFailed')) }
  finally { loading.value = false }
}

async function handleSubmit() {
  if (!formValue.name || formValue.name.length < 2) { toast.warning(t('pages.rss.invalidName')); return }
  if (!formValue.url || !/^https?:\/\//.test(formValue.url)) { toast.warning(t('pages.rss.invalidUrl')); return }
  try {
    await post('/rss', formValue)
    toast.success(t('pages.rss.added'))
    showAddModal.value = false
    resetForm()
    await fetchRSSFeeds()
  } catch (error) { toast.error(error.message || t('pages.rss.addFailed')) }
}

async function testRSSFeed() {
  if (!formValue.url) { toast.warning(t('pages.rss.enterUrl')); return }
  testing.value = true
  testResult.value = null
  try {
    const data = await post('/rss/test', { url: formValue.url })
    testResult.value = { success: true, message: t('pages.rss.testSuccess', { count: data.count }) }
  } catch (error) {
    testResult.value = { success: false, message: t('pages.rss.testFailed', { message: error.message || t('pages.rss.unknownError') }) }
  } finally { testing.value = false }
}

async function viewItems(feed) {
  currentFeed.value = feed
  loading.value = true
  try {
    const data = await get(`/rss/${feed.id}/items`)
    rssItems.value = data
    showItemsModal.value = true
  } catch (error) { toast.error(error.message || t('pages.rss.itemsFailed')) }
  finally { loading.value = false }
}

async function refreshFeed(feed) {
  try {
    await post(`/rss/${feed.id}/refresh`)
    toast.success(t('pages.rss.refreshTriggered'))
    setTimeout(fetchRSSFeeds, 30000)
  } catch (error) { toast.error(error.message || t('pages.rss.refreshFailed')) }
}

function editFeed() { toast.info(t('pages.rss.editPending')) }

async function deleteFeed(feed) {
  try {
    await del(`/rss/${feed.id}`)
    toast.success(t('pages.rss.deleted'))
    await fetchRSSFeeds()
  } catch (error) { toast.error(error.message || t('pages.rss.deleteFailed')) }
}

async function downloadItem(item) {
  try {
    await post('/downloads', { magnet_link: item.link, title: item.title })
    toast.success(t('pages.rss.queued'))
    item.downloaded = true
  } catch (error) { toast.error(error.message || t('pages.rss.queueFailed')) }
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
  return new Date(dateString).toLocaleString(locale.value)
}

onMounted(fetchRSSFeeds)
</script>
