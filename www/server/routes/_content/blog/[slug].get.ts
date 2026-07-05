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
  const post = await contentFetch<PubArticleDetail>(`/articles/${encodeURIComponent(slug)}`, {
    space: cfg.contentSpace as string,
    lang: langToApi(q.lang ? String(q.lang) : undefined),
  })
  // 中台空 tags 切片会序列化为 null，兜底为数组，避免详情页 tags.length 崩溃。
  return normalizeArticle(post)
})
