import { ref, computed, onMounted, onUnmounted } from 'vue'

/**
 * 响应式断点检测组合函数
 * 提供统一的响应式状态检测
 */
export function useResponsive() {
  // 窗口宽度
  const windowWidth = ref(window.innerWidth)
  const windowHeight = ref(window.innerHeight)

  // 断点定义（与 Tailwind CSS 保持一致）
  const breakpoints = {
    xs: 480,   // 超小屏幕（手机竖屏）
    sm: 640,   // 小屏幕（手机横屏）
    md: 768,   // 中等屏幕（平板竖屏）
    lg: 1024,  // 大屏幕（平板横屏/小笔记本）
    xl: 1280,  // 超大屏幕（桌面）
    '2xl': 1536 // 超超大屏幕（大桌面）
  }

  // 设备类型判断
  const isMobile = computed(() => windowWidth.value < breakpoints.md)
  const isTablet = computed(() => windowWidth.value >= breakpoints.md && windowWidth.value < breakpoints.lg)
  const isDesktop = computed(() => windowWidth.value >= breakpoints.lg)

  // 更精细的断点判断
  const isXs = computed(() => windowWidth.value < breakpoints.xs)
  const isSm = computed(() => windowWidth.value >= breakpoints.xs && windowWidth.value < breakpoints.sm)
  const isMd = computed(() => windowWidth.value >= breakpoints.sm && windowWidth.value < breakpoints.md)
  const isLg = computed(() => windowWidth.value >= breakpoints.md && windowWidth.value < breakpoints.lg)
  const isXl = computed(() => windowWidth.value >= breakpoints.lg && windowWidth.value < breakpoints.xl)
  const is2xl = computed(() => windowWidth.value >= breakpoints.xl)

  // 屏幕方向
  const isPortrait = computed(() => windowHeight.value > windowWidth.value)
  const isLandscape = computed(() => windowHeight.value <= windowWidth.value)

  // 触摸设备检测
  const isTouchDevice = computed(() => {
    return 'ontouchstart' in window || navigator.maxTouchPoints > 0
  })

  // 更新窗口尺寸
  const handleResize = () => {
    windowWidth.value = window.innerWidth
    windowHeight.value = window.innerHeight
  }

  // 生命周期钩子
  onMounted(() => {
    window.addEventListener('resize', handleResize)
    // 初始化时触发一次
    handleResize()
  })

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize)
  })

  return {
    // 窗口尺寸
    windowWidth,
    windowHeight,

    // 设备类型
    isMobile,
    isTablet,
    isDesktop,

    // 断点
    isXs,
    isSm,
    isMd,
    isLg,
    isXl,
    is2xl,

    // 屏幕方向
    isPortrait,
    isLandscape,

    // 触摸设备
    isTouchDevice,

    // 断点值
    breakpoints
  }
}

/**
 * 响应式网格列数计算
 */
export function useResponsiveGrid(options = {}) {
  const {
    xs = 1,
    sm = 2,
    md = 3,
    lg = 4,
    xl = 5,
    xxl = 6
  } = options

  const { windowWidth } = useResponsive()

  const columns = computed(() => {
    const width = windowWidth.value
    if (width < 480) return xs
    if (width < 640) return sm
    if (width < 768) return md
    if (width < 1024) return lg
    if (width < 1280) return xl
    return xxl
  })

  const gridClass = computed(() => {
    return `grid grid-cols-${columns.value}`
  })

  return {
    columns,
    gridClass
  }
}
