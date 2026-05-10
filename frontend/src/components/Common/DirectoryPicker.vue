<template>
  <div class="relative" ref="containerRef">
    <!-- 触发器 -->
    <button
      class="w-full flex items-center gap-2 h-9 px-3 rounded-md border border-input bg-background text-sm text-left hover:border-primary/50 focus:outline-none focus:ring-2 focus:ring-ring"
      @click="toggle"
    >
      <n-icon size="14" class="text-muted-foreground shrink-0"><FolderOutline /></n-icon>
      <span class="flex-1 truncate font-mono text-xs">{{ displayPath }}</span>
      <n-icon size="12" class="text-muted-foreground shrink-0 transition-transform" :class="open ? 'rotate-180' : ''"><ChevronDownOutline /></n-icon>
    </button>

    <!-- 下拉目录列表 -->
    <div v-if="open" class="absolute z-50 left-0 right-0 mt-1 rounded-md border bg-background shadow-lg overflow-hidden" style="max-height: 300px">
      <!-- 顶栏：当前浏览路径 + 选中此目录 -->
      <div class="flex items-center gap-2 px-3 py-2 border-b bg-muted/30 text-xs">
        <span class="text-muted-foreground">浏览:</span>
        <span class="font-mono flex-1 truncate">/{{ currentPath || '' }}</span>
        <button class="px-2 py-0.5 rounded bg-primary text-primary-foreground text-xs hover:bg-primary/90"
          @click.stop="confirmSelect">选中</button>
      </div>

      <div class="overflow-y-auto" style="max-height: 240px">
        <div v-if="loading" class="py-4 text-center text-xs text-muted-foreground">加载中...</div>
        <template v-else>
          <button v-if="currentPath"
            class="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-accent transition-colors text-muted-foreground"
            @click.stop="goUp">
            <n-icon size="14"><ArrowUpOutline /></n-icon>
            <span>..</span>
          </button>
          <button v-for="d in directories" :key="d.path"
            class="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-accent transition-colors"
            @click.stop="enter(d)">
            <n-icon size="14" class="text-muted-foreground"><FolderOutline /></n-icon>
            <span class="truncate">{{ d.name }}</span>
          </button>
          <div v-if="!directories.length" class="py-4 text-center text-xs text-muted-foreground">空目录</div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { NIcon, useMessage } from 'naive-ui'
import { FolderOutline, ChevronDownOutline, ArrowUpOutline } from '@vicons/ionicons5'
import { post } from '@/utils/api'

const props = defineProps({
  modelValue: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue'])
const message = useMessage()

const containerRef = ref(null)
const open = ref(false)
const currentPath = ref('')
const parentPath = ref('')
const directories = ref([])
const loading = ref(false)

const displayPath = computed(() => props.modelValue || '/')

function toggle() {
  open.value = !open.value
  if (open.value) {
    // 每次打开从当前选中路径开始浏览（不写入）
    const start = (props.modelValue || '').replace(/^\//, '')
    fetchDir(start)
  }
}

async function fetchDir(path) {
  loading.value = true
  try {
    const data = await post('/filesystem/list', { path: path || '' })
    directories.value = (data.children || []).filter(x => x.is_dir)
    currentPath.value = data.path || ''
    parentPath.value = data.parent_path || ''
    // 不在这里 emit，避免浏览就改父组件值
  } catch (e) {
    // 路径不存在就回到 root
    if (path) {
      fetchDir('')
      return
    }
    message.error(e.message || '读取目录失败')
    directories.value = []
  } finally {
    loading.value = false
  }
}

function enter(d) {
  fetchDir(d.path)
}

function goUp() {
  fetchDir(parentPath.value)
}

function confirmSelect() {
  emit('update:modelValue', '/' + (currentPath.value || ''))
  open.value = false
}

function onClickOutside(e) {
  if (containerRef.value && !containerRef.value.contains(e.target)) {
    open.value = false
  }
}

onMounted(() => document.addEventListener('click', onClickOutside))
onBeforeUnmount(() => document.removeEventListener('click', onClickOutside))
</script>
