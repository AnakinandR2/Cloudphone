// GET /_api/pricing —— 代理后端公开定价；后端不可用时返回 null，由页面用默认数字兜底。
import type { PublicPricing } from '~/types/pricing'

export default defineEventHandler(async (): Promise<PublicPricing | null> => {
  try {
    return await billingFetch<PublicPricing>('/billing/pricing')
  }
  catch {
    return null
  }
})
