import type {
  BillingAccount, ClaimableItem, EntitlementsResult, LedgerEntry,
  Order, OrderCreateReq, OrderDetail, QuoteRequest, QuoteResult,
  RuntimeUsageSlice, SkuWithTiers,
} from '@/types/billing'
import api from '../index'

interface R<T> { code: number, message: string, data: T }
interface Page<T> { list: T[], total: number }

export default {
  // 账户/资金
  account: () => api.get<unknown, R<BillingAccount>>('billing/account'),
  ledger: (params: { page?: number, size?: number, subject?: string, type?: string }) =>
    api.get<unknown, R<Page<LedgerEntry>>>('billing/ledger', { params }),
  topup: (amount_cents: number) => api.post<unknown, R<BillingAccount>>('billing/topup', { amount_cents }),

  // 目录/计价
  skus: () => api.get<unknown, R<SkuWithTiers[]>>('billing/skus'),
  quote: (req: QuoteRequest) => api.post<unknown, R<QuoteResult>>('billing/quote', req),

  // 订单
  createOrder: (req: OrderCreateReq) => api.post<unknown, R<OrderDetail>>('billing/orders', req),
  orders: (params: { page?: number, size?: number, status?: string }) =>
    api.get<unknown, R<Page<Order>>>('billing/orders', { params }),
  orderDetail: (id: number) => api.get<unknown, R<OrderDetail>>(`billing/orders/${id}`),
  payOrder: (id: number) => api.post<unknown, R<OrderDetail>>(`billing/orders/${id}/pay`),

  // 权益
  entitlements: () => api.get<unknown, R<EntitlementsResult>>('billing/entitlements'),

  // 运行用量（时长费切片，倒序分页）
  runtimeUsage: (params: { page?: number, size?: number }) =>
    api.get<unknown, R<Page<RuntimeUsageSlice>>>('billing/runtime/usage', { params }),

  // 试用
  trials: () => api.get<unknown, R<ClaimableItem[]>>('billing/trials'),
  claimTrial: (code: string, invite_code = '') =>
    api.post<unknown, R<null>>(`billing/trials/${code}/claim`, { invite_code }),
}
