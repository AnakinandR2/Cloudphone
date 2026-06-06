export interface PricingPlan {
  id: 'starter' | 'pro' | 'enterprise'
  monthly: number
  yearly: number
  featured?: boolean
  enterprise?: boolean
}

export const PRICING_PLANS: PricingPlan[] = [
  { id: 'starter', monthly: 0, yearly: 0 },
  { id: 'pro', monthly: 19, yearly: 15, featured: true },
  { id: 'enterprise', monthly: 49, yearly: 39, enterprise: true },
]

export interface BlogPostMeta {
  slug: string
  key: 'p1' | 'p2' | 'p3'
  date: string
  readMinutes: number
  /** Tailwind gradient classes for the cover */
  cover: string
}

export const BLOG_POSTS: BlogPostMeta[] = [
  {
    slug: 'realtime-collaboration-at-scale',
    key: 'p1',
    date: '2026-05-18',
    readMinutes: 8,
    cover: 'from-violet-500 via-purple-500 to-fuchsia-500',
  },
  {
    slug: 'guide-to-workflow-automation',
    key: 'p2',
    date: '2026-04-29',
    readMinutes: 6,
    cover: 'from-sky-500 via-blue-500 to-indigo-500',
  },
  {
    slug: 'designing-for-dark-mode',
    key: 'p3',
    date: '2026-04-12',
    readMinutes: 5,
    cover: 'from-emerald-500 via-teal-500 to-cyan-500',
  },
]

export function useBlog() {
  const { locale } = useI18n()

  function formatDate(iso: string) {
    const d = new Date(iso)
    return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    }).format(d)
  }

  function getPost(slug: string) {
    return BLOG_POSTS.find((p) => p.slug === slug)
  }

  return { posts: BLOG_POSTS, formatDate, getPost }
}
