// GET /_content/api-docs —— 当前 Key 可见的已发布 API 文档列表（仅安全字段）。
import type { ApiDocSummary } from '~/types/content'

export default defineEventHandler(async () => {
  return await contentFetch<ApiDocSummary[]>('/api-docs', {})
})
