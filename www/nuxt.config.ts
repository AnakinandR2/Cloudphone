// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2024-11-01',
  devtools: { enabled: true },

  // 用户认证：SSR 侧通过 Cookie 调后端 /user/me 拉登录态。
  //  - backendBaseUrl：仅服务端用；nginx 后通常是 http://localhost:9981/api/v1
  //  - public.userCookieName：与后端 USER_JWT_COOKIE_NAME 一致（默认 user_token）
  //  - public.myAppPath：「进入控制台」按钮跳转目标
  // 可用同名大写环境变量覆盖：NUXT_BACKEND_BASE_URL / NUXT_PUBLIC_USER_COOKIE_NAME / NUXT_PUBLIC_MY_APP_PATH
  runtimeConfig: {
    backendBaseUrl: 'http://localhost:9981/api/v1',
    // 内容中台 Pub API（服务端私有，勿放入 public —— 密钥不得进入浏览器）。
    //  - pubBaseUrl    ← NUXT_PUB_BASE_URL（如 http://192.168.10.110:9981/api/v1/pub）
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
    port: 3000,
  },

  modules: [
    '@nuxtjs/tailwindcss',
    '@nuxtjs/color-mode',
    '@nuxtjs/i18n',
  ],

  // Scalar 只在 ScalarDoc.client.vue（仅客户端）里 import，Vite 首次扫描发现不到，
  // 会在运行时按需 optimize 并触发「504 Outdated Optimize Dep」。预先 include，
  // 让 dev 启动即预打包，避免按需重优化导致的 504。仅影响 dev，不影响生产构建。
  vite: {
    optimizeDeps: { include: ['@scalar/api-reference'] },
  },

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
