// 代理IP推荐合作商（my/www 展示用精简视图，由公开接口返回）。
export interface PartnerCard {
  id: number
  name: string
  logo_url: string
  image_url: string
  intro: string
  promo_url: string
}

// 点击上报的软标识（IP/UA/Referer 由服务端采集）。
export interface PartnerClickPayload {
  anonymous_id?: string
  session_id?: string
  utm_source?: string
  utm_medium?: string
  utm_campaign?: string
  utm_term?: string
  utm_content?: string
  channel?: string
  extra?: string
}
