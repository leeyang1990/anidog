# ANIDOG 番剧自动下载管理系统 - UI/UX设计规范

## 1. 设计理念与原则

### 1.1 核心设计理念
**"简约追番，极致体验"** - 将复杂的番剧管理简化为愉悦的视觉体验

### 1.2 设计价值观
- **动漫美学**：融入动漫文化元素，但保持专业性
- **效率至上**：最少的点击完成核心任务
- **视觉层次**：清晰的信息架构和视觉引导
- **情感连接**：通过微交互和动效增强用户情感体验

### 1.3 设计原则
1. **清晰性原则**：信息展示一目了然，避免认知负担
2. **一致性原则**：统一的设计语言贯穿全系统
3. **反馈性原则**：每个操作都有即时、明确的反馈
4. **容错性原则**：优雅处理错误，提供恢复路径
5. **可访问性原则**：考虑不同用户群体的使用需求

## 2. 颜色系统

### 2.1 品牌色彩
```css
/* 主色调 - 樱花粉 */
--primary-50: #fdf2f8;
--primary-100: #fce7f3;
--primary-200: #fbcfe8;
--primary-300: #f9a8d4;
--primary-400: #f472b6;
--primary-500: #ec4899; /* 主色 */
--primary-600: #db2777;
--primary-700: #be185d;
--primary-800: #9d174d;
--primary-900: #831843;

/* 辅助色 - 天空蓝 */
--secondary-50: #f0f9ff;
--secondary-100: #e0f2fe;
--secondary-200: #bae6fd;
--secondary-300: #7dd3fc;
--secondary-400: #38bdf8;
--secondary-500: #0ea5e9; /* 辅助色 */
--secondary-600: #0284c7;
--secondary-700: #0369a1;
--secondary-800: #075985;
--secondary-900: #0c4a6e;

/* 成功色 - 翡翠绿 */
--success-500: #10b981;

/* 警告色 - 琥珀黄 */
--warning-500: #f59e0b;

/* 错误色 - 朱砂红 */
--error-500: #ef4444;

/* 信息色 - 靛蓝 */
--info-500: #6366f1;
```

### 2.2 中性色彩
```css
/* 灰度系统 */
--gray-50: #f9fafb;
--gray-100: #f3f4f6;
--gray-200: #e5e7eb;
--gray-300: #d1d5db;
--gray-400: #9ca3af;
--gray-500: #6b7280;
--gray-600: #4b5563;
--gray-700: #374151;
--gray-800: #1f2937;
--gray-900: #111827;
```

### 2.3 深色模式配色
```css
/* 深色主题变量 */
.dark {
  --bg-primary: #0f0f0f;
  --bg-secondary: #1a1a1a;
  --bg-tertiary: #262626;
  --bg-elevated: #2a2a2a;
  
  --text-primary: #ffffff;
  --text-secondary: #a1a1aa;
  --text-tertiary: #71717a;
  
  --border-primary: #27272a;
  --border-secondary: #3f3f46;
}

/* 亮色主题变量 */
.light {
  --bg-primary: #ffffff;
  --bg-secondary: #f9fafb;
  --bg-tertiary: #f3f4f6;
  --bg-elevated: #ffffff;
  
  --text-primary: #111827;
  --text-secondary: #6b7280;
  --text-tertiary: #9ca3af;
  
  --border-primary: #e5e7eb;
  --border-secondary: #d1d5db;
}
```

## 3. 字体排版规范

### 3.1 字体家族
```css
/* 主字体栈 */
--font-sans: 'Inter', 'Noto Sans SC', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;

/* 等宽字体 */
--font-mono: 'JetBrains Mono', 'Source Code Pro', monospace;

/* 装饰字体（用于Logo等） */
--font-display: 'Bebas Neue', sans-serif;
```

