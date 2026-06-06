# 统一域名整合与配套改造笔记

把 Go 后端 + 营销站（Nuxt SSR）+ 用户后台（Vite SPA）+ 后台管理（Vite SPA）整合到**同一域名**下，使浏览器 Cookie 在三端自动共享、营销站能 SSR 拿到用户登录态、各 SPA 共用一个 `/api` 后端。本笔记把改动按模块整理，含每条改动的文件路径、代码片段、坑与原因，方便迁移到其他工程。

---

## 0. 总览

| 角色 | 技术栈 | 说明 |
| --- | --- | --- |
| `backend/` | Go + Gin + GORM | 单一后端，所有接口走 `/api/v1/...`，HttpOnly Cookie 鉴权 |
| `www/` | Nuxt 3 SSR | 营销站，根路径 `/`，SSR 阶段读 Cookie 调后端识别用户 |
| `my/` | Vue3 + Vite | 用户后台 SPA，挂在 `/my/` 子路径下 |
| nginx | — | 统一域名网关，按路径分流到三个上游 |

核心思路：
1. **同源 Cookie**：浏览器把 `Set-Cookie: Path=/` 在 www、my、`/api` 间天然共享，不需要 CORS 也不用跨域配置。
2. **/my 子路径承载 SPA**：my 的 `vite.base = '/my/'`、`vue-router base = /my/`、nginx `location /my` 透传，整套不需要 rewrite。
3. **www 在 SSR 阶段透传 Cookie**：用户访问营销站时，Nuxt server 上的 `$fetch` 把浏览器 Cookie 一并发给后端 `/user/me`，直接拿到当前用户，导航条按登录状态切换。

---

## 1. 后端：`TRUSTED_PROXIES` 取真实客户端 IP

> **目的**：access_log / 风控记录的"客户端 IP"在反代后正确读到 `X-Forwarded-For`/`X-Real-IP` 而不是 nginx 自身。

### 1.1 `framework/config.go`：加配置字段

```go
type Config struct {
    // ...
    CORSAllowedOrigins []string

    // TrustedProxies 信任的反向代理 IP / CIDR 列表（逗号分隔，从 TRUSTED_PROXIES 解析）。
    // 仅当请求的 RemoteAddr 在该列表内时，gin 才会解析 X-Forwarded-For / X-Real-IP 来确定真实
    // 客户端 IP（c.ClientIP()，写入 access_logs.client_ip）。
    // 为空时回退到 gin 默认行为：信任所有来源（"0.0.0.0/0"），任何客户端都可通过伪造 XFF 头篡改
    // 日志中的客户端 IP——仅适合本地开发，生产环境务必显式配置为你的反代/网关 IP 段。
    TrustedProxies []string
}

func LoadConfig() *Config {
    cfg := &Config{
        // ...
        CORSAllowedOrigins:   getEnvAsList("CORS_ALLOWED_ORIGINS"),
        TrustedProxies:       getEnvAsList("TRUSTED_PROXIES"),
        // ...
    }

    log.Printf("[config] CORSAllowedOrigins=%v  AccessLogUserEnabled=%v  TrustedProxies=%v",
        corsDisplay(cfg.CORSAllowedOrigins), cfg.AccessLogUserEnabled, trustedProxiesDisplay(cfg.TrustedProxies))

    if len(cfg.TrustedProxies) == 0 {
        log.Println("[config] ⚠️  TRUSTED_PROXIES 未配置，将信任所有来源；任何客户端都可伪造 X-Forwarded-For 篡改 access_logs.client_ip（仅建议开发环境）")
    }
    // ...
}

func trustedProxiesDisplay(proxies []string) string {
    if len(proxies) == 0 {
        return "(信任所有/dev)"
    }
    return strings.Join(proxies, ",")
}
```

### 1.2 `main.go`：启动时收紧

```go
gin.SetMode(framework.AppConfig.GinMode)
r := gin.Default()
// 仅在显式配置 TRUSTED_PROXIES 时收紧；为空保持 gin 默认（信任所有，开发期方便、生产期不安全）。
// c.ClientIP() 会按此列表判断是否解析 X-Forwarded-For / X-Real-IP。
if len(framework.AppConfig.TrustedProxies) > 0 {
    if err := r.SetTrustedProxies(framework.AppConfig.TrustedProxies); err != nil {
        log.Fatalf("SetTrustedProxies 失败: %v", err)
    }
}
```

