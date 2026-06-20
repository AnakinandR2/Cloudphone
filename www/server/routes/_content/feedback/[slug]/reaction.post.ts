// POST /_content/feedback/:slug/reaction —— 提交赞踩（或评分）。
// query: space（默认 help）   body: { vote(-1/0/1) 或 rating(1-5), visitor_id(必填), lang, meta }
// 响应直接回带最新 FeedbackSummary（含 my_reaction）。
import type { FeedbackSummary } from '~/types/content'

export default defineEventHandler(async (event) => {
  const slug = getRouterParam(event, 'slug')
  if (!slug) throw createError({ statusCode: 400, statusMessage: '缺少文章 slug' })
  const q = getQuery(event)
  const b = await readBody<{
    vote?: number
    rating?: number
    visitor_id?: string
    lang?: string
    meta?: string
  }>(event)

  if (!b?.visitor_id) throw createError({ statusCode: 400, statusMessage: '缺少 visitor_id' })
  if (b.vote === undefined && b.rating === undefined) {
    throw createError({ statusCode: 422, statusMessage: 'rating、vote 至少提供其一' })
  }

  const body: Record<string, unknown> = {
    visitor_id: b.visitor_id,
    lang: langToApi(b.lang),
  }
  if (b.vote !== undefined) body.vote = b.vote
  if (b.rating !== undefined) body.rating = b.rating
  if (b.meta) body.meta = b.meta

  return await contentPost<FeedbackSummary>(
    event,
    `/articles/${encodeURIComponent(slug)}/reaction`,
    { space: q.space || 'help' },
    body,
  )
})
