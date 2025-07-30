<template>
  <div class="min-h-screen bg-gray-100 flex flex-col items-center justify-center">
    <div class="text-center">
      <h1 class="text-9xl font-bold text-gray-900">404</h1>
      <p class="mt-4 text-xl text-gray-600">页面不存在</p>
      <div class="mt-8">
        <n-button type="primary" @click="$router.push('/')">
          返回首页
        </n-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { NButton } from 'naive-ui'
</script> 