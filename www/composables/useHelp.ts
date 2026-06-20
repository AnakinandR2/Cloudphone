// 帮助文档组合式：目录树 + 单篇 + 正文 TOC 构建。
// 密钥只在服务端，客户端只命中本站 /api/help/*。
import type { MaybeRefOrGetter } from 'vue'
import type { PubArticleDetail, PubDirectoryNode, TocItem } from '~/types/content'

/** 目录树（分组 + 文章）。 */
export function useHelpDirectory() {
  const { locale } = useI18n()
  return useFetch<PubDirectoryNode[]>('/_content/help/directory', {
    query: computed(() => ({ lang: locale.value })),
    default: () => [],
  })
}

/** 单篇文档。返回可 `await` 的 useFetch（详情页同步处理 404）。 */
export function useHelpArticle(slug: MaybeRefOrGetter<string>) {
  const { locale } = useI18n()
  return useFetch<PubArticleDetail>(() => `/_content/help/${toValue(slug)}`, {
    query: computed(() => ({ lang: locale.value })),
  })
}

// ---------------------------------------------------------------------------
// 纯函数辅助（无副作用，便于单测）
// ---------------------------------------------------------------------------

/** 把标题文本 slug 化为 ASCII 锚点；纯 CJK / 空 → 'sec'。 */
function slugifyHeading(text: string): string {
  const base = text
    .toLowerCase()
    .replace(/<[^>]+>/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return base || 'sec'
}

/**
 * 给正文 h2/h3 注入唯一锚点 id（已有 id 则保留），并产出 TOC 列表。
 * 中台 body_html 的标题无 id，需在渲染前注入，便于右侧 TOC 跳转。
 */
export function buildToc(html: string): { html: string; toc: TocItem[] } {
  const toc: TocItem[] = []
  const used = new Set<string>()
  const out = (html || '').replace(
    /<(h[23])([^>]*)>([\s\S]*?)<\/\1>/gi,
    (full: string, tag: string, attrs: string, inner: string) => {
      const level = tag.toLowerCase() === 'h2' ? 2 : 3
      const text = inner.replace(/<[^>]+>/g, '').trim()
      if (!text) return full
      // 已有 id：沿用它（TOC 必须指向真实 id，否则锚点断裂）
      const existing = attrs.match(/\sid=["']([^"']+)["']/)
      if (existing) {
        used.add(existing[1])
        toc.push({ id: existing[1], text, level })
        return full
      }
      const base = slugifyHeading(text)
      let id = base
      let n = 1
      while (used.has(id)) id = `${base}-${n++}`
      used.add(id)
      toc.push({ id, text, level })
      return `<${tag}${attrs} id="${id}">${inner}</${tag}>`
    },
  )
  return { html: out, toc }
}

/**
 * 返回目录树中通往某文章的祖先分组标题链（用于面包屑）。
 * 例：对外读取 API > API 文档（Scalar）→ ['对外读取 API', 'API 文档（Scalar）']。未找到返回 []。
 */
export function docTrail(tree: PubDirectoryNode[], slug: string): string[] {
  function dfs(nodes: PubDirectoryNode[], path: string[]): string[] | null {
    for (const n of nodes) {
      if (n.kind === 'article') {
        if (n.slug === slug) return path
      } else {
        const found = dfs(n.children || [], [...path, n.title])
        if (found) return found
      }
    }
    return null
  }
  return dfs(tree, []) || []
}

/** 深度优先取目录树中第一篇文章的 url（用于 /help 重定向）。 */
export function firstArticleUrl(tree: PubDirectoryNode[]): string | undefined {
  for (const node of tree) {
    if (node.kind === 'article' && node.url) return node.url
    if (node.children) {
      const found = firstArticleUrl(node.children)
      if (found) return found
    }
  }
  return undefined
}
