import assert from 'node:assert/strict'
import { createPinia, setActivePinia } from 'pinia'

globalThis.localStorage = {
  values: new Map(),
  getItem(key) { return this.values.get(key) ?? null },
  setItem(key, value) { this.values.set(key, String(value)) },
  removeItem(key) { this.values.delete(key) },
}

setActivePinia(createPinia())
const { useAuthStore } = await import('../src/stores/auth.js')
const store = useAuthStore()

assert.equal(typeof store.refreshAccessToken, 'function')
store.setRefreshToken('refresh-old')
assert.equal(store.refreshTokenValue, 'refresh-old')
assert.equal(typeof store.refreshAccessToken, 'function', 'setting token must not overwrite the action')

let refreshCalls = 0
globalThis.fetch = async (url, options) => {
  assert.equal(url, '/api/v1/auth/refresh')
  refreshCalls++
  assert.deepEqual(JSON.parse(options.body), { refresh_token: 'refresh-old' })
  await new Promise(resolve => setTimeout(resolve, 5))
  return {
    ok: true,
    status: 200,
    text: async () => JSON.stringify({ access_token: 'access-new', refresh_token: 'refresh-new' }),
  }
}

await Promise.all([
  store.refreshAccessToken(),
  store.refreshAccessToken(),
  store.refreshAccessToken(),
])
assert.equal(refreshCalls, 1, 'concurrent 401 responses should share one refresh request')
assert.equal(store.token, 'access-new')
assert.equal(store.refreshTokenValue, 'refresh-new')

store.setToken('access-expired')
store.setRefreshToken('refresh-valid')
const calls = []
globalThis.fetch = async (url, options = {}) => {
  calls.push({ url, authorization: options.headers?.Authorization })
  if (url === '/api/v1/users/me' && calls.filter(call => call.url === url).length === 1) {
    return { ok: false, status: 401, text: async () => '{}' }
  }
  if (url === '/api/v1/auth/refresh') {
    assert.deepEqual(JSON.parse(options.body), { refresh_token: 'refresh-valid' })
    return {
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ access_token: 'access-rotated', refresh_token: 'refresh-rotated' }),
    }
  }
  if (url === '/api/v1/users/me') {
    assert.equal(options.headers.Authorization, 'Bearer access-rotated')
    return {
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ id: 1, username: 'admin' }),
    }
  }
  throw new Error(`unexpected URL: ${url}`)
}

const user = await store.fetchUserInfo()
assert.equal(user.username, 'admin')
assert.deepEqual(calls.map(call => call.url), [
  '/api/v1/users/me',
  '/api/v1/auth/refresh',
  '/api/v1/users/me',
])

console.log('auth store refresh tests passed')