### 1.3 `.env.example`：详细说明（直接拷过去）

```bash
# ── TRUSTED_PROXIES：信任的反向代理 IP / CIDR 列表（逗号分隔） ──────────────────
#
# 原理：
#   c.ClientIP() 的判定顺序：
#     1. 看 TCP 连接 RemoteAddr 是否落在 TRUSTED_PROXIES 内；
#     2. 是 → 按顺序读 X-Forwarded-For、X-Real-IP，取 XFF 最左侧合法 IP；
#     3. 否 → 一律回退到 RemoteAddr。
#
# 风险：
#   留空 → gin 默认信任所有 (0.0.0.0/0)，客户端可伪造 XFF 篡改日志 IP、绕过 IP 风控。
#
# 怎么填：
#   - 直接暴露公网：填 127.0.0.1（仅本机，等同禁用 XFF）
#   - 内网反代：TRUSTED_PROXIES=127.0.0.1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
#   - 公有云 CDN：填厂商回源 IP 段
#   - 本地开发：留空即可
TRUSTED_PROXIES=
```

### 1.4 注意

- **空值默认放行**是为了开发期便利。生产部署前**必须**显式填，不然 access_log 里的 IP 会被任意客户端写。
- gin 没有"信任部分头但不信任全部"这种粒度，整套是 all-or-nothing：要么 IP 在 TrustedProxies → 完全相信 XFF，要么不在 → 完全忽略 XFF。

---

## 2. nginx：统一域名网关

> **目的**：浏览器从同一域名访问全部前端 + 后端，Cookie 自动共享，无需 CORS。

`ops/nginx.conf.example`：

```nginx
upstream www_app      { server 127.0.0.1:3000; keepalive 16; }
upstream my_spa       { server 127.0.0.1:5666; keepalive 16; }
upstream backend_api  { server 127.0.0.1:9981; keepalive 16; }

# 让上游拿到真实 XFF；后端 TRUSTED_PROXIES 要包含本 nginx 的 IP/网段
map $http_x_forwarded_for $proxy_xff {
    default       "$http_x_forwarded_for, $remote_addr";
    ""            "$remote_addr";
}

server {
    listen       80;
    server_name  app.example.com;
    client_max_body_size 16m;

    # ── /api：后端接口 ──
    location /api/ {
        proxy_pass         http://backend_api;
        proxy_http_version 1.1;
        proxy_set_header   Host              $host;
        proxy_set_header   Connection        "";
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_xff;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_set_header   X-Forwarded-Host  $host;
        proxy_read_timeout 60s;
        proxy_send_timeout 60s;
    }

    # ── /my：用户后台 SPA ──
    # proxy_pass 后**不带斜杠或路径**，nginx 原样把 /my/... 转给上游。
    # 因为 my 的 vite.base='/my/'，HTML 引用 /my/assets/...、vue-router 也用 /my/，
    # 上游收到的路径与 base 对齐，无需 rewrite。
    location /my {
        proxy_pass         http://my_spa;
        proxy_http_version 1.1;
        proxy_set_header   Host              $host;
        proxy_set_header   Upgrade           $http_upgrade;        # vite HMR WS
        proxy_set_header   Connection        "upgrade";
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_xff;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_set_header   X-Forwarded-Host  $host;

        # dev：禁掉 nginx 侧缓存，避免改 index.html / vite 热更被攒着不发
        proxy_cache         off;
        proxy_buffering     off;
        proxy_request_buffering off;
        proxy_set_header    If-None-Match     "";
        proxy_set_header    If-Modified-Since "";
        add_header          Cache-Control     "no-store" always;
    }

    # ── /：兜底全部交给 www (Nuxt) ──
    location / {
        proxy_pass         http://www_app;
        proxy_http_version 1.1;
        proxy_set_header   Host              $host;
        proxy_set_header   Upgrade           $http_upgrade;        # Nitro HMR
        proxy_set_header   Connection        "upgrade";
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_xff;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_set_header   X-Forwarded-Host  $host;

        proxy_cache         off;
        proxy_buffering     off;
        proxy_request_buffering off;
        proxy_set_header    If-None-Match     "";
        proxy_set_header    If-Modified-Since "";
        add_header          Cache-Control     "no-store" always;
    }

    # 上线 HTTPS：
    #   - listen 443 ssl http2;
    #   - ssl_certificate / ssl_certificate_key
    #   - 后端 .env 设 JWT_COOKIE_SECURE=true
}
```

