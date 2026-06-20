// GET /_content/blog/taxonomy —— 分类 + 标签（并行取）。
// 中台已合并目录与分类：分类 = 目录分组，改由 /directory 的 group 节点提供
// （旧 /article-categories 端点已下线）。标签仍由 /article-tags 提供。
// query: lang
import type { PubCategory, PubDirectoryNode, PubTaxon, PubTaxonomy } from '~/types/content'

// 递归摊平目录树里的所有 group 节点（含嵌套子分组）为分类列表。
function collectGroups(nodes: PubDirectoryNode[], out: PubCategory[] = []): PubCategory[] {
  for (const n of nodes) {
    if (n.kind === 'group') {
      out.push({ slug: n.slug, name: n.title })
      if (n.children?.length) collectGroups(n.children, out)
    }
  }
  return out
}

export default defineEventHandler(async (event): Promise<PubTaxonomy> => {
  const q = getQuery(event)
  const cfg = useRuntimeConfig()
  const space = cfg.contentSpace as string
  const lang = langToApi(q.lang ? String(q.lang) : undefined)
  const [tree, tags] = await Promise.all([
    contentFetch<PubDirectoryNode[]>('/directory', { space, lang }),
    contentFetch<PubTaxon[]>('/article-tags', { space, lang }),
  ])
  return { categories: collectGroups(tree), tags }
})
