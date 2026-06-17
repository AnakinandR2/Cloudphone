// 博客数据组合式：封装对本站 /api/blog/* 代理的 useFetch，并以 i18n locale
// 作为查询维度，切换语言时自动重新取数。密钥只在服务端，客户端只命中本站代理。
import type { MaybeRefOrGetter } from 'vue'
import type { PubArticleDetail, PubList, PubTaxonomy } from '~/types/content'

export interface BlogQuery {
  page?: number
  size?: number
  categoryId?: number
  tagId?: number
  sort?: 'directory' | 'published_desc'
}

/** 文章列表。传入响应式查询，变化时自动重取。 */
export function useBlogPosts(query: MaybeRefOrGetter<BlogQuery> = {}) {
  const { locale } = useI18n()
  const q = computed<BlogQuery>(() => toValue(query) ?? {})
  const params = computed(() => ({
    lang: locale.value,
    page: q.value.page,
    size: q.value.size,
    category_id: q.value.categoryId,
    tag_id: q.value.tagId,
    sort: q.value.sort,
  }))
  const { data, pending, error, refresh } = useFetch<PubList>('/api/blog/posts', {
    query: params,
    default: () => ({ list: [], total: 0 }),
  })
  const list = computed(() => data.value?.list ?? [])
  const total = computed(() => data.value?.total ?? 0)
  return { list, total, pending, error, refresh }
}

/**
 * 单篇文章。返回 useFetch 结果（可 `await`），便于详情页在 setup 中
 * 同步处理 404 并产出正确的 SSR 状态码。
 */
export function useBlogPost(slug: MaybeRefOrGetter<string>) {
  const { locale } = useI18n()
  return useFetch<PubArticleDetail>(() => `/api/blog/${toValue(slug)}`, {
    query: computed(() => ({ lang: locale.value })),
  })
}

/**
 * 分类 + 标签。返回 useFetch 结果（可 `await`），便于列表页在构造文章
 * 查询前先等到 taxonomy 就绪 —— slug→id 映射依赖它，避免 SSR 时序导致漏过滤。
 */
export function useBlogTaxonomy() {
  const { locale } = useI18n()
  return useFetch<PubTaxonomy>('/api/blog/taxonomy', {
    query: computed(() => ({ lang: locale.value })),
    default: () => ({ categories: [], tags: [] }),
  })
}

// ---------------------------------------------------------------------------
// 纯函数辅助（无副作用，便于单测）
// ---------------------------------------------------------------------------

/** 按站点 locale 本地化日期；非法日期返回空串。 */
export function formatBlogDate(iso: string, locale: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return new Intl.DateTimeFormat(locale === 'zh' ? 'zh-CN' : 'en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  }).format(d)
}

/**
 * 从 body_html 估算阅读时长（分钟，至少 1）。
 * 中文按约 400 字/分钟，英文按约 200 词/分钟，取主导者。
 */
export function readingMinutes(html: string): number {
  const text = (html || '').replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim()
  if (!text) return 1
  const cjk = (text.match(/[一-鿿]/g) || []).length
  const words = text.split(/\s+/).filter(Boolean).length
  const minutes = cjk > words ? Math.ceil(cjk / 400) : Math.ceil(words / 200)
  return Math.max(1, minutes)
}
