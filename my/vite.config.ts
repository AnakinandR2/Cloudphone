import process from 'node:process'
import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig, loadEnv } from 'vite'
import { vitePluginFakeServer } from 'vite-plugin-fake-server'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const enableMock = env.VITE_APP_MOCK === 'true'
  // base 决定打包资源前缀，并经 import.meta.env.BASE_URL 喂给 vue-router。
  // 与 nginx 把 /my 透传给本 SPA 配套：保持 /my/ 前缀。单独 dev 访问 http://localhost:5666/my/。
  // 想恢复根路径可设 VITE_BASE=/。
  const base = env.VITE_BASE || '/my/'
  return {
    base,
    plugins: [
      vue(),
      tailwindcss(),
      // Mock 服务：拦截 basename(/mock-api) 下的请求，独立于后端代理
      vitePluginFakeServer({
        logger: true,
        include: 'src/mock',
        infixName: false,
        basename: 'mock-api',
        enableProd: false,
        enableDev: enableMock,
      }),
    ],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      port: 5666,
      strictPort: false, 
      host: true,
      // 开发环境放开所有 Host 头校验（反代/自定义域名访问，如 vibe06.u4a.cn）
      allowedHosts: true,
      // HMR 客户端路径跟 base 对齐到 /my/，让 nginx 的 location /my 走 WS Upgrade。
      hmr: {
        path: '/my/',
      },
      proxy: {
        // 真实后端 Go/Gin 服务（BasePath /api/v1，端口 9981）。开启 Mock 时走 /mock-api，不经此代理。
        '/api': {
          target: env.VITE_API_TARGET || 'http://localhost:9981',
          changeOrigin: true,
        },
      },
    },
  }
})
