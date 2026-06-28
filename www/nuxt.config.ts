// https://nuxt.com/docs/api/configuration/nuxt-config
import process from 'node:process'
import { copyFileSync, existsSync, mkdirSync } from 'node:fs'

// 把 Scalar 的 standalone 浏览器构建拷到 public/，以 <script> 方式自托管加载，
// 让它彻底脱离 Vite 模块图——规避 504 Outdated Optimize Dep、Pre-transform
// "Maximum call stack size exceeded" 等大依赖在 dev 下的预转换/预打包问题。
// 在 dev/build/prepare 每次加载配置时执行，确保产物存在（文件本身 gitignore）。
try {
  const scalarSrc = 'node_modules/@scalar/api-reference/dist/browser/standalone.js'
  if (existsSync(scalarSrc)) {
    mkdirSync('public/vendor', { recursive: true })
    copyFileSync(scalarSrc, 'public/vendor/scalar-standalone.js')
  }
} catch {}

export default defineNuxtConfig({
  compatibilityDate: '2024-11-01',
  devtools: { enabled: true },

  // 用户认证：SSR 侧通过 Cookie 调后端 /user/me 拉登录态。
  //  - backendBaseUrl：仅服务端用；nginx 后通常是 http://localhost:9981/api/v1
  //  - public.userCookieName：与后端 USER_JWT_COOKIE_NAME 一致（默认 user_token）
  //  - public.myAppPath：登录/注册/控制台跳转的地址前缀。默认 /my/（同源部署）；
  //      跨域部署时直接填完整地址（如 https://my.example.com/my 或开发态 http://127.0.0.1:5666/my）。
  // 可用同名大写环境变量覆盖：NUXT_BACKEND_BASE_URL / NUXT_PUBLIC_USER_COOKIE_NAME / NUXT_PUBLIC_MY_APP_PATH
  runtimeConfig: {
    backendBaseUrl: 'http://localhost:9981/api/v1',
    // 后端公开营销接口基址（免鉴权，/api/open/v1）。供 /pricing 价格页 SSR 取数。
    // 覆盖：NUXT_BACKEND_PUBLIC_URL（如 http://localhost:9981/api/open/v1）。
    backendPublicUrl: 'http://localhost:9981/api/open/v1',
    // 内容中台 Pub API（服务端私有，勿放入 public —— 密钥不得进入浏览器）。
    //  - pubBaseUrl    ← NUXT_PUB_BASE_URL（如 http://192.168.10.110:9981/api/open/v1）
    //  - contentApiKey ← NUXT_CONTENT_API_KEY（cp_ 开头的只读密钥）
    //  - contentSpace  博客内容空间 slug
    pubBaseUrl: '',
    contentApiKey: '',
    contentSpace: 'blog',
    public: {
      userCookieName: 'user_token',
      myAppPath: '/my/',
    },
  },

  // Listen on all network interfaces (LAN access / containers).
  devServer: {
    host: '0.0.0.0',
    // 开发服务端口：从环境变量 NUXT_DEV_PORT 读取，缺省 3000（Nuxt 在评估 config 前自动载入 .env）。
    port: Number(process.env.NUXT_DEV_PORT) || 3000,
  },

  modules: [
    '@nuxtjs/tailwindcss',
    '@nuxtjs/color-mode',
    '@nuxtjs/i18n',
  ],

  css: ['~/assets/css/tailwind.css'],

  // Register auto-imported components by filename (ignore nested folder prefix),
  // but skip the shadcn-vue `ui/` primitives — those are imported explicitly.
  components: [
    { path: '~/components', pathPrefix: false, ignore: ['**/ui/**'] },
  ],

  app: {
    head: {
      htmlAttrs: { lang: 'en' },
      title: 'Gloryphone — 海外社媒与跨境电商的云手机平台',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        {
          name: 'description',
          content:
            'Gloryphone — a cloud phone platform built for overseas social and cross-border e-commerce teams. Dedicated IP, unique device fingerprint, 24/7 online.',
        },
      ],
      link: [
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Manrope:wght@400;500;600;700;800&family=JetBrains+Mono:wght@400;500&display=swap',
        },
      ],
    },
  },

  colorMode: {
    classSuffix: '',
    preference: 'light',
    fallback: 'light',
    storageKey: 'gp-color-mode',
  },

  i18n: {
    strategy: 'no_prefix',
    defaultLocale: 'en',
    bundle: {
      optimizeTranslationDirective: false,
    },
    locales: [
      { code: 'en', name: 'English', file: 'en.json' },
      { code: 'zh', name: '简体中文', file: 'zh.json' },
    ],
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'gp-lang',
      redirectOn: 'root',
      alwaysRedirect: true,
    },
  },
})
