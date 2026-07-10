import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}

const GiB = 1024 ** 3

export default defineFakeRoute([
  {
    url: '/v1/library/overview',
    method: 'get',
    response: () => ok({
      usage: {
        used_bytes: 24 * GiB,
        capacity_bytes: 500 * GiB,
        locked: false,
      },
      subscription: {
        tier_code: 't500',
        capacity_bytes: 500 * GiB,
        monthly_price_cents: 9900,
        expire_at: '2026-12-31T23:59:59Z',
        is_free: false,
      },
      free_quota_bytes: 5 * GiB,
      tiers: [
        { code: 't50', name: '50GB', capacity_gb: 50, monthly_price_cents: 1900, tier_discount_bps: 10000, enabled: true, sort: 1 },
        { code: 't100', name: '100GB', capacity_gb: 100, monthly_price_cents: 2900, tier_discount_bps: 10000, enabled: true, sort: 2 },
        { code: 't500', name: '500GB', capacity_gb: 500, monthly_price_cents: 9900, tier_discount_bps: 10000, enabled: true, sort: 3 },
      ],
      custom_tier: {
        enabled: true,
        min_capacity_gb: 50,
        price_per_gb_month_cents: 20,
        tier_discount_bps: 10000,
      },
      duration_options: [
        { days: 30, discount_bps: 10000 },
        { days: 90, discount_bps: 9500 },
        { days: 365, discount_bps: 8500 },
      ],
      notice: '素材库容量按套餐计费，超额后将锁定上传与取用。',
      billing_note: '到期未续费将降级为免费额度。',
    }),
  },
])
