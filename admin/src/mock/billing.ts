import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}

// ---- SKUs ----
let skus = [
  { id: 1, code: 'instance_fee', category: 'instance_fee', name: '云手机实例费', description: '按台月计费', unit_price_cents: 3000, unit: '台月', listed: true, sort: 1 },
  { id: 2, code: 'boot_pack', category: 'boot_pack', name: '开机包', description: '一次性开机包', unit_price_cents: 2000, unit: '次', listed: true, sort: 2 },
  { id: 3, code: 'time_pack', category: 'time_pack', name: '时长包', description: '按小时计费', unit_price_cents: 20, unit: '小时', listed: true, sort: 3 },
]
let skuNextId = 4

// ---- Discount Tiers ----
let tiers = [
  // instance_fee: 按订阅周期
  { id: 1, sku_id: 1, cycle_months: 1, min_quantity: 1, discount_bps: 10000 },
  { id: 2, sku_id: 1, cycle_months: 3, min_quantity: 1, discount_bps: 8500 },
  { id: 3, sku_id: 1, cycle_months: 12, min_quantity: 1, discount_bps: 7000 },
  // boot_pack: 按订阅周期
  { id: 4, sku_id: 2, cycle_months: 1, min_quantity: 1, discount_bps: 10000 },
  { id: 5, sku_id: 2, cycle_months: 3, min_quantity: 1, discount_bps: 8500 },
  { id: 6, sku_id: 2, cycle_months: 12, min_quantity: 1, discount_bps: 7000 },
  // time_pack: 按数量阶梯（cycle_months=0 表示时长包）
  { id: 7, sku_id: 3, cycle_months: 0, min_quantity: 1, discount_bps: 10000 },
  { id: 8, sku_id: 3, cycle_months: 0, min_quantity: 500, discount_bps: 9000 },
  { id: 9, sku_id: 3, cycle_months: 0, min_quantity: 1000, discount_bps: 8000 },
]
let tierNextId = 10

// ---- Orders ----
let orders = [
  { id: 1, order_no: 'ORD202606010001', user_id: 101, status: 'paid', pay_method: 'balance', total_cents: 3000, paid_at: '2026-06-01T10:00:00Z', created_at: '2026-06-01T09:55:00Z', updated_at: '2026-06-01T10:00:00Z' },
  { id: 2, order_no: 'ORD202606010002', user_id: 102, status: 'pending', pay_method: 'wechat', total_cents: 8500, paid_at: null, created_at: '2026-06-01T11:00:00Z', updated_at: '2026-06-01T11:00:00Z' },
  { id: 3, order_no: 'ORD202606020001', user_id: 101, status: 'paid', pay_method: 'alipay', total_cents: 20000, paid_at: '2026-06-02T14:30:00Z', created_at: '2026-06-02T14:20:00Z', updated_at: '2026-06-02T14:30:00Z' },
  { id: 4, order_no: 'ORD202606030001', user_id: 103, status: 'pending', pay_method: 'alipay', total_cents: 6000, paid_at: null, created_at: '2026-06-03T08:00:00Z', updated_at: '2026-06-03T08:00:00Z' },
  { id: 5, order_no: 'ORD202606040001', user_id: 104, status: 'cancelled', pay_method: 'wechat', total_cents: 1000, paid_at: null, created_at: '2026-06-04T16:00:00Z', updated_at: '2026-06-04T17:00:00Z' },
]

const orderItems: Record<number, any[]> = {
  1: [{ id: 1, order_id: 1, sku_code: 'instance_fee', sku_name: '云手机实例费', category: 'instance_fee', cycle_months: 1, quantity: 1, unit_price_cents: 3000, discount_bps: 10000, original_cents: 3000, payable_cents: 3000 }],
  2: [{ id: 2, order_id: 2, sku_code: 'instance_fee', sku_name: '云手机实例费', category: 'instance_fee', cycle_months: 3, quantity: 1, unit_price_cents: 3000, discount_bps: 8500, original_cents: 9000, payable_cents: 8500 }],
  3: [{ id: 3, order_id: 3, sku_code: 'time_pack', sku_name: '时长包', category: 'time_pack', cycle_months: 0, quantity: 1000, unit_price_cents: 20, discount_bps: 8000, original_cents: 20000, payable_cents: 16000 }],
  4: [{ id: 4, order_id: 4, sku_code: 'instance_fee', sku_name: '云手机实例费', category: 'instance_fee', cycle_months: 1, quantity: 2, unit_price_cents: 3000, discount_bps: 10000, original_cents: 6000, payable_cents: 6000 }],
  5: [{ id: 5, order_id: 5, sku_code: 'boot_pack', sku_name: '开机包', category: 'boot_pack', cycle_months: 1, quantity: 1, unit_price_cents: 1000, discount_bps: 10000, original_cents: 1000, payable_cents: 1000 }],
}

