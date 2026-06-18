// GET /api/blog/posts —— 博客文章列表代理。
// query: lang, page, size, category_id, tag_id, sort（默认 published_desc）
import type { PubList } from '~/types/content'

export default defineEventHandler(async (event) => {
  const q = getQuery(event)
  const cfg = useRuntimeConfig()
  return await contentFetch<PubList>('/articles', {
    space: cfg.contentSpace as string,
    lang: langToApi(q.lang ? String(q.lang) : undefined),
    page: q.page,
    size: q.size,
    category_id: q.category_id,
    tag_id: q.tag_id,
    sort: q.sort || 'published_desc',
  })
})
