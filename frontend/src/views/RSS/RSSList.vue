<template>
  <div class="p-4">
    <n-card title="RSS 订阅列表">
      <template #header-extra>
        <n-button type="primary" @click="showAddModal = true">
          添加订阅
        </n-button>
      </template>

      <n-space vertical>
        <n-data-table
          :columns="columns"
          :data="rssList"
          :loading="loading"
          :pagination="pagination"
          @update:page="handlePageChange"
        />
      </n-space>
    </n-card>

    <!-- 添加/编辑 RSS 订阅的模态框 -->
    <n-modal v-model:show="showAddModal" preset="dialog" title="RSS 订阅">
      <n-form
        ref="formRef"
        :model="formModel"
        :rules="rules"
        label-placement="left"
        label-width="auto"
        require-mark-placement="right-hanging"
      >
        <n-form-item label="名称" path="name">
          <n-input v-model:value="formModel.name" placeholder="请输入订阅名称" />
        </n-form-item>
        <n-form-item label="URL" path="url">
          <n-input v-model:value="formModel.url" placeholder="请输入RSS URL" />
        </n-form-item>
        <n-form-item label="关键词" path="keywords">
          <n-input v-model:value="formModel.keywords" placeholder="请输入关键词，多个关键词用逗号分隔" />
        </n-form-item>
        <n-form-item label="排除关键词" path="exclude_keywords">
          <n-input v-model:value="formModel.exclude_keywords" placeholder="请输入排除关键词，多个关键词用逗号分隔" />
        </n-form-item>
        <n-form-item label="下载目录" path="download_dir">
          <n-input v-model:value="formModel.download_dir" placeholder="请输入下载目录" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space>
          <n-button @click="showAddModal = false">取消</n-button>
          <n-button type="primary" :loading="submitting" @click="handleSubmit">
            确定
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup>
import { ref, onMounted, h } from 'vue'
import {
  NCard,
  NButton,
  NSpace,
  NDataTable,
  NModal,
  NForm,
  NFormItem,
  NInput,
  useMessage
} from 'naive-ui'
import { get, post, put, del } from '../../utils/api'

const message = useMessage()
const loading = ref(false)
const submitting = ref(false)
const showAddModal = ref(false)
const rssList = ref([])
const formRef = ref(null)

const formModel = ref({
  name: '',
  url: '',
  keywords: '',
  exclude_keywords: '',
  download_dir: ''
})

const rules = {
  name: {
    required: true,
    message: '请输入订阅名称',
    trigger: 'blur'
  },
  url: {
    required: true,
    message: '请输入RSS URL',
    trigger: 'blur'
  }
}

const pagination = ref({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [10, 20, 30, 40],
  onChange: (page) => {
    pagination.value.page = page
  },
  onUpdatePageSize: (pageSize) => {
    pagination.value.pageSize = pageSize
    pagination.value.page = 1
  }
})

const columns = [
  {
    title: '名称',
    key: 'name',
  },
  {
    title: '关键词',
    key: 'keywords',
  },
  {
    title: '排除关键词',
    key: 'exclude_keywords',
  },
  {
    title: '下载目录',
    key: 'download_dir',
  },
  {
    title: '最后更新',
    key: 'updated_at',
  },
  {
    title: '操作',
    key: 'actions',
    render(row) {
      return h(
        NSpace,
        {},
        {
          default: () => [
            h(
              NButton,
              {
                size: 'small',
                onClick: () => handleEdit(row)
              },
              { default: () => '编辑' }
            ),
            h(
              NButton,
              {
                size: 'small',
                type: 'error',
                onClick: () => handleDelete(row)
              },
              { default: () => '删除' }
            )
          ]
        }
      )
    }
  }
]

async function fetchRSSList(page = 1) {
  loading.value = true
  try {
    const params = {
      page: page,
      per_page: pagination.value.pageSize
    }
    
    const response = await get('/api/v1/rss', params)
    const data = await response.json()
    rssList.value = data.items
    pagination.value.itemCount = data.total
  } catch (error) {
    console.error('获取RSS列表失败:', error)
    message.error('获取RSS列表失败')
  } finally {
    loading.value = false
  }
}

async function handleSubmit() {
  await formRef.value?.validate()
  
  submitting.value = true
  try {
    let response
    
    if (formModel.value.id) {
      response = await put(`/api/v1/rss/${formModel.value.id}`, formModel.value)
    } else {
      response = await post('/api/v1/rss', formModel.value)
    }
    
    if (!response.ok) throw new Error('提交失败')
    
    message.success('保存成功')
    showAddModal.value = false
    fetchRSSList(pagination.value.page)
  } catch (error) {
    console.error('保存RSS失败:', error)
    message.error('保存失败')
  } finally {
    submitting.value = false
  }
}

function handleEdit(row) {
  formModel.value = { ...row }
  showAddModal.value = true
}

async function handleDelete(row) {
  if (!confirm('确定要删除这个RSS订阅吗？')) return
  
  try {
    const response = await del(`/api/v1/rss/${row.id}`)
    
    if (!response.ok) throw new Error('删除失败')
    
    message.success('删除成功')
    fetchRSSList(pagination.value.page)
  } catch (error) {
    console.error('删除RSS失败:', error)
    message.error('删除失败')
  }
}

function handlePageChange(page) {
  fetchRSSList(page)
}

onMounted(() => {
  fetchRSSList()
})
</script> 