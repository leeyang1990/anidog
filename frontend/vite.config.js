import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  base: '/',
  plugins: [
    vue(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 3033,
    proxy: {
      // xfwd: 与生产 nginx 一致地转发 X-Forwarded-Host —— 后端用它做同源 CORS 判定，
      // 少了它本地开发环境所有带 Origin 的请求（登录、写操作）都会被判成跨站而 403。
      '/api': {
        target: process.env.VITE_API_URL || 'http://localhost:8088',
        changeOrigin: true,
        xfwd: true,
      },
      '/ws': {
        target: (process.env.VITE_API_URL || 'http://localhost:8088').replace('http', 'ws'),
        ws: true,
        xfwd: true,
      }
    }
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    assetsDir: 'assets',
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor': ['vue', 'vue-router', 'pinia'],
        }
      }
    }
  }
})
