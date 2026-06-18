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

// 博客内容已迁移至内容中台，见 composables/useBlog.ts。