### 2.1 关键坑（务必）

1. **`proxy_pass http://my_spa;` 不能加斜杠**。带斜杠或路径会让 nginx 把 `/my` **剥掉**再转给上游，但 my 的 vite 是按 `base='/my/'` 跑的，剥掉后路径变成 `/login` —— vite 找不到。
2. **不要 rewrite 把 `/my` 去掉**。my 的 HTML 引用 `/my/assets/...`，剥掉后这些资源回到 nginx，落到兜底 `location /` 给了 www，必然 404。
3. **WebSocket 升级头必须给**。Vite/Nuxt 都用 WS 做 HMR，少了 `Upgrade`/`Connection: upgrade` 会反复 404。
4. **生产模式 production 之前**，dev 模式下务必加 `proxy_cache off; proxy_buffering off;`，否则改了 index.html 看不到效果。
5. **后端 `TRUSTED_PROXIES` 一定要含 nginx 的 IP/网段**，否则 access_log 全是 nginx 自身的 IP。

---

## 3. my（Vite SPA）部署到 `/my/` 子路径

### 3.1 `my/vite.config.ts`

```ts
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  // base 决定打包资源前缀，同时通过 import.meta.env.BASE_URL 喂给 vue-router。
  // 与 nginx 把 /my 转给本 SPA 的转发规则配套：保持 /my/ 前缀不被剥离，资源与路由
  // 都自然带上 /my/。单独跑 `pnpm dev` 时访问 http://localhost:5666/my/ 即可。
  // 想恢复成根路径可设 VITE_BASE=/。
  const base = env.VITE_BASE || '/my/'

  return {
    base,
    plugins: [ /* ... */ ],
    resolve: { /* ... */ },
    server: {
      port: 5666,
      strictPort: true, // 端口被占就直接 fail，避免 vite 悄悄换号导致 nginx 转到老进程
      host: true,
      // HMR 客户端默认 ws://<window.location.host>/ —— 经 nginx 时根路径会被路由到 www，
      // 升级失败。把 WS 路径也跟 base 对齐到 /my/，让 nginx 的 `location /my` 走 Upgrade。
      hmr: {
        path: '/my/',
      },
      proxy: { /* ... */ },
    },
  }
})
```

### 3.2 `my/index.html`：**保持** `/`-绝对路径

```html
<link rel="icon" type="image/svg+xml" href="/vite.svg" />
<script type="module" src="/src/main.ts"></script>
```

Vite 7 在 dev 阶段会自动把 `/vite.svg` 改写为 `/my/vite.svg`、`/src/main.ts` 改写为 `/my/src/main.ts`，同时它注入的 `/@vite/client`、`/node_modules/...`、`/src/...` 全都会带上 `/my/` 前缀。**不要**手动在 index.html 里写死 `/my/...`，否则会被 vite 再加一层变成 `/my/my/vite.svg`。

### 3.3 `my/src/router/index.ts`

`createWebHistory(import.meta.env.BASE_URL)` 会自动用上 `vite.base`，无需特殊改。

### 3.4 注意

- 把 my 当作子路径下挂的标准 SPA 部署即可，浏览器的 URL 始终是 `/my/...`，对用户、对收藏夹、对分享链接都一致。
- 单独跑 `pnpm dev`：浏览器开 `http://localhost:5666/my/`（不是 `/`）。
- **如果发现 HTML 里资源路径有"双 `/my/`" 或 "没有 `/my/`"**，99% 是端口冲突——5666 上跑的不是这个 my，是别的 app。`Get-NetTCPConnection -LocalPort 5666 -State Listen` 检查。

