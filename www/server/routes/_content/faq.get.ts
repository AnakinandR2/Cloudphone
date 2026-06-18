// GET /api/faq —— FAQ 聚合代理。
// 先取列表（含分类），再并行取各篇 body_html（答案），按分类分组返回。
// query: lang
import type { FaqGroup, PubArticleDetail, PubList } from '~/types/content'

export default defineEventHandler(async (event): Promise<FaqGroup[]> => {
  const q = getQuery(event)
  const lang = langToApi(q.lang ? String(q.lang) : undefined)

  const list = await contentFetch<PubList>('/articles', { space: 'faq', lang, size: 50 })

  const details = await Promise.all(
    list.list.map((a) =>
      contentFetch<PubArticleDetail>(`/articles/${encodeURIComponent(a.slug)}`, { space: 'faq', lang }),
    ),
  )

  // 按分类分组，分组与组内顺序均沿用列表顺序。
  const groups: FaqGroup[] = []
  const byCat = new Map<string, FaqGroup>()
  for (const d of details) {
    const category = d.category?.name || ''
    let g = byCat.get(category)
    if (!g) {
      g = { category, items: [] }
      byCat.set(category, g)
      groups.push(g)
    }
    g.items.push({ slug: d.slug, question: d.title, answerHtml: d.body_html })
  }
  return groups
})