### 3.2 字体大小规范
```css
/* 基于 Tailwind 的字体大小系统 */
--text-xs: 0.75rem;    /* 12px - 标签、辅助文字 */
--text-sm: 0.875rem;   /* 14px - 次要内容 */
--text-base: 1rem;     /* 16px - 正文 */
--text-lg: 1.125rem;   /* 18px - 强调文字 */
--text-xl: 1.25rem;    /* 20px - 小标题 */
--text-2xl: 1.5rem;    /* 24px - 标题 */
--text-3xl: 1.875rem;  /* 30px - 大标题 */
--text-4xl: 2.25rem;   /* 36px - 页面标题 */
```

### 3.3 行高与字重
```css
/* 行高 */
--leading-tight: 1.25;
--leading-normal: 1.5;
--leading-relaxed: 1.75;

/* 字重 */
--font-normal: 400;
--font-medium: 500;
--font-semibold: 600;
--font-bold: 700;
```

## 4. 组件设计规范（基于 Naive UI）

### 4.1 按钮组件
```vue
<!-- 主按钮 -->
<n-button type="primary" size="medium" round>
  <template #icon>
    <n-icon><download-icon /></n-icon>
  </template>
  开始下载
</n-button>

<!-- 样式定制 -->
<style>
/* 按钮尺寸 */
.n-button--small { height: 32px; padding: 0 12px; }
.n-button--medium { height: 36px; padding: 0 16px; }
.n-button--large { height: 40px; padding: 0 20px; }

/* 圆角风格 */
.n-button--round { border-radius: 18px; }
</style>
```

### 4.2 卡片组件
```vue
<!-- 番剧卡片 -->
<n-card 
  class="anime-card"
  hoverable
  :segmented="{
    content: true,
    footer: 'soft'
  }"
>
  <template #cover>
    <img class="anime-cover" :src="coverUrl" />
  </template>
  <template #header>
    <div class="anime-title">{{ title }}</div>
  </template>
  <template #footer>
    <n-space justify="space-between">
      <n-tag type="info">更新至{{ episode }}集</n-tag>
      <n-button text>管理</n-button>
    </n-space>
  </template>
</n-card>

<style>
.anime-card {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.anime-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 24px rgba(0, 0, 0, 0.1);
}
</style>
```

### 4.3 表单组件
```vue
<!-- 订阅表单 -->
<n-form
  :model="formData"
  :rules="rules"
  label-placement="left"
  label-width="100"
>
  <n-form-item label="RSS源" path="rssUrl">
    <n-input 
      v-model:value="formData.rssUrl"
      placeholder="输入RSS订阅地址"
      clearable
    />
  </n-form-item>
  
  <n-form-item label="过滤规则" path="filter">
    <n-select
      v-model:value="formData.filter"
      multiple
      :options="filterOptions"
      placeholder="选择过滤条件"
    />
  </n-form-item>
</n-form>
```

### 4.4 数据展示组件
```vue
<!-- 下载列表 -->
<n-data-table
  :columns="columns"
  :data="downloads"
  :pagination="pagination"
  :row-props="rowProps"
  striped
/>

<!-- 进度条 -->
<n-progress
  type="line"
  :percentage="downloadProgress"
  :height="20"
  :border-radius="10"
  :fill-border-radius="10"
>
  <div class="progress-text">
    {{ downloadSpeed }} MB/s
  </div>
</n-progress>
```

## 5. 布局栅格系统

### 5.1 响应式栅格
```css
/* 基于 12 列栅格系统 */
.container {
  width: 100%;
  margin: 0 auto;
  padding: 0 16px;
}

/* 响应式容器宽度 */
@media (min-width: 640px) {
  .container { max-width: 640px; }
}
@media (min-width: 768px) {
  .container { max-width: 768px; }
}
@media (min-width: 1024px) {
  .container { max-width: 1024px; }
}
@media (min-width: 1280px) {
  .container { max-width: 1280px; }
}
@media (min-width: 1536px) {
  .container { max-width: 1536px; }
}
```

