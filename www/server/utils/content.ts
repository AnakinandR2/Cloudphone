// 内容中台 Pub API 的服务端调用封装。
// 密钥只在服务端（runtimeConfig）读取，浏览器永不接触；页面只调用本站 /api/blog/*。

/**
 * 站点 i18n locale → 中台语言码。
 * 站点用 'zh' / 'en'，中台用 'zh-CN' / 'en'。
 */
export function langToApi(locale?: string): 'zh-CN' | 'en' {
  return locale === 'zh' || locale === 'zh-CN' ? 'zh-CN' : 'en'
}

interface Envelope<T> {
  code: number
  message: string
  data: T
}

/**
 * 调用 Pub API 并解封 `{ code, message, data }` 信封。
 * - 注入 `X-API-Key` 头与基础地址；
 * - 剔除空值查询参数；
 * - 上游 404 透传为 404，其余失败统一抛 502（携带中台 message）。
 */
export async function contentFetch<T>(
  path: string,
  query: Record<string, unknown>,
): Promise<T> {
  const cfg = useRuntimeConfig()
  const base = (cfg.pubBaseUrl as string) || ''
  const key = (cfg.contentApiKey as string) || ''
  if (!base || !key) {
    throw createError({
      statusCode: 500,
      statusMessage: '内容中台未配置（缺少 NUXT_PUB_BASE_URL / NUXT_CONTENT_API_KEY）',
    })
  }

  // 剔除 undefined / null / 空串，避免把空参数透传给中台。
  const cleaned: Record<string, string | number> = {}
  for (const [k, v] of Object.entries(query)) {
    if (v !== undefined && v !== null && v !== '') cleaned[k] = v as string | number
  }

  let res: Envelope<T>
  try {
    res = await $fetch<Envelope<T>>(base + path, {
      query: cleaned,
      headers: { 'X-API-Key': key },
      timeout: 10_000,
    })
  } catch (e: unknown) {
    const err = e as { response?: { status?: number }; data?: { message?: string }; message?: string }
    const upstream = err?.response?.status
    throw createError({
      statusCode: upstream === 404 ? 404 : 502,
      statusMessage: err?.data?.message || err?.message || '内容中台请求失败',
    })
  }

  if (!res || res.code !== 0) {
    throw createError({
      statusCode: res?.code === 404 ? 404 : 502,
      statusMessage: res?.message || '内容中台返回错误',
    })
  }
  return res.data
}
