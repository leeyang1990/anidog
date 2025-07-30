import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
// 不要在这里导入api.js，避免循环依赖

export const useAuthStore = defineStore('auth', {
  // 使用选项API而不是组合API，避免可能的响应式问题
  state: () => ({
    token: localStorage.getItem('token') || null,
    refreshToken: localStorage.getItem('refreshToken') || null,
    user: null,
    isLoggedIn: !!localStorage.getItem('token')
  }),
  
  actions: {
    async login({ username, password }) {
      try {
        // 登录请求使用原生fetch，因为api.js依赖于authStore
        const response = await fetch('/api/v1/auth/login', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/x-www-form-urlencoded'
          },
          body: new URLSearchParams({
            username,
            password
          })
        })

        if (!response.ok) {
          const error = await response.json()
          throw new Error(error.detail || '登录失败')
        }

        const data = await response.json()
        console.log('登录响应:', data)
        
        // 确保令牌格式正确
        if (!data.access_token) {
          throw new Error('服务器返回的令牌格式不正确')
        }
        
        this.setToken(data.access_token)
        if (data.refresh_token) {
          this.setRefreshToken(data.refresh_token)
        }
        
        // 获取用户信息
        await this.fetchUserInfo()
        
        return true
      } catch (error) {
        console.error('登录失败:', error)
        throw error
      }
    },

    async refreshToken() {
      if (!this.refreshToken) {
        throw new Error('没有可用的刷新令牌')
      }

      try {
        const response = await fetch('/api/v1/auth/refresh', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json'
          },
          body: JSON.stringify({
            refresh_token: this.refreshToken
          })
        })

        if (!response.ok) {
          throw new Error('刷新令牌失败')
        }

        const data = await response.json()
        if (!data.access_token) {
          throw new Error('服务器返回的令牌格式不正确')
        }

        this.setToken(data.access_token)
        if (data.refresh_token) {
          this.setRefreshToken(data.refresh_token)
        }

        return data.access_token
      } catch (error) {
        console.error('刷新令牌失败:', error)
        this.logout()
        throw error
      }
    },

    async fetchUserInfo() {
      if (!this.token) {
        console.error('无法获取用户信息：没有认证令牌')
        return null
      }
      
      try {
        console.log('获取用户信息，使用令牌:', this.token.substring(0, 10) + '...')
        
        // 使用原生fetch，避免循环依赖
        const response = await fetch('/api/v1/users/me', {
          headers: {
            'Authorization': `Bearer ${this.token}`,
            'Content-Type': 'application/json'
          }
        })

        if (!response.ok) {
          console.error('获取用户信息失败，状态码:', response.status)
          throw new Error('获取用户信息失败')
        }

        const userData = await response.json()
        console.log('获取到的用户信息:', userData)
        this.setUser(userData)
        return userData
      } catch (error) {
        console.error('获取用户信息失败:', error)
        throw error
      }
    },

    setToken(newToken) {
      if (!newToken) {
        console.warn('尝试设置空令牌')
        this.token = null
        this.isLoggedIn = false
        localStorage.removeItem('token')
        return
      }
      
      console.log('设置新令牌:', newToken.substring(0, 10) + '...')
      this.token = newToken
      this.isLoggedIn = true
      localStorage.setItem('token', newToken)
    },

    setRefreshToken(newToken) {
      if (!newToken) {
        localStorage.removeItem('refreshToken')
        this.refreshToken = null
        return
      }
      
      this.refreshToken = newToken
      localStorage.setItem('refreshToken', newToken)
    },

    setUser(newUser) {
      this.user = newUser
    },

    logout() {
      console.log('执行登出操作')
      this.token = null
      this.refreshToken = null
      this.user = null
      this.isLoggedIn = false
      localStorage.removeItem('token')
      localStorage.removeItem('refreshToken')
    }
  }
}) 