### 5.2 间距系统
```css
/* 基于 4px 的间距系统 */
--space-1: 0.25rem;  /* 4px */
--space-2: 0.5rem;   /* 8px */
--space-3: 0.75rem;  /* 12px */
--space-4: 1rem;     /* 16px */
--space-5: 1.25rem;  /* 20px */
--space-6: 1.5rem;   /* 24px */
--space-8: 2rem;     /* 32px */
--space-10: 2.5rem;  /* 40px */
--space-12: 3rem;    /* 48px */
--space-16: 4rem;    /* 64px */
```

## 6. 图标和插画风格

### 6.1 图标系统
- **主要图标库**：Heroicons (Outline 风格为主)
- **辅助图标库**：Vicons (动漫特色图标)
- **图标尺寸**：16px, 20px, 24px, 32px
- **图标颜色**：跟随文字颜色或使用主题色

### 6.2 插画风格
- **风格定位**：扁平化矢量插画，带有动漫元素
- **色彩运用**：使用品牌色系，保持高饱和度
- **应用场景**：
  - 空状态页面
  - 引导页面
  - 加载动画
  - 成功/错误反馈

### 6.3 Logo设计
```
ANIDOG Logo 设计要素：
- 主体：可爱的柴犬形象（动漫风格）
- 元素：下载箭头巧妙融入尾巴设计
- 颜色：渐变色（从樱花粉到天空蓝）
- 字体：Bebas Neue 圆角处理
```

## 7. 交互设计规范

### 7.1 微交互动效
```css
/* 通用过渡动画 */
.transition-all {
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

/* 悬停效果 */
.hover-lift:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

/* 点击反馈 */
.click-scale:active {
  transform: scale(0.97);
}

/* 加载动画 */
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}
.loading {
  animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}
```

### 7.2 页面过渡
```vue
<!-- 路由过渡 -->
<router-view v-slot="{ Component }">
  <transition name="fade-slide" mode="out-in">
    <component :is="Component" />
  </transition>
</router-view>

<style>
.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: all 0.3s ease;
}
.fade-slide-enter-from {
  opacity: 0;
  transform: translateX(30px);
}
.fade-slide-leave-to {
  opacity: 0;
  transform: translateX(-30px);
}
</style>
```

### 7.3 交互反馈
- **即时反馈**：按钮点击、表单输入立即响应
- **进度提示**：长时间操作显示进度条或骨架屏
- **成功反馈**：完成操作后的动画确认
- **错误处理**：友好的错误提示和恢复建议

## 8. 响应式设计断点

### 8.1 断点定义
```css
/* 移动优先的断点系统 */
--screen-sm: 640px;   /* 手机横屏 */
--screen-md: 768px;   /* 平板竖屏 */
--screen-lg: 1024px;  /* 平板横屏/小笔记本 */
--screen-xl: 1280px;  /* 桌面显示器 */
--screen-2xl: 1536px; /* 大屏显示器 */
```

### 8.2 布局适配策略
- **移动端（<768px）**：
  - 单列布局
  - 底部导航
  - 全屏弹窗
  - 大触摸目标（最小44px）

- **平板端（768px-1024px）**：
  - 双列布局
  - 侧边栏可收缩
  - 弹窗居中显示

- **桌面端（>1024px）**：
  - 多列布局
  - 固定侧边栏
  - 悬浮操作面板
  - 快捷键支持

## 9. 动效设计指南

### 9.1 动效原则
- **目的性**：每个动效都服务于用户体验
- **快速响应**：动画时长控制在200-400ms
- **自然流畅**：使用缓动函数创造自然效果
- **性能优先**：优先使用transform和opacity

### 9.2 常用动效时长
```css
--duration-fast: 150ms;    /* 快速反馈 */
--duration-normal: 250ms;  /* 常规过渡 */
--duration-slow: 350ms;    /* 复杂动画 */
--duration-slower: 500ms;  /* 页面切换 */
```

