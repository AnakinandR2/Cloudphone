// GET /api/blog/taxonomy —— 分类 + 标签列表（并行取）。
// query: lang
import type { PubTaxon, PubTaxonomy } from '~/types/content'

export default defineEventHandler(async (event): Promise<PubTaxonomy> => {
  const q = getQuery(event)
  const cfg = useRuntimeConfig()
  const space = cfg.contentSpace as string
  const lang = langToApi(q.lang ? String(q.lang) : undefined)
  const [categories, tags] = await Promise.all([
    contentFetch<PubTaxon[]>('/article-categories', { space, lang }),
    contentFetch<PubTaxon[]>('/article-tags', { space, lang }),
  ])
  return { categories, tags }
})
