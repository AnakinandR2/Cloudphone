// 后端公开营销接口（/api/open/v1/billing/*）的服务端调用封装。
// 仅 SSR 侧调用；浏览器只打本站 /_api/*。返回解封后的 data；失败抛错由路由兜底成 null。

interface Envelope<T> {
  code: number
  message: string
  data: T
}

export async function billingFetch<T>(path: string): Promise<T> {
  const cfg = useRuntimeConfig()
  const base = (cfg.backendPublicUrl as string) || ''
  if (!base) {
    throw createError({ statusCode: 500, statusMessage: 'backendPublicUrl 未配置（NUXT_BACKEND_PUBLIC_URL）' })
  }
  const res = await $fetch<Envelope<T>>(base + path, { timeout: 8000 })
  if (!res || res.code !== 0) {
    throw createError({ statusCode: 502, statusMessage: res?.message || '后端返回错误' })
  }
  return res.data
}
