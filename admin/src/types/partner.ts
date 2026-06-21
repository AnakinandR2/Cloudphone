// 代理IP合作商（运营全局内容）。点击明细见 PartnerClick。
export interface Partner {
  id: number
  name: string
  logo_url: string
  image_url: string
  intro: string
  promo_url: string
  sort: number
  enabled: boolean
  click_count: number
  created_at: string
  updated_at: string
}

export interface PartnerCreate {
  name: string
  logo_url?: string
  image_url?: string
  intro?: string
  promo_url: string
  sort?: number
  enabled?: boolean
}

export interface PartnerUpdate {
  name?: string
  logo_url?: string
  image_url?: string
  intro?: string
  promo_url?: string
  sort?: number
  enabled?: boolean
}

export interface PartnerListParams {
  page: number
  size: number
  kw?: string
}

export interface PartnerListResult {
  list: Partner[]
  total: number
}

// 点击明细（运营查看趋势/明细）。
export interface PartnerClick {
  id: number
  partner_id: number
  user_id: number
  ip: string
  user_agent: string
  referer: string
  anonymous_id: string
  session_id: string
  utm_source: string
  utm_medium: string
  utm_campaign: string
  utm_term: string
  utm_content: string
  channel: string
  extra: string
  created_at: string
}

export interface PartnerClickListResult {
  list: PartnerClick[]
  total: number
}
