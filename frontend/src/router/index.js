import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  {
    path: '/',
    component: () => import('../views/Layout/MainLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: () => import('../views/Dashboard.vue'),
      },
      {
        path: 'anime',
        name: 'AnimeList',
        component: () => import('../views/Anime/AnimeList.vue'),
      },
      {
        path: 'anime/:id',
        name: 'AnimeDetail',
        component: () => import('../views/Anime/AnimeDetail.vue'),
        props: true,
      },
      {
        path: 'rss',
        name: 'RSSList',
        component: () => import('../views/RSS/RSSList.vue'),
      },
      {
        path: 'downloads',
        name: 'Downloads',
        component: () => import('../views/Downloads/DownloadList.vue'),
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('../views/Settings/index.vue'),
      },
    ],
  },
  // 新的 Naive UI 布局路由
  {
    path: '/naive',
    component: () => import('../views/Layout/NaiveLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'NaiveDashboard',
        component: () => import('../views/Dashboard.vue'),
      },
      {
        path: 'anime',
        name: 'NaiveAnimeList',
        component: () => import('../views/Anime/NaiveAnimeList.vue'),
      },
      {
        path: 'anime/:id',
        name: 'NaiveAnimeDetail',
        component: () => import('../views/Anime/AnimeDetail.vue'),
        props: true,
      },
      {
        path: 'rss',
        name: 'NaiveRSSList',
        component: () => import('../views/RSS/RSSList.vue'),
      },
      {
        path: 'downloads',
        name: 'NaiveDownloads',
        component: () => import('../views/Downloads/DownloadList.vue'),
      },
      {
        path: 'settings',
        name: 'NaiveSettings',
        component: () => import('../views/Settings/index.vue'),
      },
    ],
  },
  {
    path: '/auth',
    component: () => import('../views/Layout/AuthLayout.vue'),
    children: [
      {
        path: 'login',
        name: 'Login',
        component: () => import('../views/Auth/Login.vue'),
      },
      {
        path: 'register',
        name: 'Register',
        component: () => import('../views/Auth/Register.vue'),
      },
    ],
  },
  // 新的 Naive UI 认证布局路由
  {
    path: '/auth/naive',
    component: () => import('../views/Layout/AuthLayout.vue'), // 可以复用原来的布局或创建新的
    children: [
      {
        path: 'login',
        name: 'NaiveLogin',
        component: () => import('../views/Auth/NaiveLogin.vue'),
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('../views/NotFound.vue'),
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 全局导航守卫
router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()
  const requiresAuth = to.matched.some(record => record.meta.requiresAuth)

  console.log('路由导航:', from.path, '->', to.path, '需要认证:', requiresAuth)
  console.log('当前认证状态:', authStore.isLoggedIn ? '已登录' : '未登录', '用户信息:', authStore.user ? '已加载' : '未加载')

  // 检查是否需要授权且用户未登录
  if (requiresAuth) {
    if (!authStore.isLoggedIn) {
      // 用户未登录，重定向到登录页面
      console.log('未登录，重定向到登录页面')
      // 根据路径判断使用哪个登录页面
      const loginRoute = from.path.includes('/naive') ? 'NaiveLogin' : 'Login'
      next({ name: loginRoute, query: { redirect: to.fullPath } })
    } else if (!authStore.user) {
      try {
        // 有令牌但没有用户信息，尝试获取用户信息
        console.log('路由守卫：尝试获取用户信息')
        await authStore.fetchUserInfo()
        console.log('获取用户信息成功，继续导航')
        next()
      } catch (error) {
        // 获取用户信息失败，可能是令牌无效
        console.error('路由守卫：获取用户信息失败', error)
        authStore.logout()
        const loginRoute = from.path.includes('/naive') ? 'NaiveLogin' : 'Login'
        next({ name: loginRoute, query: { redirect: to.fullPath } })
      }
    } else {
      // 用户已登录且有用户信息，正常导航
      console.log('已登录且有用户信息，正常导航')
      next()
    }
  } else {
    // 不需要授权的页面，正常导航
    console.log('页面不需要授权，正常导航')
    next()
  }
})

export default router 