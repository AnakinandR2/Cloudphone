// GET /_api/trial-overview —— 代理后端公开营销试用（单条）；不可用/未配置时返回 { policy: null }。
import type { TrialOverview } from '~/types/pricing'

export default defineEventHandler(async (): Promise<TrialOverview> => {
  try {
    return await billingFetch<TrialOverview>('/billing/trial-overview')
  }
  catch {
    return { policy: null }
  }
})
