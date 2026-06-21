// 后端公开营销接口（/api/open/v1/billing/*）的数据形状。

export interface QtyTier { min_quantity: number, discount_bps: number }
export interface DurationOption { value: number, discount_bps: number }

export interface KindPricing {
  unit_price_cents: number
  unit_label: string
  duration_unit: 'month' | 'day' | string
  qty_tiers: QtyTier[]
  duration_options: DurationOption[]
}

export interface RuntimePackPricing {
  unit_price_cents_per_minute: number
  min_minutes: number
  packs: { minutes: number, discount_bps: number }[]
  daily_cap_minutes: number
  gift_minutes_per_seat_month: number
}

export interface PublicPricing {
  kinds: Record<string, KindPricing>
  runtime_pack: RuntimePackPricing
  recharge_presets_cents: number[]
}

export interface TrialItem { subject: string, quantity: number, expire_days: number }
export interface TrialPolicyOverview { name: string, items: TrialItem[] }
export interface TrialOverview { policy: TrialPolicyOverview | null }
