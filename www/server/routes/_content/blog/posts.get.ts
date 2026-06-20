// GET /_content/blog/posts —— 博客文章列表代理。
// query: lang, page, size, group（分类 slug，递归含子分组）, tag_id, sort（默认 published_desc）
import type { PubList } from '~/types/content'

export default defineEventHandler(async (event) => {
  const q = getQuery(event)
  const cfg = useRuntimeConfig()
  return await contentFetch<PubList>('/articles', {
    space: cfg.contentSpace as string,
    lang: langToApi(q.lang ? String(q.lang) : undefined),
    page: q.page,
    size: q.size,
    group: q.group,
    tag_id: q.tag_id,
    sort: q.sort || 'published_desc',
  })
})
