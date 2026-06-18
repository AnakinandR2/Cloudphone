// 内容中台 Pub API 的数据类型，服务端代理与客户端组合式共用。
// 仅类型，无运行时代码 —— 编译后会被擦除，不会在两端间引入耦合。

/** 分类 / 标签的精简表示。 */
export interface PubTaxon {
  id: number
  slug: string
  name: string
}

/** 文章列表项（不含正文）。 */
export interface PubArticleSummary {
  id: number
  slug: string
  lang: string
  title: string
  path: string
  full_url: string
  summary: string
  cover_url: string
  cover_alt: string
  seo_title: string
  seo_description: string
  seo_keywords: string
  category: PubTaxon | null
  tags: PubTaxon[]
  published_at: string
  created_at: string
  updated_at: string
}

/** 文章详情（在列表项基础上增加正文与可用语言）。 */
export interface PubArticleDetail extends PubArticleSummary {
  body_html: string
  available_langs: string[]
}

/** 列表端点返回结构：当前页 + 过滤后总数。 */
export interface PubList {
  list: PubArticleSummary[]
  total: number
}

/** 分类 + 标签聚合（taxonomy 端点）。 */
export interface PubTaxonomy {
  categories: PubTaxon[]
  tags: PubTaxon[]
}

/** 目录树节点：分组（含子节点）或文章（叶子，带 url）。 */
export interface PubDirectoryNode {
  kind: 'group' | 'article'
  title: string
  slug: string
  collapsed: boolean
  url?: string
  children?: PubDirectoryNode[]
}

/** FAQ 一条问答（答案为 body_html）。 */
export interface FaqItem {
  slug: string
  question: string
  answerHtml: string
}

/** FAQ 按分类分组。 */
export interface FaqGroup {
  category: string
  items: FaqItem[]
}

/** 文档正文 TOC 一项。 */
export interface TocItem {
  id: string
  text: string
  level: number
}

/** SEO 配置的一条 meta：特殊 key `__title__` 表示 <title>；attr 缺省按 name。 */
export interface SeoMeta {
  key: string
  value: string
  attr?: string
}

/** seo/resolve 返回：是否命中 + 命中配置 id + meta 列表。 */
export interface SeoResolveResult {
  matched: boolean
  config_id?: number
  metas: SeoMeta[]
}