---

## 4. www（Nuxt SSR）：用 Cookie 拉登录态

### 4.1 `www/nuxt.config.ts`

```ts
export default defineNuxtConfig({
  // ...
  // 用户认证：SSR 侧通过 Cookie 调后端 /user/me 拉登录态。
  //  - backendBaseUrl：仅服务端用；nginx 后通常是 http://localhost:9981/api/v1
  //  - public.userCookieName：与后端 USER_JWT_COOKIE_NAME / my 的 cookie 名保持一致
  //  - public.myAppPath：「进入控制台」按钮跳转目标
  //
  // Nuxt 会自动用同名大写的环境变量覆盖这些默认值：
  //   NUXT_BACKEND_BASE_URL / NUXT_PUBLIC_USER_COOKIE_NAME / NUXT_PUBLIC_MY_APP_PATH
  runtimeConfig: {
    backendBaseUrl: 'http://localhost:9981/api/v1',
    public: {
      userCookieName: 'user_token',
      myAppPath: '/my/',
    },
  },
})
```

### 4.2 `www/composables/useAuthUser.ts`（新建）

```ts
// 与后端 User 形状一致——通过 /api/v1/user/me 返回。
export interface AuthUser {
  id: number
  phone: string
  nickname: string
  avatar: string
  is_active: boolean
  created_at: string
  updated_at: string
}

interface ApiEnvelope<T> { code: number; message: string; data: T }

/** 全局共享的「当前登录用户」状态。SSR 填充，客户端通过 payload 自动 hydrate。 */
export const useAuthUser = () => useState<AuthUser | null>('auth-user', () => null)

/**
 * 在 SSR 上下文里，把浏览器带过来的 Cookie 透传给后端 /user/me。
 * - Cookie 是 HttpOnly，客户端 JS 读不到，所以只能在 server 上做。
 * - 客户端无需重复调用：useState 已被 Nuxt 自动 hydrate。
 * - 任何失败（401 / 网络）都静默回退到「未登录」。
 */
export async function fetchAuthUserOnServer(): Promise<void> {
  if (!import.meta.server) return
  const user = useAuthUser()
  if (user.value) return

  const config = useRuntimeConfig()
  const cookieName = config.public.userCookieName
  const headers = useRequestHeaders(['cookie'])
  const cookieHeader = headers.cookie ?? ''
  if (!cookieHeader || !cookieHeader.includes(`${cookieName}=`)) return

  try {
    const res = await $fetch<ApiEnvelope<AuthUser>>(`${config.backendBaseUrl}/user/me`, {
      headers: { cookie: cookieHeader },
      ignoreResponseError: true,
    })
    if (res?.code === 0 && res.data) user.value = res.data
  }
  catch { /* 静默忽略 */ }
}

/**
 * 登出：调后端清 Cookie，本地状态清空。
 * HttpOnly Cookie 必须由后端 Set-Cookie MaxAge<0 才能清掉，所以一定要发请求。
 */
export async function logoutAuthUser(): Promise<void> {
  const user = useAuthUser()
  try {
    await $fetch('/api/v1/user/auth/logout', { method: 'POST', ignoreResponseError: true })
  }
  catch { /* 即便失败也要清本地态 */ }
  user.value = null
  await refreshNuxtData()
}
```

### 4.3 `www/plugins/auth.server.ts`（新建）

```ts
// 服务端启动时拉一次当前用户。文件名 .server 表示仅在 SSR 阶段执行；
// 拉到的 user 经 useState 自动序列化到 payload，客户端 hydrate 无闪烁。
export default defineNuxtPlugin(async () => {
  await fetchAuthUserOnServer()
})
```

### 4.4 `www/components/AppNavbar.vue`：导航按登录态切换

