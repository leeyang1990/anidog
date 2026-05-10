import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  {
    path: '/',
    component: () => import('../views/Layout/NaiveLayout.vue'),
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
        component: () => import('../views/Anime/NaiveAnimeList.vue'),
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
        component: () => import('../views/RSS/RSSManagement.vue'),
      },
      {
        path: 'downloads',
        name: 'Downloads',
        component: () => import('../views/Downloads/DownloadList.vue'),
      },
      {
        path: 'calendar',
        name: 'Calendar',
        component: () => import('../views/Calendar/index.vue'),
      },
      {
        path: 'search',
        name: 'Search',
        component: () => import('../views/Search/index.vue'),
      },
      {
        path: 'anime-library',
        name: 'AnimeLibrary',
        component: () => import('../views/Anime/AnimeLibrary.vue'),
      },
      {
        path: 'anime-library/:id',
        name: 'BangumiDetail',
        component: () => import('../views/Anime/AnimeDetail.vue'),
        props: true,
      },
      {
        path: 'stream-rules',
        name: 'StreamRules',
        component: () => import('../views/StreamRules/index.vue'),
      },
      {
        path: 'notifications',
        name: 'Notifications',
        component: () => import('../views/Notification/index.vue'),
      },
      {
        path: 'settings',
        name: 'Settings',
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
        component: () => import('../views/Auth/NaiveLogin.vue'),
      },
      {
        path: 'register',
        name: 'Register',
        component: () => import('../views/Auth/Register.vue'),
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

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()
  const requiresAuth = to.matched.some(record => record.meta.requiresAuth)

  if (requiresAuth) {
    if (!authStore.isLoggedIn) {
      next({ name: 'Login', query: { redirect: to.fullPath } })
    } else if (!authStore.user) {
      try {
        await authStore.fetchUserInfo()
        next()
      } catch (error) {
        authStore.logout()
        next({ name: 'Login', query: { redirect: to.fullPath } })
      }
    } else {
      next()
    }
  } else {
    next()
  }
})

export default router
