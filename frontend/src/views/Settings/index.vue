<template>
  <div class="p-4">
    <n-card title="系统设置">
      <n-tabs type="line" animated>
        <!-- 基本设置 -->
        <n-tab-pane name="basic" tab="基本设置">
          <n-form
            ref="basicFormRef"
            :model="basicForm"
            :rules="basicRules"
            label-placement="left"
            label-width="auto"
            require-mark-placement="right-hanging"
          >
            <n-form-item label="下载目录" path="downloadDir">
              <n-input v-model:value="basicForm.downloadDir" placeholder="请输入默认下载目录" />
            </n-form-item>
            <n-form-item label="并发下载数" path="maxConcurrent">
              <n-input-number
                v-model:value="basicForm.maxConcurrent"
                :min="1"
                :max="10"
              />
            </n-form-item>
            <n-form-item>
              <n-button
                type="primary"
                :loading="saving.basic"
                @click="saveBasicSettings"
              >
                保存设置
              </n-button>
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 用户设置 -->
        <n-tab-pane name="user" tab="用户设置">
          <n-form
            ref="userFormRef"
            :model="userForm"
            :rules="userRules"
            label-placement="left"
            label-width="auto"
            require-mark-placement="right-hanging"
          >
            <n-form-item label="用户名" path="username">
              <n-input v-model:value="userForm.username" disabled />
            </n-form-item>
            <n-form-item label="旧密码" path="oldPassword">
              <n-input
                v-model:value="userForm.oldPassword"
                type="password"
                placeholder="请输入旧密码"
              />
            </n-form-item>
            <n-form-item label="新密码" path="newPassword">
              <n-input
                v-model:value="userForm.newPassword"
                type="password"
                placeholder="请输入新密码"
              />
            </n-form-item>
            <n-form-item label="确认密码" path="confirmPassword">
              <n-input
                v-model:value="userForm.confirmPassword"
                type="password"
                placeholder="请再次输入新密码"
              />
            </n-form-item>
            <n-form-item>
              <n-button
                type="primary"
                :loading="saving.user"
                @click="saveUserSettings"
              >
                修改密码
              </n-button>
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 系统信息 -->
        <n-tab-pane name="system" tab="系统信息">
          <n-descriptions bordered>
            <n-descriptions-item label="系统版本">
              {{ systemInfo.version || '未知' }}
            </n-descriptions-item>
            <n-descriptions-item label="运行时间">
              {{ systemInfo.uptime || '未知' }}
            </n-descriptions-item>
            <n-descriptions-item label="CPU 使用率">
              {{ systemInfo.cpuUsage || '0' }}%
            </n-descriptions-item>
            <n-descriptions-item label="内存使用率">
              {{ systemInfo.memoryUsage || '0' }}%
            </n-descriptions-item>
            <n-descriptions-item label="磁盘使用率">
              {{ systemInfo.diskUsage || '0' }}%
            </n-descriptions-item>
          </n-descriptions>
        </n-tab-pane>
      </n-tabs>
    </n-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '../../stores/auth'
import { get, put } from '../../utils/api'

const message = useMessage()
const authStore = useAuthStore()

const basicFormRef = ref(null)
const userFormRef = ref(null)

const saving = ref({
  basic: false,
  user: false
})

const basicForm = ref({
  downloadDir: '',
  maxConcurrent: 3
})

const userForm = ref({
  username: authStore.user?.username || '',
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const systemInfo = ref({
  version: '',
  uptime: '',
  cpuUsage: 0,
  memoryUsage: 0,
  diskUsage: 0
})

const basicRules = {
  downloadDir: {
    required: true,
    message: '请输入下载目录',
    trigger: 'blur'
  },
  maxConcurrent: {
    required: true,
    type: 'number',
    message: '请输入并发下载数',
    trigger: 'blur'
  }
}

const userRules = {
  oldPassword: {
    required: true,
    message: '请输入旧密码',
    trigger: 'blur'
  },
  newPassword: {
    required: true,
    message: '请输入新密码',
    trigger: 'blur'
  },
  confirmPassword: {
    required: true,
    message: '请确认新密码',
    trigger: 'blur',
    validator: (rule, value) => {
      return value === userForm.value.newPassword || new Error('两次输入的密码不一致')
    }
  }
}

async function fetchSettings() {
  try {
    const response = await get('/api/v1/settings')
    const data = await response.json()
    basicForm.value = {
      downloadDir: data.downloadDir,
      maxConcurrent: data.maxConcurrent
    }
  } catch (error) {
    console.error('获取设置失败:', error)
    message.error('获取设置失败')
  }
}

async function fetchSystemInfo() {
  try {
    const response = await get('/api/v1/system/info')
    const data = await response.json()
    systemInfo.value = data
  } catch (error) {
    console.error('获取系统信息失败:', error)
    message.error('获取系统信息失败')
  }
}

async function saveBasicSettings() {
  try {
    await basicFormRef.value?.validate()
    saving.value.basic = true
    
    const response = await put('/api/v1/settings', basicForm.value)

    if (!response.ok) {
      throw new Error('保存失败')
    }

    message.success('保存成功')
  } catch (error) {
    if (error?.message) {
      message.error(error.message)
    }
  } finally {
    saving.value.basic = false
  }
}

async function saveUserSettings() {
  try {
    await userFormRef.value?.validate()
    saving.value.user = true
    
    const response = await put('/api/v1/users/password', {
      old_password: userForm.value.oldPassword,
      new_password: userForm.value.newPassword
    })

    if (!response.ok) {
      throw new Error('修改密码失败')
    }

    message.success('密码修改成功')
    userForm.value.oldPassword = ''
    userForm.value.newPassword = ''
    userForm.value.confirmPassword = ''
  } catch (error) {
    if (error?.message) {
      message.error(error.message)
    }
  } finally {
    saving.value.user = false
  }
}

onMounted(() => {
  fetchSettings()
  fetchSystemInfo()
})
</script> 