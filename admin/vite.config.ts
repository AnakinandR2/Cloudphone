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
  return {
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
      host: true,
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
