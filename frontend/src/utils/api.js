import { useAuthStore } from '@/stores/auth'

const API_BASE_URL = '/api/v1'
const MAX_RETRIES = 3
const RETRY_DELAY = 1000 // 1秒

class ApiError extends Error {
  constructor(message, status, data) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.data = data
  }
}

// 基础请求函数
async function request(endpoint, options = {}, retryCount = 0) {
  const authStore = useAuthStore()

  // 非公开端点要求有 token；否则直接跳登录，避免反复送无头请求
  const isPublic = endpoint.startsWith('/auth/login') || endpoint.startsWith('/auth/register')
  if (!isPublic && !authStore.token) {
    // 清理本地状态 + 跳转登录
    authStore.logout()
    if (typeof window !== 'undefined' && !window.location.pathname.startsWith('/auth/')) {
      const next = window.location.pathname + window.location.search
      window.location.replace('/auth/login?redirect=' + encodeURIComponent(next))
    }
    throw new ApiError('未登录，请先登录', 401)
  }

  // 添加认证头
  const headers = {
    'Content-Type': 'application/json',
    ...options.headers
  }

  if (authStore.token) {
    headers['Authorization'] = `Bearer ${authStore.token}`
  }

  try {
    const response = await fetch(`${API_BASE_URL}${endpoint}`, {
      ...options,
      headers
    })

    // 检查响应状态
    if (!response.ok) {
      // 如果是认证错误，尝试刷新令牌
      if (response.status === 401 && authStore.token) {
        if (retryCount < MAX_RETRIES) {
          // 等待一段时间后重试
          await new Promise(resolve => setTimeout(resolve, RETRY_DELAY))
          // 尝试刷新令牌
          try {
            await authStore.refreshToken()
            // 重试请求
            return await request(endpoint, options, retryCount + 1)
          } catch (error) {
            // 如果刷新令牌失败，清除认证状态并抛出错误
            authStore.logout()
            throw new ApiError('认证已过期，请重新登录', 401)
          }
        }
      }

      const responseText = await response.text()
      let errorData
      try {
        errorData = JSON.parse(responseText)
      } catch (e) {
        // 如果解析失败，使用原始文本
        errorData = { detail: responseText }
      }

      throw new ApiError(
        errorData.detail || '请求失败',
        response.status,
        errorData
      )
    }

    const responseText = await response.text()
    let data
    try {
      data = JSON.parse(responseText)
    } catch (e) {
      throw new ApiError(
        '服务器返回的不是有效的JSON格式',
        response.status,
        { responseText }
      )
    }

    return data
  } catch (error) {
    if (error instanceof ApiError) {
      throw error
    }

    // 网络错误重试
    if (retryCount < MAX_RETRIES) {
      await new Promise(resolve => setTimeout(resolve, RETRY_DELAY))
      return await request(endpoint, options, retryCount + 1)
    }

    throw new ApiError(
      '网络请求失败，请检查网络连接',
      0,
      { originalError: error }
    )
  }
}

// 导出各种请求方法
export async function get(endpoint, options = {}) {
  // 处理查询参数
  let url = endpoint
  if (options.params) {
    const searchParams = new URLSearchParams()
    Object.keys(options.params).forEach(key => {
      if (options.params[key] !== undefined && options.params[key] !== null) {
        searchParams.append(key, options.params[key])
      }
    })
    const queryString = searchParams.toString()
    if (queryString) {
      url += (endpoint.includes('?') ? '&' : '?') + queryString
    }
  }
  
  return await request(url, {
    ...options,
    method: 'GET'
  })
}

export async function post(endpoint, data, options = {}) {
  return await request(endpoint, {
    ...options,
    method: 'POST',
    body: JSON.stringify(data)
  })
}

export async function put(endpoint, data, options = {}) {
  return await request(endpoint, {
    ...options,
    method: 'PUT',
    body: JSON.stringify(data)
  })
}

export async function del(endpoint, options = {}) {
  return await request(endpoint, {
    ...options,
    method: 'DELETE'
  })
}

// 导出完整的 API 对象
export const api = {
  get,
  post,
  put,
  delete: del
}

// 导出错误类
export { ApiError } 