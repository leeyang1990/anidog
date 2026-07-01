<template>
  <div class="min-h-screen flex items-center justify-center bg-background ac-grass-pattern p-4 relative overflow-hidden">
    <!-- 浮动叶子 -->
    <div class="absolute top-10 left-10 size-12 opacity-30 animate-bounce-soft">🍃</div>
    <div class="absolute bottom-20 right-16 size-12 opacity-30 animate-bounce-soft" style="animation-delay: 0.6s">🌿</div>
    <div class="absolute top-1/3 right-10 size-12 opacity-30 animate-bounce-soft" style="animation-delay: 1.2s">🌱</div>

    <AcCard padding="lg" rounded="2xl" class="text-center max-w-md relative z-10">
      <div class="text-7xl mb-4">🐾</div>
      <div class="text-8xl font-bold tracking-tighter text-ac-grass/40 font-num">404</div>
      <h1 class="text-2xl font-bold tracking-tight mt-4 text-foreground">迷路啦~</h1>
      <p class="text-sm text-muted-foreground mt-2">这只小狗找不到这一页啦，可能是去追番了 🌸</p>
      <div class="flex items-center justify-center gap-3 mt-8">
        <AcButton variant="primary" @click="router.push('/')">
          <template #icon><HomeOutline class="size-4" /></template>
          返回首页
        </AcButton>
        <AcButton variant="outline" @click="router.back()">
          <template #icon><ArrowBackOutline class="size-4" /></template>
          返回上页
        </AcButton>
      </div>
    </AcCard>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
import { HomeOutline, ArrowBackOutline } from '@vicons/ionicons5'
import { AcCard, AcButton } from '../components/ac'

const router = useRouter()
</script>
