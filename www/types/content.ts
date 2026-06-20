// 内容中台 Pub API 的数据类型，服务端代理与客户端组合式共用。
// 仅类型，无运行时代码 —— 编译后会被擦除，不会在两端间引入耦合。

/** 标签的精简表示。 */
export interface PubTaxon {
  id: number
  slug: string
  name: string
}

/**
 * 文章所属目录分组（即「分类」）。中台已合并目录与分类：
 * 一篇文章的分类就是它在目录树中的父分组，一文一位。
 */
export interface PubGroupRef {
  id: number
  slug: string
  name: string
  full_url: string
}

/**
 * 分类筛选项：由 /directory 的 group 节点摊平而来（无独立 id，按 slug 递归过滤）。
 */
export interface PubCategory {
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
  group: PubGroupRef | null
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

/** 分类 + 标签聚合（taxonomy 端点）。分类源自目录分组，标签源自 /article-tags。 */
export interface PubTaxonomy {
  categories: PubCategory[]
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

/** API 文档列表项（仅安全字段，正文 spec 另取）。 */
export interface ApiDocSummary {
  slug: string
  name: string
  description: string
  visibility: string
  spec_title: string
  spec_version: string
}
