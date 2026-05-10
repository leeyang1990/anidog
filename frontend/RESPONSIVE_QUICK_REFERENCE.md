# 响应式设计快速参考

## 🎯 快速开始

### 1. 导入响应式工具
```javascript
import { useResponsive } from '@/composables/useResponsive'

const { isMobile, isTablet, isDesktop } = useResponsive()
```

### 2. 条件渲染
```vue
<template>
  <!-- 仅移动端显示 -->
  <div v-if="isMobile">移动端内容</div>

  <!-- 仅桌面端显示 -->
  <div v-if="isDesktop">桌面端内容</div>

  <!-- 响应式类名 -->
  <div :class="{ 'mobile-style': isMobile, 'desktop-style': isDesktop }">
    响应式内容
  </div>
</template>
```

### 3. 响应式样式
```vue
<style scoped>
/* 移动端 */
@media (max-width: 768px) {
  .container {
    padding: 12px;
  }
}

/* 平板 */
@media (min-width: 768px) and (max-width: 1024px) {
  .container {
    padding: 16px;
  }
}

/* 桌面 */
@media (min-width: 1024px) {
  .container {
    padding: 20px;
  }
}
</style>
```

## 📐 断点速查

| 名称 | 宽度 | 设备 | 网格列数 |
|------|------|------|---------|
| xs | <480px | 手机竖屏 | 1-2列 |
| sm | 480-640px | 手机横屏 | 2列 |
| md | 640-768px | 小平板 | 2-3列 |
| lg | 768-1024px | 平板 | 3-4列 |
| xl | 1024-1280px | 小桌面 | 4-5列 |
| 2xl | >1280px | 大桌面 | 5-6列 |

## 🎨 常用模式

### 响应式网格
```javascript
const gridClasses = computed(() => {
  if (isMobile.value) return 'grid grid-cols-2 gap-4'
  if (isTablet.value) return 'grid grid-cols-3 gap-6'
  return 'grid grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-8'
})
```

### 响应式文本
```vue
<h1 :class="isMobile ? 'text-lg' : 'text-xl'">
  标题
</h1>
```

### 响应式组件属性
```vue
<n-button :size="isMobile ? 'small' : 'medium'">
  按钮
</n-button>

<n-input
  :placeholder="isMobile ? '搜索...' : '搜索动漫、角色或声优...'"
/>
```

## 🔧 实用代码片段

### 1. 响应式侧边栏
```vue
<n-layout-sider
  :collapsed-width="isMobile ? 0 : 64"
  :width="240"
  :collapsed="collapsed"
  :show-trigger="!isMobile"
  :class="{ 'mobile-sidebar': isMobile }"
/>
```

### 2. 移动端遮罩
```vue
<div
  v-if="isMobile && !collapsed"
  class="mobile-overlay"
  @click="collapsed = true"
/>

<style>
.mobile-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 999;
}
</style>
```

### 3. 响应式卡片间距
```vue
<n-card
  :bordered="!isMobile"
  :content-style="{ padding: isMobile ? '12px' : '20px' }"
/>
```

### 4. 条件加载组件
```vue
<!-- 移动端显示简化版 -->
<mobile-component v-if="isMobile" />

<!-- 桌面端显示完整版 -->
<desktop-component v-else />
```

## 📱 触摸优化

### 按钮大小
```css
/* 最小触摸目标：44x44px */
.touch-target {
  min-width: 44px;
  min-height: 44px;
}
```

### 取消默认行为
```css
/* 禁用长按选择 */
.no-select {
  -webkit-user-select: none;
  user-select: none;
  -webkit-touch-callout: none;
}

/* 移除点击高亮 */
.no-tap-highlight {
  -webkit-tap-highlight-color: transparent;
}
```

## 🎯 性能优化

### 1. 防抖 Resize
```javascript
import { debounce } from 'lodash-es'

const handleResize = debounce(() => {
  windowWidth.value = window.innerWidth
}, 200)
```

### 2. 条件资源加载
```vue
<script setup>
// 仅桌面端加载重量级组件
const HeavyComponent = isDesktop.value
  ? defineAsyncComponent(() => import('./HeavyComponent.vue'))
  : null
</script>
```

## 🐛 常见问题

### Q: 移动端侧边栏不显示？
A: 检查 `z-index` 是否足够高（>= 1000）

### Q: 响应式断点不生效？
A: 确保已在组件中导入并调用 `useResponsive()`

### Q: 滚动穿透问题？
A: 在遮罩层显示时添加 `overflow: hidden` 到 body

```javascript
watch(showOverlay, (val) => {
  document.body.style.overflow = val ? 'hidden' : ''
})
```

## 📚 完整文档

详见: `/RESPONSIVE_DESIGN.md`
