<template>
  <div class="relative" ref="containerRef">
    <!-- 触发器 -->
    <button
      type="button"
      class="w-full flex items-center gap-2 h-10 px-3 rounded-2xl border-2 border-ac-sand bg-card text-sm text-left hover:border-ac-grass focus:outline-none focus:ring-4 focus:ring-ac-grass/20 transition-colors"
      @click="toggle"
    >
      <FolderOutline class="size-4 text-ac-wood-dark shrink-0" />
      <span class="flex-1 truncate font-num text-xs text-foreground">{{ displayPath }}</span>
      <ChevronDownOutline class="size-3 text-muted-foreground shrink-0 transition-transform" :class="open ? 'rotate-180' : ''" />
    </button>

    <!-- 下拉目录列表 -->
    <div v-if="open" class="absolute z-50 left-0 right-0 mt-2 rounded-2xl border-2 border-ac-sand bg-card shadow-lg overflow-hidden" style="max-height: 320px">
      <!-- 顶栏：当前浏览路径 -->
      <div class="flex items-center gap-2 px-3 py-2 border-b-2 border-dashed border-ac-sand bg-ac-sand/30 text-xs">
        <span class="text-muted-foreground font-bold">浏览:</span>
        <span class="font-num flex-1 truncate text-foreground">/{{ currentPath || '' }}</span>
      </div>

      <div class="overflow-y-auto" style="max-height: 260px">
        <div v-if="loading" class="py-6 flex justify-center"><AcSpinner :size="24" /></div>
        <template v-else>
          <button v-if="currentPath"
            type="button"
            class="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-ac-sand/40 transition-colors text-muted-foreground"
            @click.stop="goUp">
            <ArrowUpOutline class="size-4" />
            <span>..</span>
          </button>
          <button v-for="d in directories" :key="d.path"
            type="button"
            class="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-ac-sand/40 transition-colors"
            @click.stop="enter(d)">
            <FolderOutline class="size-4 text-ac-sun-dark" />
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
import { FolderOutline, ChevronDownOutline, ArrowUpOutline } from '@vicons/ionicons5'
import { post } from '@/utils/api'
import { useToast } from '../../composables/useToast'
import { AcSpinner } from '../ac'

const props = defineProps({
  modelValue: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue'])
const toast = useToast()

const containerRef = ref(null)
const open = ref(false)
const currentPath = ref('')
const parentPath = ref('')
const directories = ref([])
const loading = ref(false)
const ROOT_PATH = '/downloads'

const displayPath = computed(() => props.modelValue || '/')

function toRelativePath(path) {
  if (!path || path === '/' || path === ROOT_PATH) return ''
  if (path.startsWith(ROOT_PATH + '/')) return path.slice(ROOT_PATH.length + 1)
  return path.replace(/^\/+/, '')
}

function toggle() {
  open.value = !open.value
  if (open.value) {
    const start = toRelativePath(props.modelValue)
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
  } catch (e) {
    if (path) {
      fetchDir('')
      return
    }
    toast.error(e.message || '读取目录失败')
    directories.value = []
  } finally {
    loading.value = false
  }
}

function enter(d) {
  emit('update:modelValue', `${ROOT_PATH}/${d.path}`)
  fetchDir(d.path)
}
function goUp() { fetchDir(parentPath.value) }

function onClickOutside(e) {
  if (containerRef.value && !containerRef.value.contains(e.target)) {
    open.value = false
  }
}

onMounted(() => document.addEventListener('click', onClickOutside))
onBeforeUnmount(() => document.removeEventListener('click', onClickOutside))
</script>
