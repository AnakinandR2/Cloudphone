// 内容中台 Pub API 的服务端调用封装。
// 密钥只在服务端（runtimeConfig）读取，浏览器永不接触；页面只调用本站 /_content/*。
import type { H3Event } from 'h3'
import type { PubTaxon } from '~/types/content'

/**
 * 规整中台文章对象的可空数组字段。
 * 中台（Go）把「空切片」序列化为 JSON `null`，但前端契约声明 `tags` 恒为数组，
 * 且模板直接 `tags.length` / `tags.slice(...)`。此处在 BFF 边界兜底为 `[]`，
 * 让无标签文章不再导致列表 / 详情页渲染崩溃（reading 'length' of null）。
 * 传入空值时原样返回（详情不存在等情形由上游 404 处理，不在此臆造对象）。
 */
export function normalizeArticle<T extends { tags?: PubTaxon[] | null }>(a: T): T {
  if (!a) return a
  return { ...a, tags: Array.isArray(a.tags) ? a.tags : [] }
}

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
  event?: H3Event,
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
      headers: { 'X-API-Key': key, ...(event ? clientForwardHeaders(event) : {}) },
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

/**
 * 提取真实客户端 IP / User-Agent，产出转发给中台的请求头。
 * www 处于 nginx 之后：`x-forwarded-for` 首段即真实访客，回退 `x-real-ip` / 连接地址。
 * 让中台后台明细记录到真实访客，而非 www 服务器自身。
 */
export function clientForwardHeaders(event: H3Event): Record<string, string> {
  const xff = getHeader(event, 'x-forwarded-for') || ''
  const clientIp =
    xff.split(',')[0]?.trim() ||
    getHeader(event, 'x-real-ip') ||
    getRequestIP(event, { xForwardedFor: true }) ||
    ''
  const ua = getHeader(event, 'user-agent') || ''
  const headers: Record<string, string> = {}
  if (clientIp) {
    headers['X-Forwarded-For'] = clientIp
    headers['X-Real-IP'] = clientIp
  }
  if (ua) headers['User-Agent'] = ua
  return headers
}

/**
 * POST 回源内容中台并解封信封（reaction / feedback 写接口）。
 * 注入 `X-API-Key` 与真实客户端 IP/UA 转发头；上游 4xx 透传（403 scope 不足 / 422 校验），其余 502。
 */
export async function contentPost<T>(
  event: H3Event,
  path: string,
  query: Record<string, unknown>,
  body: Record<string, unknown>,
): Promise<T> {
  const cfg = useRuntimeConfig()
  const base = (cfg.pubBaseUrl as string) || ''
  const key = (cfg.contentApiKey as string) || ''
  if (!base || !key) {
    throw createError({ statusCode: 500, statusMessage: '内容中台未配置（缺少 NUXT_PUB_BASE_URL / NUXT_CONTENT_API_KEY）' })
  }

  let res: Envelope<T>
  try {
    res = await $fetch<Envelope<T>>(base + path, {
      method: 'POST',
      query,
      headers: { 'X-API-Key': key, ...clientForwardHeaders(event) },
      body,
      timeout: 10_000,
    })
  } catch (e: unknown) {
    const err = e as { response?: { status?: number }; data?: { message?: string }; message?: string }
    const upstream = err?.response?.status
    throw createError({
      statusCode: upstream && upstream >= 400 && upstream < 500 ? upstream : 502,
      statusMessage: err?.data?.message || err?.message || '内容中台请求失败',
    })
  }

  if (!res || res.code !== 0) {
    throw createError({ statusCode: 502, statusMessage: res?.message || '内容中台返回错误' })
  }
  return res.data
}

/**
 * 回源内容中台任意「原始字节」端点（非信封，如 web-files / api-docs spec）。
 * 透传上游 Content-Type / ETag；按 text 取，保留原始内容。
 */
export async function proxyRaw(
  event: H3Event,
  path: string,
  query: Record<string, string> = {},
): Promise<string> {
  const cfg = useRuntimeConfig()
  const base = (cfg.pubBaseUrl as string) || ''
  const key = (cfg.contentApiKey as string) || ''
  if (!base || !key) {
    throw createError({ statusCode: 500, statusMessage: '内容中台未配置（缺少 NUXT_PUB_BASE_URL / NUXT_CONTENT_API_KEY）' })
  }
  try {
    const res = await $fetch.raw<string>(base + path, {
      query,
      headers: { 'X-API-Key': key },
      responseType: 'text',
    })
    const ct = res.headers.get('content-type')
    if (ct) setResponseHeader(event, 'content-type', ct)
    const etag = res.headers.get('etag')
    if (etag) setResponseHeader(event, 'etag', etag)
    return (res._data as string) ?? ''
  } catch (e: unknown) {
    const err = e as { response?: { status?: number } }
    throw createError({ statusCode: err?.response?.status === 404 ? 404 : 502, statusMessage: '资源不存在' })
  }
}

/**
 * 回源内容中台的「文本/文件」（web-files）。
 * lang 留空时走中台的语言回退链（→ 全局 → zh-CN）。
 */
export function proxyWebFile(event: H3Event, urlPath: string, lang?: string): Promise<string> {
  const query: Record<string, string> = { path: urlPath }
  if (lang) query.lang = lang
  return proxyRaw(event, '/web-files/by-path', query)
}