### 9.3 缓动函数
```css
--ease-in: cubic-bezier(0.4, 0, 1, 1);
--ease-out: cubic-bezier(0, 0, 0.2, 1);
--ease-in-out: cubic-bezier(0.4, 0, 0.2, 1);
--ease-bounce: cubic-bezier(0.68, -0.55, 0.265, 1.55);
```

## 10. 具体页面线框图

### 10.1 仪表板页面
```
┌─────────────────────────────────────────────────┐
│ ┌─────────────────────────────────────────────┐ │
│ │ Logo  ANIDOG        搜索框...    头像 设置 │ │ <- 顶部导航栏
│ └─────────────────────────────────────────────┘ │
│                                                  │
│ ┌───────────┬─────────────────────────────────┐ │
│ │           │ ┌─────────┐ ┌─────────┐ ┌─────┐ │ │
│ │  仪表板   │ │ 今日更新 │ │ 下载中  │ │ 已完 │ │ <- 统计卡片
│ │  订阅管理 │ │   12    │ │    3    │ │  8  │ │ │
│ │  下载队列 │ └─────────┘ └─────────┘ └─────┘ │ │
│ │  媒体库   │                                  │ │
│ │  设置     │ ┌─────────────────────────────┐ │ │
│ │           │ │        最近更新              │ │ │
│ └───────────┤ │ ┌───┐ 番剧名称 S01E12   ▼  │ │ │
│             │ │ └───┘ 字幕组 1080P     85% │ │ │ <- 下载列表
│             │ │ ┌───┐ 番剧名称 S01E11   ✓  │ │ │
│             │ │ └───┘ 字幕组 1080P    完成 │ │ │
│             │ └─────────────────────────────┘ │ │
│             └─────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

### 10.2 订阅管理页面
```
┌─────────────────────────────────────────────────┐
│ ┌─────────────────────────────────────────────┐ │
│ │ 订阅管理                      + 添加订阅    │ │
│ └─────────────────────────────────────────────┘ │
│                                                  │
│ ┌─────────────────────────────────────────────┐ │
│ │ 搜索番剧...             筛选 ▼   视图切换   │ │
│ └─────────────────────────────────────────────┘ │
│                                                  │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐         │
│ │  封面图  │ │  封面图  │ │  封面图  │         │ <- 卡片视图
│ │          │ │          │ │          │         │
│ │ 番剧名称 │ │ 番剧名称 │ │ 番剧名称 │         │
│ │ 更新至12 │ │ 更新至8  │ │ 已完结   │         │
│ │ [管理]   │ │ [管理]   │ │ [管理]   │         │
│ └──────────┘ └──────────┘ └──────────┘         │
└─────────────────────────────────────────────────┘
```

### 10.3 下载管理页面
```
┌─────────────────────────────────────────────────┐
│ ┌─────────────────────────────────────────────┐ │
│ │ 下载管理      正在下载(3)  已完成(150)      │ │
│ └─────────────────────────────────────────────┘ │
│                                                  │
│ ┌─────────────────────────────────────────────┐ │
│ │ 全局设置：▼ 限速 2MB/s  同时下载数 3        │ │
│ └─────────────────────────────────────────────┘ │
│                                                  │
│ ┌─────────────────────────────────────────────┐ │
│ │ ┌──┐ 番剧名称 第12集                       │ │
│ │ └──┘ ████████████░░░░░░░░ 60% 1.2MB/s     │ │
│ │      剩余时间：5分钟    [暂停] [删除]       │ │
│ ├─────────────────────────────────────────────┤ │
│ │ ┌──┐ 番剧名称 第11集                       │ │
│ │ └──┘ ███████░░░░░░░░░░░░ 35% 0.8MB/s     │ │
│ │      剩余时间：12分钟   [暂停] [删除]       │ │
│ └─────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

