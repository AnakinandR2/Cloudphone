// POST /_content/feedback/:slug/feedback —— 提交文本留言。
// query: space（默认 help）   body: { content(必填), contact?, visitor_id(必填), lang, meta }
// 响应直接回带最新 FeedbackSummary。
import type { FeedbackSummary } from '~/types/content'

export default defineEventHandler(async (event) => {
  const slug = getRouterParam(event, 'slug')
  if (!slug) throw createError({ statusCode: 400, statusMessage: '缺少文章 slug' })
  const q = getQuery(event)
  const b = await readBody<{
    content?: string
    contact?: string
    visitor_id?: string
    lang?: string
    meta?: string
  }>(event)

  if (!b?.visitor_id) throw createError({ statusCode: 400, statusMessage: '缺少 visitor_id' })
  const content = (b.content || '').trim()
  if (!content) throw createError({ statusCode: 400, statusMessage: '留言内容不能为空' })

  const body: Record<string, unknown> = {
    content,
    visitor_id: b.visitor_id,
    lang: langToApi(b.lang),
  }
  if (b.contact) body.contact = b.contact
  if (b.meta) body.meta = b.meta

  return await contentPost<FeedbackSummary>(
    event,
    `/articles/${encodeURIComponent(slug)}/feedback`,
    { space: q.space || 'help' },
    body,
  )
})
