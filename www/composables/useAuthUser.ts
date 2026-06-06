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

interface ApiEnvelope<T> { code: number, message: string, data: T }

/** 全局共享的「当前登录用户」状态。SSR 填充，客户端通过 payload 自动 hydrate。 */
export const useAuthUser = () => useState<AuthUser | null>('auth-user', () => null)

/**
 * 在 SSR 上下文里，把浏览器带过来的 Cookie 透传给后端 /user/me。
 * - Cookie 是 HttpOnly，客户端 JS 读不到，所以只能在 server 上做。
 * - 客户端无需重复调用：useState 已被 Nuxt 自动 hydrate。
 * - 任何失败（401 / 网络）都静默回退到「未登录」。
 */
export async function fetchAuthUserOnServer(): Promise<void> {
  if (!import.meta.server)
    return
  const user = useAuthUser()
  if (user.value)
    return

  const config = useRuntimeConfig()
  const cookieName = config.public.userCookieName
  const headers = useRequestHeaders(['cookie'])
  const cookieHeader = headers.cookie ?? ''
  if (!cookieHeader || !cookieHeader.includes(`${cookieName}=`))
    return

  try {
    const res = await $fetch<ApiEnvelope<AuthUser>>(`${config.backendBaseUrl}/user/me`, {
      headers: { cookie: cookieHeader },
      ignoreResponseError: true,
    })
    if (res?.code === 0 && res.data)
      user.value = res.data
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
