import type { PartnerCard, PartnerClickPayload } from '@/types/partner'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// 代理IP推荐：公开接口（无需登录；登录态下 axios 会自动带 token，后端软取 user_id）。
export default {
  // 启用合作商列表，按运营设定的排序返回。
  list: () => api.get<unknown, R<PartnerCard[]>>('partner/list'),

  // 上报一次推广链接点击。best-effort，调用方不应因失败阻断跳转。
  click: (id: number, payload: PartnerClickPayload) =>
    api.post<unknown, R<null>>(`partner/${id}/click`, payload),
}
