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