### 10.4 设置页面
```
┌─────────────────────────────────────────────────┐
│ ┌─────────────────────────────────────────────┐ │
│ │ 系统设置                                     │ │
│ └─────────────────────────────────────────────┘ │
│                                                  │
│ ┌──────────┬──────────────────────────────────┐ │
│ │ 基础设置 │ 主题设置                         │ │
│ │ 下载设置 │ ○ 浅色模式  ● 深色模式  ○ 跟随系统│ │
│ │ 通知设置 │                                  │ │
│ │ 高级选项 │ 语言设置                         │ │
│ │ 关于     │ [中文简体 ▼]                     │ │
│ │          │                                  │ │
│ │          │ 数据存储路径                     │ │
│ │          │ [/Users/anime/] [浏览]           │ │
│ │          │                                  │ │
│ │          │            [保存设置]            │ │
│ └──────────┴──────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

## 11. 组件使用示例

### 11.1 订阅卡片组件
```vue
<template>
  <n-card 
    class="subscription-card rounded-xl overflow-hidden"
    :class="{ 'border-2 border-primary-500': isActive }"
    hoverable
  >
    <template #cover>
      <div class="relative">
        <img 
          :src="anime.cover" 
          class="w-full h-48 object-cover"
          :alt="anime.title"
        >
        <div class="absolute top-2 right-2">
          <n-tag 
            :type="anime.isAiring ? 'success' : 'default'"
            size="small"
            round
          >
            {{ anime.isAiring ? '连载中' : '已完结' }}
          </n-tag>
        </div>
      </div>
    </template>
    
    <div class="p-4">
      <h3 class="font-semibold text-lg mb-2 line-clamp-1">
        {{ anime.title }}
      </h3>
      <p class="text-sm text-gray-500 mb-3">
        更新至 {{ anime.latestEpisode }} 集
      </p>
      
      <n-space justify="space-between" align="center">
        <n-space size="small">
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-icon size="18" :color="themeVars.primaryColor">
                <rss-icon />
              </n-icon>
            </template>
            {{ anime.rssSource }}
          </n-tooltip>
          <span class="text-xs text-gray-400">
            {{ anime.updateTime }}
          </span>
        </n-space>
        
        <n-dropdown 
          trigger="click" 
          :options="menuOptions"
          @select="handleSelect"
        >
          <n-button text size="small">
            <n-icon size="18">
              <more-icon />
            </n-icon>
          </n-button>
        </n-dropdown>
      </n-space>
    </div>
  </n-card>
</template>
```

### 11.2 下载进度组件
```vue
<template>
  <div class="download-item bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
    <div class="flex items-start gap-4">
      <div class="w-16 h-16 rounded overflow-hidden flex-shrink-0">
        <img :src="download.cover" class="w-full h-full object-cover">
      </div>
      
      <div class="flex-1">
        <div class="flex justify-between items-start mb-2">
          <div>
            <h4 class="font-medium">{{ download.title }}</h4>
            <p class="text-sm text-gray-500">
              第 {{ download.episode }} 集 · {{ download.quality }}
            </p>
          </div>
          <n-tag 
            :type="statusType" 
            size="small"
            :bordered="false"
          >
            {{ statusText }}
          </n-tag>
        </div>
        
        <n-progress
          type="line"
          :percentage="download.progress"
          :show-indicator="false"
          :height="6"
          :border-radius="3"
          :fill-border-radius="3"
          class="mb-2"
        />
        
        <div class="flex justify-between items-center text-sm">
          <span class="text-gray-500">
            {{ download.downloadSpeed }} · 剩余 {{ download.eta }}
          </span>
          <n-space size="small">
            <n-button 
              text 
              size="tiny"
              @click="togglePause"
            >
              <n-icon size="16">
                <pause-icon v-if="!download.isPaused" />
                <play-icon v-else />
              </n-icon>
            </n-button>
            <n-button 
              text 
              size="tiny"
              type="error"
              @click="handleDelete"
            >
              <n-icon size="16">
                <trash-icon />
              </n-icon>
            </n-button>
          </n-space>
        </div>
      </div>
    </div>
  </div>