```vue
<script setup lang="ts">
const runtime = useRuntimeConfig()
const authUser = useAuthUser()
const isLoggedIn = computed(() => !!authUser.value)
const displayName = computed(() => authUser.value?.nickname || authUser.value?.phone || '')
const myAppPath = runtime.public.myAppPath

async function onLogout() {
  await logoutAuthUser()
}
</script>

<template>
  <template v-if="isLoggedIn">
    <a :href="myAppPath" class="btn btn-primary btn-sm">控制台</a>
    <!-- 头像下拉：用户名/手机号 + 控制台入口 + 退出 -->
    <button @click="onLogout">退出登录</button>
  </template>
  <template v-else>
    <!-- 注意：跳转 my 用 <a> 而不是 <NuxtLink>，因为 my 是另一个 SPA，必须发起真实导航 -->
    <a :href="`${myAppPath}login`" class="btn btn-ghost btn-sm">登录</a>
    <a :href="`${myAppPath}register`" class="btn btn-primary btn-sm">免费注册</a>
  </template>
</template>
```

### 4.5 注意

- `auth.server.ts` 是 **server-only plugin**，文件名必须以 `.server.ts` 结尾，否则客户端也会跑（拿不到 HttpOnly Cookie，无意义）。
- `useState('auth-user', ...)` 必须给 key，否则 SSR payload 序列化不到客户端。
- 调后端用 `useRequestHeaders(['cookie'])` 拿到原始 Cookie 头，**整个透传**给后端。不要尝试解析 cookieName 单独发——后端中间件读全 Cookie 头。
- `ignoreResponseError: true` 让 401 不抛错，避免 SSR 报红、空响应静默被处理成未登录。
- 营销站如果有「跳到用户后台」的按钮，**必须用 `<a href>`**，不能用 `<NuxtLink>`。`NuxtLink` 走 Nuxt router，会被理解为 www 内部路由，结果是 SPA 内导航而不会触发真实跳转。

---

## 5. my：登出竞态修复

> **症状**：点退出登录没回到登录页（实际上回了一下又被路由守卫弹回首页）。

### 5.1 病因

```ts
function logout() {
  userStore.logout()      // async 但没 await
  router.push('/login')   // 立刻 push
}
```

`userStore.logout()` 是 async（发请求 → `clear()` 把 `isLogin` 置 false），没 await 就 push。路由守卫看到 `isLogin=true` 跳 login 页，命中"已登录访问 login 页 → 重定向到首页"分支，把用户弹回首页。

### 5.2 修法

```ts
async function logout() {
  // 必须 await：等 store.clear() 把 isLogin 置 false 再跳转。
  // 否则路由守卫看到 isLogin=true，会把 /login 重定向回首页，造成"退出后回不到登录页"。
  await userStore.logout()
  // 用 replace 而不是 push，避免浏览器后退键能回到受保护页面
  router.replace({ name: 'login' })
}
```

**所有登出入口都要改**。本工程里在 `layouts/DefaultLayout.vue` 和 `layouts/components/AccountMenu.vue` 两处。

### 5.3 通用规则

凡是"清状态 → 导航"的场景（登出、切换组织、删除当前账号…），都要：

1. 清状态用 `await`
2. 跳转用 `router.replace`（而不是 push），杜绝后退回到旧 session

---

## 6. my：登录/注册页 logo 用外链回 www

```vue
<!-- ❌ 错：vue-router 会加 base 解析成 /my/ -->
<RouterLink class="auth-aside__brand" to="/">
  <span class="brand-mark" />
  <span>{{ t('auth.brand') }}</span>
</RouterLink>

<!-- ✅ 对：原生 a 标签绕开 SPA 路由，发真实跳转给 nginx，落到 www -->
<a class="auth-aside__brand" href="/">
  <span class="brand-mark" />
  <span>{{ t('auth.brand') }}</span>
</a>
```

通用原则：**SPA 内的"跨 app"导航不能用 RouterLink**。RouterLink 会被本 SPA 的 vue-router 拦截、加上 base 前缀。要跳到另一个 SPA / 营销站，必须用 `<a href>`。

---

## 7. www：Cookie 同意 banner + 政策页（GDPR 风格）

> 一套独立的小功能，跟主题切换、登录态都无耦合，可以单独拷过去。

### 7.1 `www/composables/useCookieConsent.ts`（新建）

