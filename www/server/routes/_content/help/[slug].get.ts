// GET /api/help/:slug —— 帮助文档单篇详情代理（含 body_html）。
// query: lang
import type { PubArticleDetail } from '~/types/content'

export default defineEventHandler(async (event) => {
  const slug = getRouterParam(event, 'slug')
  if (!slug) {
    throw createError({ statusCode: 400, statusMessage: '缺少文档 slug' })
  }
  const q = getQuery(event)
  return await contentFetch<PubArticleDetail>(`/articles/${encodeURIComponent(slug)}`, {
    space: 'help',
    lang: langToApi(q.lang ? String(q.lang) : undefined),
  })
})
