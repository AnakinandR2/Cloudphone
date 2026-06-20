// GET /_content/feedback/:slug/summary —— 文章反馈聚合代理。
// query: space（默认 help）, lang, visitor_id（带上才会返回 my_reaction）
import type { FeedbackSummary } from '~/types/content'

export default defineEventHandler(async (event) => {
  const slug = getRouterParam(event, 'slug')
  if (!slug) throw createError({ statusCode: 400, statusMessage: '缺少文章 slug' })
  const q = getQuery(event)
  return await contentFetch<FeedbackSummary>(
    `/articles/${encodeURIComponent(slug)}/feedback-summary`,
    {
      space: q.space || 'help',
      lang: langToApi(q.lang ? String(q.lang) : undefined),
      visitor_id: q.visitor_id,
    },
    event,
  )
})
