<template>
  <div class="min-h-screen flex items-center justify-center bg-background p-4">
    <div class="text-center max-w-md">
      <div class="text-8xl font-bold tracking-tighter text-muted-foreground/30">404</div>
      <h1 class="text-2xl font-semibold tracking-tight mt-4">页面不存在</h1>
      <p class="text-sm text-muted-foreground mt-2">抱歉，您访问的页面走丢了... 可能是去追番了</p>
      <div class="flex items-center justify-center gap-3 mt-8">
        <button
          class="inline-flex items-center gap-1.5 bg-primary text-primary-foreground hover:bg-primary/90 rounded-md h-10 px-6 text-sm font-medium transition-colors"
          @click="router.push('/')"
        >
          <n-icon size="16"><HomeOutline /></n-icon> 返回首页
        </button>
        <button
          class="inline-flex items-center gap-1.5 border border-input bg-background hover:bg-accent rounded-md h-10 px-6 text-sm font-medium transition-colors"
          @click="router.back()"
        >
          <n-icon size="16"><ArrowBackOutline /></n-icon> 返回上页
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
import { NIcon } from 'naive-ui'
import { HomeOutline, ArrowBackOutline } from '@vicons/ionicons5'

const router = useRouter()
</script>