```ts
// Cookie 同意状态：用 JSON cookie（gp-consent）落库；useCookie 让 SSR 与客户端共享同一值。
// - 未做选择时 value=null，模板上据此显示底部弹窗 banner。
// - 做过选择后 value 非空，banner 自动消失；用户后续可在 /cookies 页改偏好。
// - "必要"类别永远 true，不暴露开关；点"全部拒绝"也保留它。

export type ConsentCategory = 'necessary' | 'functional' | 'analytics' | 'marketing'

export interface ConsentValue {
  v: number           // schema 版本；加新类别时 bump，旧记录视为未决策
  ts: number          // 决策时间戳（ms）
  necessary: true
  functional: boolean
  analytics: boolean
  marketing: boolean
}

export const CONSENT_VERSION = 1
const COOKIE_NAME = 'gp-consent'
const COOKIE_MAX_AGE = 60 * 60 * 24 * 180 // 半年；过期重新征求

function nowConsent(opts: Partial<Omit<ConsentValue, 'v' | 'ts' | 'necessary'>>): ConsentValue {
  return {
    v: CONSENT_VERSION,
    ts: Date.now(),
    necessary: true,
    functional: opts.functional ?? false,
    analytics: opts.analytics ?? false,
    marketing: opts.marketing ?? false,
  }
}

export function useCookieConsent() {
  const cookie = useCookie<ConsentValue | null>(COOKIE_NAME, {
    default: () => null,
    maxAge: COOKIE_MAX_AGE,
    sameSite: 'lax',
  })

  const consent = computed<ConsentValue | null>(() => {
    const v = cookie.value
    if (!v || typeof v !== 'object' || v.v !== CONSENT_VERSION) return null
    return v
  })

  const hasDecided = computed(() => consent.value !== null)

  const allowed = (cat: ConsentCategory) => {
    if (cat === 'necessary') return true
    return !!consent.value && !!consent.value[cat]
  }

  function acceptAll() {
    cookie.value = nowConsent({ functional: true, analytics: true, marketing: true })
  }
  function rejectAll() {
    cookie.value = nowConsent({ functional: false, analytics: false, marketing: false })
  }
  function savePreferences(prefs: Partial<Pick<ConsentValue, 'functional' | 'analytics' | 'marketing'>>) {
    cookie.value = nowConsent({
      functional: prefs.functional ?? consent.value?.functional ?? false,
      analytics: prefs.analytics ?? consent.value?.analytics ?? false,
      marketing: prefs.marketing ?? consent.value?.marketing ?? false,
    })
  }
  function reset() { cookie.value = null }

  return { consent, hasDecided, allowed, acceptAll, rejectAll, savePreferences, reset }
}
```

### 7.2 `www/components/CookieConsent.vue`（新建）

底部 dialog 风格 banner：
- `<Teleport to="body">` + `role="dialog"` + `aria-live="polite"`
- 按钮：「全部接受」「仅必要」「自定义」「保存偏好」
- 展开后四类（必要 / 功能性 / 分析 / 广告）逐项开关，必要项显示「始终启用」徽章不可改
- 响应式：720px 以下竖排，按钮拉满宽
- 整个组件用 `v-if="!hasDecided"` 决定是否显示

### 7.3 `www/pages/cookies.vue`（新建）

完整政策 + 偏好编辑：
- 顶部：标题 + 最近更新时间 + 简介
- 中部：偏好编辑卡片（每类一行：名称/描述 + 开关；底部「全部拒绝 / 全部接受 / 保存偏好」三按钮 + 上次保存时间）
- 下部：政策正文 5 段（什么是 Cookie / 我们的目的 / 第三方与跨境 / 如何撤回 / 联系）
- `useHead({ title: ... })` 设页面标题

### 7.4 在 layout 挂载 + footer 加链接

```vue
<!-- layouts/default.vue -->
<ClientOnly>
  <!-- ClientOnly 是为了避免 Teleport 在 SSR 阶段反复 hydrate -->
  <CookieConsent />
</ClientOnly>
```

```vue
<!-- components/AppFooter.vue -->
<NuxtLink :to="localePath('/cookies')">管理偏好</NuxtLink>
```

### 7.5 怎么消费同意状态

任何要装分析/像素脚本的地方：

```ts
const { allowed } = useCookieConsent()
watch(() => allowed('analytics'), (ok) => {
  if (ok) loadGoogleAnalytics()
  else removeGoogleAnalytics()
}, { immediate: true })
```

