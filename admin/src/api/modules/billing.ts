import type {
  Account,
  AccountView,
  DiscountTier,
  Order,
  OrderDetail,
  Sku,
  SkuCreate,
  SkuUpdate,
  TierCreate,
  TierUpdate,
  TrialGrant,
  TrialPolicy,
  TrialPolicyCreate,
  TrialPolicyUpdate,
} from '@/types/billing'
import api from '../index'

interface R<T> { code: number, message: string, data: T }
interface Page<T> { list: T[], total: number }

export default {
  // 定价
  listSkus: () => api.get<unknown, R<Sku[]>>('admin/billing/skus'),
  createSku: (d: SkuCreate) => api.post<unknown, R<Sku>>('admin/billing/skus', d),
  updateSku: (id: number, d: SkuUpdate) => api.put<unknown, R<Sku>>(`admin/billing/skus/${id}`, d),
  deleteSku: (id: number) => api.delete<unknown, R<null>>(`admin/billing/skus/${id}`),
  listTiers: (skuId: number) => api.get<unknown, R<DiscountTier[]>>(`admin/billing/skus/${skuId}/tiers`),
  createTier: (skuId: number, d: TierCreate) => api.post<unknown, R<DiscountTier>>(`admin/billing/skus/${skuId}/tiers`, d),
  updateTier: (tierId: number, d: TierUpdate) => api.put<unknown, R<DiscountTier>>(`admin/billing/tiers/${tierId}`, d),
  deleteTier: (tierId: number) => api.delete<unknown, R<null>>(`admin/billing/tiers/${tierId}`),

  // 订单
  orders: (params: { page?: number, size?: number, userId?: number, status?: string }) =>
    api.get<unknown, R<Page<Order>>>('admin/billing/orders', { params }),
  markPaid: (id: number) => api.post<unknown, R<OrderDetail>>(`admin/billing/orders/${id}/mark-paid`),

  // 账户/资源调整
  account: (userId: number, params?: { page?: number, size?: number }) =>
    api.get<unknown, R<AccountView>>(`admin/billing/accounts/${userId}`, { params }),
  adjustBalance: (userId: number, delta_cents: number, reason: string) =>
    api.post<unknown, R<Account>>(`admin/billing/accounts/${userId}/adjust`, { delta_cents, reason }),
  adjustResource: (userId: number, subject: string, delta: number, reason: string) =>
    api.post<unknown, R<unknown>>(`admin/billing/accounts/${userId}/adjust-resource`, { subject, delta, reason }),

  // 试用
  listTrials: () => api.get<unknown, R<TrialPolicy[]>>('admin/billing/trials'),
  createTrial: (d: TrialPolicyCreate) => api.post<unknown, R<TrialPolicy>>('admin/billing/trials', d),
  updateTrial: (id: number, d: TrialPolicyUpdate) => api.put<unknown, R<TrialPolicy>>(`admin/billing/trials/${id}`, d),
  deleteTrial: (id: number) => api.delete<unknown, R<null>>(`admin/billing/trials/${id}`),
  grantEligibility: (id: number, user_id: number) => api.post<unknown, R<null>>(`admin/billing/trials/${id}/eligibility`, { user_id }),
  trialGrants: (id: number) => api.get<unknown, R<TrialGrant[]>>(`admin/billing/trials/${id}/grants`),
}