</template>
```

## 12. 深色模式适配

### 12.1 颜色变量切换
```css
/* 自动适配系统主题 */
@media (prefers-color-scheme: dark) {
  :root {
    --n-color-modal: rgba(24, 24, 28, 0.99);
    --n-color-popover: rgba(48, 48, 52, 0.99);
  }
}

/* Vue组件中的主题切换 */
const themeOverrides = computed(() => {
  return isDark.value ? {
    common: {
      primaryColor: '#ec4899',
      primaryColorHover: '#f472b6',
      primaryColorPressed: '#db2777',
    },
    Card: {
      color: 'rgba(24, 24, 28, 0.99)',
      borderColor: 'rgba(255, 255, 255, 0.09)',
    }
  } : null
})
```

### 12.2 图片和图标处理
```vue
<!-- 深色模式下的图片处理 -->
<img 
  :src="logoUrl" 
  :class="{ 'brightness-90 contrast-110': isDark }"
  alt="ANIDOG"
>

<!-- SVG图标颜色自适应 -->
<n-icon 
  :color="isDark ? '#ffffff' : '#111827'"
  size="20"
>
  <download-icon />
</n-icon>
```

## 13. 可访问性设计

### 13.1 键盘导航
- Tab键顺序逻辑清晰
- 支持方向键导航列表
- Esc键关闭弹窗
- Enter键确认操作

### 13.2 屏幕阅读器支持
```vue
<!-- ARIA标签使用 -->
<button
  aria-label="下载番剧第12集"
  aria-pressed="false"
  role="button"
>
  <span class="sr-only">下载</span>
  <download-icon />
</button>

<!-- 状态提示 -->
<div 
  role="status" 
  aria-live="polite"
  aria-label="下载进度60%"
>
  <n-progress :percentage="60" />
</div>
```

### 13.3 对比度要求
- 正文文字对比度 ≥ 4.5:1
- 大字体对比度 ≥ 3:1
- 交互元素对比度 ≥ 3:1

## 14. 性能优化建议

### 14.1 动画性能
```css
/* 使用 transform 而非 position */
.slide-in {
  transform: translateX(0);
  transition: transform 0.3s ease;
}

/* 使用 will-change 优化 */
.will-animate {
  will-change: transform, opacity;
}

/* 动画完成后移除 */
.animation-done {
  will-change: auto;
}
```

### 14.2 图片优化
- 使用懒加载
- 提供多种尺寸
- 使用 WebP 格式
- 实现渐进式加载

## 15. 设计交付规范

### 15.1 文件命名规范
```
/design
  /components
    - button.vue
    - card.vue
    - form.vue
  /layouts
    - default-layout.vue
    - mobile-layout.vue
  /styles
    - variables.css
    - utilities.css
    - animations.css
```

### 15.2 注释规范
```vue
<!--
  订阅卡片组件
  用途：展示番剧订阅信息
  props: 
    - anime: Object 番剧信息对象
    - isActive: Boolean 是否激活状态
  emits:
    - select: 选择番剧
    - manage: 管理订阅
-->
```

### 15.3 设计token
```javascript
// design-tokens.js
export const tokens = {
  color: {
    primary: '#ec4899',
    secondary: '#0ea5e9',
    // ...
  },
  spacing: {
    xs: '0.25rem',
    sm: '0.5rem',
    // ...
  },
  animation: {
    fast: '150ms',
    normal: '250ms',
    // ...
  }
}
```

---

## 设计文档版本信息

- **版本**: 1.0.0
- **创建日期**: 2025-01-03
- **设计师**: AI UI/UX Designer
- **技术栈**: Vue 3 + Naive UI + Tailwind CSS
- **设计工具**: Figma (推荐)

本设计规范为ANIDOG番剧自动下载管理系统提供完整的视觉和交互指导，确保产品在各个平台上提供一致且优质的用户体验。设计充分考虑了动漫爱好者的审美偏好，在保持专业性的同时融入了趣味性元素。