### 7.6 i18n 文案位置

中英两份完整文案，结构：

```ts
cookies: {
  banner: { title, body, accept, reject, customize, manage, learnMore },
  prefs: { title, save, acceptAll, rejectAll, savedAt: 'Last updated: {time}', notDecided, alwaysOn },
  categories: {
    necessary:  { name, desc },
    functional: { name, desc },
    analytics:  { name, desc },
    marketing:  { name, desc },
  },
  policy: {
    title, updated, intro,
    sections: [{ h, p }, ...]
  },
}
```

### 7.7 注意

- **schema 版本号 `v: 1`**：将来加新类别（例如 `personalization`），bump 到 2 → 老用户的 cookie 会被识别为"未决策"，banner 重新弹一次。这是 GDPR 推荐做法。
- **HttpOnly 不能开**：客户端要读它来决定 banner 是否显示。`SameSite=lax` 够用，跨域 iframe 嵌入再考虑 `none + secure`。
- **`<ClientOnly>` 必加**：`<Teleport>` 在 SSR + Nuxt hydration 里容易出 mismatch 警告。

---

## 8. www：跨"另一个 SPA"的链接习惯

| 场景 | 用什么 | 解析为 |
| --- | --- | --- |
| www 内部路由（同站页面） | `<NuxtLink :to="localePath('/pricing')">` | 客户端 SPA 跳转 |
| www → 用户后台（my） | `<a :href="myAppPath + 'login'">` | 真实浏览器导航 |
| my 内部路由 | `<RouterLink :to="/dashboard">` | SPA 跳转，自动带 base `/my/` |
| my → 营销站（www） | `<a href="/">` | 真实浏览器导航，落到 nginx 兜底 |
| 任何 → 外部链接 | `<a href="https://..." target="_blank" rel="noopener">` | 标准外链 |

**一句话总结**：跨 SPA 的链接全部用原生 `<a>`，**绝不要用框架的 Link/RouterLink/NuxtLink**——它们都会被本 SPA 的路由器拦截、加 base、保留 SPA 上下文，结果不会跳到隔壁应用。

---


## 10. 部署 checklist

| 项 | 完成标志 |
| --- | --- |
| 后端 `.env` 填了 `TRUSTED_PROXIES` | 启动日志没有 ⚠️ 警告 |
| 后端 `USER_JWT_COOKIE_NAME` 与前端 `userCookieName` 一致 | 用 my 登录后 `document.cookie` 看不到（HttpOnly 正常），后端日志有写入 |
| my 的 `vite.base` 与 nginx `location /my` 对齐 | `curl :nginx/my/login` 看到的 HTML 资源全是 `/my/...` |
| nginx `location /my` 的 `proxy_pass` 末尾**没有**斜杠 | 同上 |
| WebSocket Upgrade 头都给了 | 浏览器 Network 里 vite `ws://.../my/?token=` 是 101 |
| HTTPS 上线时改 `JWT_COOKIE_SECURE=true` | Cookie 只走 https，不漏 |
| HTTPS 上线时 cookie 也带 `Secure` 标志 | `Set-Cookie: ...; Secure; HttpOnly; SameSite=Lax` |

---

## 11. 端到端冒烟流程

1. nginx 起来，三个上游全部就绪。
2. 浏览器开 `https://app.example.com/`，落 www 首页，导航条显示「登录 / 注册」。
3. 点登录跳 `/my/login`，my 加载（HTML 里资源全是 `/my/...`）。
4. 用 mock 账号登录 → 后端发 `Set-Cookie: user_token=...; Path=/; HttpOnly; SameSite=Lax`。
5. 跳回 `/` → www SSR 透传 Cookie 调 `/api/v1/user/me` → 导航条变成「控制台 + 头像 + 用户名」。
6. 点头像下拉「退出登录」→ www 调 `/api/v1/user/auth/logout` → 后端清 Cookie → state 清空 → 按钮变回「登录 / 注册」。
7. 点 footer「管理偏好」进 `/cookies` 改 cookie 同意，刷新页面无 banner 重弹。
