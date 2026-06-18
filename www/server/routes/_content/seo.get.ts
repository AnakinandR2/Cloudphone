// GET /_content/seo?path= —— 解析某 URL 的 SEO 配置（TDK 等），按 Key 工作空间。
import type { SeoResolveResult } from '~/types/content'

export default defineEventHandler(async (event) => {
  const q = getQuery(event)
  const path = q.path ? String(q.path) : '/'
  return await contentFetch<SeoResolveResult>('/seo/resolve', { path })
})
