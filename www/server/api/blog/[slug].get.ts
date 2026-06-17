// GET /api/blog/:slug —— 单篇文章详情代理（含 body_html）。
// query: lang
import type { PubArticleDetail } from '~/types/content'

export default defineEventHandler(async (event) => {
  const slug = getRouterParam(event, 'slug')
  if (!slug) {
    throw createError({ statusCode: 400, statusMessage: '缺少文章 slug' })
  }
  const q = getQuery(event)
  const cfg = useRuntimeConfig()
  return await contentFetch<PubArticleDetail>(`/articles/${encodeURIComponent(slug)}`, {
    space: cfg.contentSpace as string,
    lang: langToApi(q.lang ? String(q.lang) : undefined),
  })
})