// ---- Account (userId=101) ----
const accountStore: Record<number, any> = {
  101: {
    account: { id: 1, user_id: 101, balance_cents: 15000, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-06-01T10:00:00Z' },
    ledger: [
      { id: 1, user_id: 101, subject: 'balance', type: 'topup', delta: 20000, balance_after: 20000, reason: '充值', order_id: 0, operator: 'user', created_at: '2026-05-01T09:00:00Z' },
      { id: 2, user_id: 101, subject: 'balance', type: 'consume', delta: -3000, balance_after: 17000, reason: '订单 ORD202606010001', order_id: 1, operator: 'system', created_at: '2026-06-01T10:00:00Z' },
      { id: 3, user_id: 101, subject: 'balance', type: 'consume', delta: -2000, balance_after: 15000, reason: '订单 ORD202606020001', order_id: 3, operator: 'system', created_at: '2026-06-02T14:30:00Z' },
    ],
    ledger_total: 3,
    capacities: { instance_seat: 5, boot_seat: 10, runtime_minute: 3600 },
  },
}

// ---- Trial Policies ----
let trials: any[] = [
  { id: 1, code: 'new_user_trial', name: '新用户试用', enabled: true, per_user_limit: 1, allow_new_user: true, invite_code: '', items: [
    { id: 1, policy_id: 1, subject: 'instance_seat', quantity: 1, expire_days: 7 },
    { id: 2, policy_id: 1, subject: 'runtime_minute', quantity: 600, expire_days: 0 },
  ], created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
  { id: 2, code: 'invite_trial', name: '邀请码大礼包', enabled: true, per_user_limit: 1, allow_new_user: false, invite_code: 'GLORY2026', items: [
    { id: 3, policy_id: 2, subject: 'instance_seat', quantity: 2, expire_days: 30 },
    { id: 4, policy_id: 2, subject: 'runtime_minute', quantity: 1440, expire_days: 30 },
    { id: 5, policy_id: 2, subject: 'boot_seat', quantity: 1, expire_days: 30 },
  ], created_at: '2026-03-01T00:00:00Z', updated_at: '2026-03-01T00:00:00Z' },
]
let trialNextId = 3
let trialItemNextId = 6

const trialGrantsStore: Record<number, any[]> = {
  1: [
    { id: 1, policy_id: 1, user_id: 201, subject: 'instance_seat', quantity: 1, created_at: '2026-05-10T08:00:00Z' },
    { id: 2, policy_id: 1, user_id: 202, subject: 'instance_seat', quantity: 1, created_at: '2026-05-15T10:30:00Z' },
  ],
  2: [
    { id: 3, policy_id: 2, user_id: 203, subject: 'runtime_minute', quantity: 1440, created_at: '2026-06-01T12:00:00Z' },
  ],
}
let grantNextId = 4

export default defineFakeRoute([
  // ---- SKUs ----
  {
    url: '/v1/admin/billing/skus',
    method: 'get',
    response: () => ok(skus),
  },
  {
    url: '/v1/admin/billing/skus',
    method: 'post',
    response: ({ body }) => {
      const d = body as any
      const sku = { id: skuNextId++, code: d.code, category: d.category, name: d.name, description: d.description || '', unit_price_cents: d.unit_price_cents, unit: d.unit || '', listed: d.listed !== false, sort: d.sort || 0 }
      skus.push(sku)
      return ok(sku)
    },
  },
  {
    url: '/v1/admin/billing/skus/:id',
    method: 'put',
    response: ({ params, body }) => {
      const id = Number(params.id)
      const idx = skus.findIndex(s => s.id === id)
      if (idx < 0) return { code: 404, message: 'not found', data: null }
      const d = body as any
      skus[idx] = { ...skus[idx], ...d }
      return ok(skus[idx])
    },
  },
  {
    url: '/v1/admin/billing/skus/:id',
    method: 'delete',
    response: ({ params }) => {
      const id = Number(params.id)
      skus = skus.filter(s => s.id !== id)
      return ok(null)
    },
  },
  // ---- Tiers by SKU ----
  {
    url: '/v1/admin/billing/skus/:id/tiers',
    method: 'get',
    response: ({ params }) => {
      const skuId = Number(params.id)
      return ok(tiers.filter(t => t.sku_id === skuId))
    },
  },
  {
    url: '/v1/admin/billing/skus/:id/tiers',
    method: 'post',
    response: ({ params, body }) => {
      const skuId = Number(params.id)
      const d = body as any
      const tier = { id: tierNextId++, sku_id: skuId, cycle_months: d.cycle_months, min_quantity: d.min_quantity, discount_bps: d.discount_bps }
      tiers.push(tier)
      return ok(tier)
    },
  },
  // ---- Tier by tierId ----
  {
    url: '/v1/admin/billing/tiers/:tierId',
    method: 'put',
    response: ({ params, body }) => {
      const tierId = Number(params.tierId)
      const idx = tiers.findIndex(t => t.id === tierId)
      if (idx < 0) return { code: 404, message: 'not found', data: null }
      const d = body as any
      tiers[idx] = { ...tiers[idx], ...d }
      return ok(tiers[idx])
    },
  },
  {
    url: '/v1/admin/billing/tiers/:tierId',
    method: 'delete',
    response: ({ params }) => {
      const tierId = Number(params.tierId)
      tiers = tiers.filter(t => t.id !== tierId)
      return ok(null)
    },
  },
  // ---- Orders ----
  {
    url: '/v1/admin/billing/orders',
    method: 'get',
    response: ({ query }) => {
      let list = [...orders]
      if (query.userId) list = list.filter(o => o.user_id === Number(query.userId))
      if (query.status) list = list.filter(o => o.status === query.status)
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      const total = list.length
      const start = (page - 1) * size
      return ok({ list: list.slice(start, start + size), total })
    },
  },
  {
    url: '/v1/admin/billing/orders/:id/mark-paid',
    method: 'post',
    response: ({ params }) => {
      const id = Number(params.id)
      const idx = orders.findIndex(o => o.id === id)
      if (idx < 0) return { code: 404, message: 'not found', data: null }
      orders[idx] = { ...orders[idx], status: 'paid', paid_at: new Date().toISOString(), updated_at: new Date().toISOString() }
      return ok({ order: orders[idx], items: orderItems[id] || [] })
    },
  },
  // ---- Accounts ----
  {
    url: '/v1/admin/billing/accounts/:userId',
    method: 'get',
    response: ({ params, query }) => {
      const userId = Number(params.userId)
      const view = accountStore[userId]
      if (!view) {
        // Return a default empty account for unknown users
        const page = Number(query.page) || 1
        const size = Number(query.size) || 20
        return ok({
          account: { id: 0, user_id: userId, balance_cents: 0, created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
          ledger: [],
          ledger_total: 0,
          capacities: { instance_seat: 0, boot_seat: 0, runtime_minute: 0 },
        })
      }
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      const total = view.ledger.length
      const start = (page - 1) * size
      return ok({ ...view, ledger: view.ledger.slice(start, start + size), ledger_total: total })
    },
  },
  {
    url: '/v1/admin/billing/accounts/:userId/adjust',
    method: 'post',
    response: ({ params, body }) => {
      const userId = Number(params.userId)
      const d = body as any
      if (!accountStore[userId]) {
        accountStore[userId] = {
          account: { id: 0, user_id: userId, balance_cents: 0, created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
          ledger: [],
          ledger_total: 0,
          capacities: { instance_seat: 0, boot_seat: 0, runtime_minute: 0 },
        }
      }
      const acct = accountStore[userId].account
      acct.balance_cents += d.delta_cents
      acct.updated_at = new Date().toISOString()
      const entry = {
        id: accountStore[userId].ledger.length + 1,
        user_id: userId,
        subject: 'balance',
        type: d.delta_cents >= 0 ? 'adjust_grant' : 'adjust_deduct',
        delta: d.delta_cents,
        balance_after: acct.balance_cents,
        reason: d.reason || '',
        order_id: 0,
        operator: 'admin',
        created_at: new Date().toISOString(),
      }
      accountStore[userId].ledger.unshift(entry)
      accountStore[userId].ledger_total = accountStore[userId].ledger.length
      return ok(acct)
    },
  },
  {
    url: '/v1/admin/billing/accounts/:userId/adjust-resource',
    method: 'post',
    response: ({ params, body }) => {
      const userId = Number(params.userId)
      const d = body as any
      if (!accountStore[userId]) {
        accountStore[userId] = {
          account: { id: 0, user_id: userId, balance_cents: 0, created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
          ledger: [],
          ledger_total: 0,
          capacities: { instance_seat: 0, boot_seat: 0, runtime_minute: 0 },
        }
      }
      const caps = accountStore[userId].capacities
      if (d.subject in caps) {
        caps[d.subject as keyof typeof caps] += d.delta
      }
      return ok(null)
    },
  },
  // ---- Trial Policies ----
  {
    url: '/v1/admin/billing/trials',
    method: 'get',
    response: () => ok(trials),
  },
  {
    url: '/v1/admin/billing/trials',
    method: 'post',
    response: ({ body }) => {
      const d = body as any
      const id = trialNextId++
      const items = (d.items || []).map((it: any) => ({ id: trialItemNextId++, policy_id: id, subject: it.subject, quantity: it.quantity, expire_days: it.expire_days || 0 }))
      const policy = {
        id,
        code: d.code,
        name: d.name,
        enabled: d.enabled !== false,
        per_user_limit: d.per_user_limit || 1,
        allow_new_user: d.allow_new_user !== false,
        invite_code: d.invite_code || '',
        items,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      }
      trials.push(policy)
      return ok(policy)
    },
  },
  {
    url: '/v1/admin/billing/trials/:id',
    method: 'put',
    response: ({ params, body }) => {
      const id = Number(params.id)
      const idx = trials.findIndex(t => t.id === id)
      if (idx < 0) return { code: 404, message: 'not found', data: null }
      const d = body as any
      const next = { ...trials[idx], ...d, updated_at: new Date().toISOString() }
      if (d.items) {
        next.items = d.items.map((it: any) => ({ id: trialItemNextId++, policy_id: id, subject: it.subject, quantity: it.quantity, expire_days: it.expire_days || 0 }))
      }
      trials[idx] = next
      return ok(next)
    },
  },
  {
    url: '/v1/admin/billing/trials/:id',
    method: 'delete',
    response: ({ params }) => {
      const id = Number(params.id)
      trials = trials.filter(t => t.id !== id)
      return ok(null)
    },
  },
  {
    url: '/v1/admin/billing/trials/:id/eligibility',
    method: 'post',
    response: ({ params, body }) => {
      const id = Number(params.id)
      const d = body as any
      if (!trialGrantsStore[id]) trialGrantsStore[id] = []
      const policy = trials.find(t => t.id === id)
      const claimId = grantNextId++
      const items = policy?.items?.length ? policy.items : [{ subject: 'instance_seat', quantity: 1 }]
      for (const it of items) {
        trialGrantsStore[id].push({
          id: grantNextId++,
          claim_id: claimId,
          policy_id: id,
          user_id: d.user_id,
          subject: it.subject,
          quantity: it.quantity,
          created_at: new Date().toISOString(),
        })
      }
      return ok(null)
    },
  },
  {
    url: '/v1/admin/billing/trials/:id/grants',
    method: 'get',
    response: ({ params }) => {
      const id = Number(params.id)
      return ok(trialGrantsStore[id] || [])
    },
  },
])
